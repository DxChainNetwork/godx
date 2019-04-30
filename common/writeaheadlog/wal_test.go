package writeaheadlog

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func retry(tries int, durationBetweenAttemps time.Duration, fn func() error) (err error) {
	for i := 1; i < tries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(durationBetweenAttemps)
	}
	return fn()
}

func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "wal", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v", path))
	}
	return path
}

type walTester struct {
	wal           *Wal
	recoveredTxns []*Transaction
	path          string
}

func (wt *walTester) close() error {
	return wt.wal.Close()
}

func newWalTester(name string, utils utilsSet) (*walTester, error) {
	testdir := tempDir(name)
	err := os.Mkdir(testdir, 0700)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(testdir, "test.wal")
	wal, recoveredTxns, err := newWal(path, utils)
	if err != nil {
		return nil, err
	}

	for _, txn := range recoveredTxns {
		if err := txn.Release(); err != nil {
			wal.Close()
			return nil, err
		}
	}

	cmt := &walTester{
		wal:           wal,
		recoveredTxns: recoveredTxns,
		path:          path,
	}
	return cmt, nil
}

// TestCommitFailed checks if a corruption of the first page of the
// transaction during the commit is handled correctly
func TestCommitFailed(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsCommitFail{})
	if err != nil {
		t.Fatal(err)
	}
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(1234),
	})
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}
	<-txn.InitComplete
	if txn.InitErr != nil {
		t.Errorf("unexpected init error: %v", err)
	}
	wait := txn.Commit()
	if err := <-wait; err == nil {
		t.Errorf("Expected fail in commit fail: %v", err)
	}
	wt.close()

	// After close, the wal should still exist
	if _, err := os.Stat(wt.path); os.IsNotExist(err) {
		t.Errorf("wal was deleted at %v", wt.path)
	}
	recoveredWal, recoveredTxns, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}
	defer recoveredWal.Close()

	if len(recoveredTxns) != 0 {
		t.Errorf("Number of updates after restart didn't match. Expected %v, but was %v",
			0, len(recoveredTxns))
	}
}

// TestReleaseFailed checks if a corruption of the first page of the
// transaction during the commit is handled correctly
func TestReleaseFailed(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsReleaseFail{})
	if err != nil {
		t.Fatal(err)
	}

	// Create a transaction with 1 update
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(1234),
	})

	// Create the transaction
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}

	// Committing the txn should fail on purpose
	wait := txn.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete failed %v", err)
	}

	//Committing the txn should fail on purpose
	if err := txn.Release(); err == nil {
		t.Error("SignalUpdatesApplies should have failed but didn't")
	}

	// shutdown the wal
	err = wt.close()
	if err == nil {
		t.Errorf("Expected error when closed with unfinished txn")
	}

	// make sure the wal is still there
	if _, err := os.Stat(wt.path); os.IsNotExist(err) {
		t.Errorf("wal was deleted at %v", wt.path)
	}
	// Restart it. There should be 1 unfinished update since it was committed
	// but never released
	recoveredWal, recoveredTxns, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}
	defer recoveredWal.Close()

	if len(recoveredTxns) != 1 {
		t.Errorf("Number of ops after restart didn't match. Expected %v, but was %v",
			1, len(recoveredTxns))
	}
}

// TestReleaseNotCalled checks if an interrupt between committing and releasing a
// transaction is handled correctly upon reboot
func TestReleaseNotCalled(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsUncleanShutdown{})
	if err != nil {
		t.Fatal(err)
	}
	// Create a transaction with 1 update
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(1234),
	})
	// Create one transaction which will be committed and one that will be applied
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}
	txn2, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}

	// wait for the transactions to be committed
	wait := txn.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete for the first transaction failed %v", err)
	}
	wait2 := txn2.Commit()
	if err := <-wait2; err != nil {
		t.Errorf("SignalSetupComplete for the second transaction failed")
	}

	// release the changes of the second transaction
	if err := txn2.Release(); err != nil {
		t.Errorf("SignalApplyComplete for the second transaction failed")
	}

	// shutdown the wal
	wt.close()

	// make sure the wal is still there
	if _, err := os.Stat(wt.path); os.IsNotExist(err) {
		t.Errorf("wal was deleted at %v", wt.path)
	}

	// Restart it and check if exactly 1 unfinished transaction is reported
	w, ops2, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if len(ops2) != len(ops) {
		t.Errorf("Number of ops after restart didn't match. Expected %v, but was %v",
			len(ops), len(ops2))
	}
}

// TestPayloadCorrupted creates 2 update and corrupts the first one. Therefore
// the second transaction should be recovered.
func TestPayloadCorrupted(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsUncleanShutdown{})
	if err != nil {
		t.Fatal(err)
	}

	// Create a transaction with 1 update
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(1234),
	})

	// Create 2 txns
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}
	txn2, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}

	// Committing the txns but don't release them
	wait := txn.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete failed %v", err)
	}

	wait = txn2.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete failed %v", err)
	}

	// Corrupt the payload of the first txn
	txn.headPage.payload = randomBytes(2000)
	_, err = txn.wal.logFile.WriteAt(txn.headPage.marshal(nil), int64(txn.headPage.offset))
	if err != nil {
		t.Errorf("Corrupting the page failed %v", err)
	}

	// shutdown the wal
	wt.close()

	// make sure the wal is still there
	if _, err := os.Stat(wt.path); os.IsNotExist(err) {
		t.Errorf("wal was deleted at %v", wt.path)
	}

	// Restart it. 1 unfinished transaction should be reported
	w, ops2, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if len(ops2) != 1 {
		t.Errorf("Number of ops after restart didn't match. Expected %v, but was %v",
			1, len(ops2))
	}
}

// TestPayloadCorrupted2 creates 2 update and corrupts the second one. Therefore
// one unfinished transaction should be reported
func TestPayloadCorrupted2(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsUncleanShutdown{})
	if err != nil {
		t.Fatal(err)
	}

	// Create a transaction with 1 update
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(1234),
	})

	// Create 2 txns
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}
	txn2, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}

	// Committing the txns but don't release them
	wait := txn.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete failed %v", err)
	}

	wait = txn2.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete failed %v", err)
	}

	// Corrupt the payload of the second txn
	txn2.headPage.payload = randomBytes(2000)
	_, err = txn2.wal.logFile.WriteAt(txn2.headPage.marshal(nil), int64(txn2.headPage.offset))
	if err != nil {
		t.Errorf("Corrupting the page failed %v", err)
	}

	// shutdown the wal
	wt.close()

	// make sure the wal is still there
	if _, err := os.Stat(wt.path); os.IsNotExist(err) {
		t.Errorf("wal was deleted at %v", wt.path)
	}

	// Restart it. 1 Unfinished transaction should be reported.
	w, ops2, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if len(ops2) != 1 {
		t.Errorf("Number of updates after restart didn't match. Expected %v, but was %v",
			0, len(ops2))
	}
}

// TestWalParallel checks if the wal still works without errors under a high load parallel work
// The wal won't be deleted but reloaded instead to check if the amount of returned failed updates
// equals 0
func TestWalParallel(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsProd{})
	if err != nil {
		t.Fatal(err)
	}

	// Prepare a random update
	ops := []Operation{}
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(1234),
	})

	// Define a function that creates a transaction from this update and applies it
	done := make(chan error)
	fn := func() {
		// Create txn
		txn, err := wt.wal.NewTransaction(ops)
		if err != nil {
			done <- err
			return
		}
		// Wait for the txn to be committed
		if err := <-txn.Commit(); err != nil {
			done <- err
			return
		}
		if err := txn.Release(); err != nil {
			done <- err
			return
		}
		done <- nil
	}

	// Create numThreads instances of the function and wait for it to complete without error
	numThreads := 200
	for i := 0; i < numThreads; i++ {
		go fn()
	}
	for i := 0; i < numThreads; i++ {
		err := <-done
		if err != nil {
			t.Errorf("Thread %v failed: %v", i, err)
		}
	}

	// The number of available pages should equal the number of created pages
	if wt.wal.pageCount != uint64(len(wt.wal.availablePages)) {
		t.Errorf("number of available pages doesn't match the number of created ones. Expected %v, but was %v",
			wt.wal.availablePages, wt.wal.pageCount)
	}

	// shutdown the wal
	wt.close()

	// Get the fileinfo
	fi, err := os.Stat(wt.path)
	if os.IsNotExist(err) {
		t.Fatalf("wal was deleted but shouldn't have")
	}

	// Log some stats about the file
	t.Logf("filesize: %v mb", float64(fi.Size())/float64(1e+6))
	t.Logf("used pages: %v", wt.wal.pageCount)

	// Restart it and check that no unfinished transactions are reported
	w, ops2, err := New(wt.path)
	if err != nil {
		t.Error(err)
	}
	defer w.Close()

	if len(ops2) != 0 {
		t.Errorf("Number of ops after restart didn't match. Expected %v, but was %v",
			0, len(ops2))
	}
}

// TestPageRecycling checks if pages are actually freed and used again after a transaction was applied
func TestPageRecycling(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsProd{})
	if err != nil {
		t.Error(err)
	}
	defer wt.close()

	// Prepare a random update
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(5000),
	})

	// Create txn
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the txn to be committed
	if err := <-txn.Commit(); err != nil {
		t.Errorf("SignalSetupComplete failed: %v", err)
	}

	// There should be no available pages before the transaction was applied
	if len(wt.wal.availablePages) != 0 {
		t.Errorf("Number of available pages should be 0 but was %v", len(wt.wal.availablePages))
	}

	if err := txn.Release(); err != nil {
		t.Errorf("SignalApplyComplete failed: %v", err)
	}

	usedPages := wt.wal.pageCount
	availablePages := len(wt.wal.availablePages)
	// The number of used pages should be greater than 0
	if usedPages == 0 {
		t.Errorf("The number of used pages should be greater than 0")
	}
	// Make sure usedPages equals availablePages and remember the values
	if usedPages != uint64(availablePages) {
		t.Errorf("number of used pages doesn't match number of available pages")
	}

	// Create second txn
	txn2, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the txn to be committed
	if err := <-txn2.Commit(); err != nil {
		t.Errorf("SignalSetupComplete failed: %v", err)
	}
	// There should be no available pages before the transaction was applied
	if len(wt.wal.availablePages) != 0 {
		t.Errorf("Number of available pages should be 0 but was %v", len(wt.wal.availablePages))
	}
	if err := txn2.Release(); err != nil {
		t.Errorf("SignalApplyComplete failed: %v", err)
	}

	// The number of used pages shouldn't have increased and still be equal to the number of available ones
	if wt.wal.pageCount != usedPages || len(wt.wal.availablePages) != availablePages {
		t.Errorf("expected used pages %v, was %v", usedPages, wt.wal.pageCount)
		t.Errorf("expected available pages %v, was %v", availablePages, len(wt.wal.availablePages))
	}
}

// TestRecoveryFailed checks if the WAL behave correctly if a crash occurs
// during a call to RecoveryComplete
func TestRecoveryFailed(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsUncleanShutdown{})
	if err != nil {
		t.Error(err)
	}

	// Prepare random ops
	numOps := 10
	var ops []Operation
	for i := 0; i < numOps; i++ {
		ops = append(ops, Operation{
			Name: "test",
			Data: randomBytes(10000),
		})
	}

	// Create txn
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		return
	}

	// Wait for the txn to be committed
	if err = <-txn.Commit(); err != nil {
		return
	}

	// Close and restart the wal.
	if err := wt.close(); err == nil {
		t.Error("There should have been an error but there wasn't")
	}

	w, recoveredTxns2, err := newWal(wt.path, &utilsRecoveryFail{})
	if err != nil {
		t.Fatal(err)
	}

	// New should return numOps ops
	numRecoveredUpdates := 0
	for _, txn := range recoveredTxns2 {
		numRecoveredUpdates += len(txn.Operations)
	}
	if numRecoveredUpdates != numOps {
		t.Errorf("There should be %v ops but there were %v", numOps, numRecoveredUpdates)
	}

	// Signal that the recovery is complete
	for _, txn := range recoveredTxns2 {
		if err := txn.Release(); err != nil {
			t.Errorf("Failed to signal applied ops: %v", err)
		}
	}

	// Restart the wal again
	if err := w.Close(); err != nil {
		t.Errorf("Failed to close wal: %v", err)
	}
	w, recoveredTxns3, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}

	// There should be 0 ops this time
	if len(recoveredTxns3) != 0 {
		t.Errorf("There should be %v ops but there were %v", 0, len(recoveredTxns3))
	}
	// The metadata should say "unclean"
	mdData := make([]byte, PageSize)
	if _, err := w.logFile.ReadAt(mdData, 0); err != nil {
		t.Fatal(err)
	}
	recoveryState, err := readMetadata(mdData)
	if err != nil {
		t.Fatal(err)
	}
	if recoveryState != stateUnclean {
		t.Errorf("recoveryState should be %v but was %v",
			stateUnclean, recoveryState)
	}

	// Close the wal again and check that the file still exists on disk
	if err := w.Close(); err != nil {
		t.Errorf("Failed to close wal: %v", err)
	}
	_, err = os.Stat(wt.path)
	if os.IsNotExist(err) {
		t.Errorf("wal was deleted but shouldn't have")
	}
}

// TestTransactionAppend tests the functionality of the Transaction's Append
// call
func TestTransactionAppend(t *testing.T) {
	wt, err := newWalTester(t.Name(), &utilsProd{})
	if err != nil {
		t.Fatal(err)
	}

	// Create a transaction with 1 update
	ops := []Operation{{
		Name: "test",
		Data: randomBytes(3000),
	}}
	txn, err := wt.wal.NewTransaction(ops)
	if err != nil {
		t.Fatal(err)
	}

	// Append another update
	if err := <-txn.Append(ops); err != nil {
		t.Errorf("Append failed: %v", err)
	}

	// wait for the transactions to be committed
	wait := txn.Commit()
	if err := <-wait; err != nil {
		t.Errorf("SignalSetupComplete for the first transaction failed %v", err)
	}

	// shutdown the wal
	wt.close()

	// Restart it and check if exactly 2 unfinished transactions are reported
	w, recoveredTxns2, err := New(wt.path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if len(recoveredTxns2[0].Operations) != len(ops)*2 {
		t.Errorf("Number of ops after restart didn't match. Expected %v, but was %v",
			len(ops)*2, len(recoveredTxns2[0].Operations))
	}
}

// BenchmarkTransactionSpeedAppend runs benchmarkTransactionSpeed with append =
// false
func BenchmarkTransactionSpeed(b *testing.B) {
	numThreads := []int{1, 10, 50, 100}
	for _, n := range numThreads {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkTransactionSpeed(b, n, false, 1*time.Second)
		})
	}
}

// BenchmarkTransactionSpeedAppend runs benchmarkTransactionSpeed with append =
// true
func BenchmarkTransactionSpeedAppend(b *testing.B) {
	numThreads := []int{1, 10, 50, 100}
	for _, n := range numThreads {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkTransactionSpeed(b, n, true, 1*time.Second)
		})
	}
}

// benchmarkTransactionSpeed is a helper function to create benchmarks that run
// for 1 min to find out how many transactions can be applied to the wal and
// how large the wal grows during that time using a certain number of threads.
// When appendUpdate is set to 'true', a second update will be appended to the
// transaction before it is committed.
func benchmarkTransactionSpeed(b *testing.B, numThreads int, appendUpdate bool, duration time.Duration) {
	b.Logf("Running benchmark with %v threads", numThreads)

	wt, err := newWalTester(b.Name(), &utilsProd{})
	if err != nil {
		b.Error(err)
	}
	defer wt.close()

	// Prepare a random update
	var ops []Operation
	ops = append(ops, Operation{
		Name: "test",
		Data: randomBytes(4000), // 1 page / txn
	})

	// Define a function that creates a transaction from this update and
	// applies it. It returns the duration it took to commit the transaction.
	f := func() (latency time.Duration, err error) {
		// Get start time
		startTime := time.Now()
		// Create txn
		txn, err := wt.wal.NewTransaction(ops)
		if err != nil {
			return
		}
		// Append second update
		if appendUpdate {
			if err = <-txn.Append(ops); err != nil {
				return
			}
		}
		// Wait for the txn to be committed
		if err = <-txn.Commit(); err != nil {
			return
		}
		// Calculate latency after committing
		latency = time.Since(startTime)
		if err = txn.Release(); err != nil {
			return
		}
		return latency, nil
	}

	// Create a channel to stop threads
	stop := make(chan struct{})

	// Create atomic variables to count transactions and errors
	var atomicNumTxns uint64
	var atomicNumErr uint64

	// Create waitgroup to wait for threads before reading the counters
	var wg sync.WaitGroup

	// Save latencies
	latencies := make([]time.Duration, numThreads, numThreads)

	// Start threads
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			for {
				// Check for stop signal
				select {
				case <-stop:
					return
				default:
				}
				// Execute the function
				latency, err := f()
				if err != nil {
					// Abort thread on error
					atomic.AddUint64(&atomicNumErr, 1)
					return
				}
				atomic.AddUint64(&atomicNumTxns, 1)

				// Remember highest latency
				if latency > latencies[j] {
					latencies[j] = latency
				}
			}
		}(i)
	}

	// Kill threads after 1 minute
	select {
	case <-time.After(duration):
		close(stop)
	}

	// Wait for each thread to finish
	wg.Wait()

	// Check if any errors happened
	if atomicNumErr > 0 {
		b.Fatalf("%v errors happened during execution", atomicNumErr)
	}

	// Get the fileinfo
	fi, err := os.Stat(wt.path)
	if os.IsNotExist(err) {
		b.Errorf("wal was deleted but shouldn't have")
	}

	// Find the maximum latency it took to commit a transaction
	var maxLatency time.Duration
	for i := 0; i < numThreads; i++ {
		if latencies[i] > maxLatency {
			maxLatency = latencies[i]
		}
	}

	// Log results
	b.Logf("filesize: %v mb", float64(fi.Size())/float64(1e+6))
	b.Logf("used pages: %v", wt.wal.pageCount)
	b.Logf("total transactions: %v", atomicNumTxns)
	b.Logf("txn/s: %v", float64(atomicNumTxns)/60.0)
	b.Logf("maxLatency: %v", maxLatency)
}

func randomBytes(length int) []byte {
	rand.Seed(time.Now().UnixNano())
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic("Cannot create random bytes")
	}
	return randomBytes
}

func transactionPages(txn *Transaction) (pages []page) {
	page := txn.headPage
	for page != nil {
		pages = append(pages, *page)
		page = page.nextPage
	}
	return
}

////BenchmarkDiskWrites1 starts benchmarkDiskWrites with 9990 threads, 4kib
////pages and overwrites those pages once
//func BenchmarkDiskWrites1(b *testing.B) {
//	fmt.Println("\n============")
//	benchmarkDiskWrites(b, 1, 4096, 9990)
//}
//
//
////BenchmarkDiskWrites4 starts benchmarkDiskWrites with 9990 threads, 4kib
////pages and overwrites those pages 4 times
//func BenchmarkDiskWrites4(b *testing.B) {
//	fmt.Println("\n==============")
//	benchmarkDiskWrites(b, 4, 4096, 9990)
//}
//
////benchmarkDiskWrites writes numThreads pages of pageSize size and spins up 1
////goroutine for each page that overwrites it numWrites times
//func benchmarkDiskWrites(b *testing.B, numWrites int, pageSize int, numThreads int) {
//	b.Logf("Starting benchmark with %v writes and %v threads for pages of size %v",
//		numWrites, numThreads, pageSize)
//
//	// Get a tmp dir path
//	tmpdir := tempDir(b.Name())
//
//	// Create dir
//	err := os.MkdirAll(tmpdir, 0700)
//	if err != nil {
//		b.Fatal(err)
//	}
//
//	// Create a tmp file
//	f, err := os.Create(filepath.Join(tmpdir, "wal.dat"))
//	if err != nil {
//		b.Fatal(err)
//	}
//
//	// Close it after test
//	defer f.Close()
//
//	// Write numThreads pages to file
//	_, err = f.Write(randomBytes(pageSize * numThreads))
//	if err != nil {
//		b.Fatal(err)
//	}
//
//	// Sync it
//	if err = f.Sync(); err != nil {
//		b.Fatal(err)
//	}
//
//	// Define random page data
//	data := randomBytes(pageSize)
//
//	// Declare a waitGroup for later
//	var wg sync.WaitGroup
//
//	// Count errors during execution
//	var atomicCounter uint64
//
//	// Declare a function that writes a page at the offset i * pageSize 4 times
//	write := func(i int) {
//		defer wg.Done()
//		for j := 0; j < numWrites; j++ {
//			if _, err = f.WriteAt(data, int64(i*pageSize)); err != nil {
//				atomic.AddUint64(&atomicCounter, 1)
//				return
//			}
//			if err = f.Sync(); err != nil {
//				atomic.AddUint64(&atomicCounter, 1)
//				return
//			}
//		}
//	}
//
//	// Reset the timer
//	b.ResetTimer()
//
//	// Create one thread for each page and make it overwrite the page and call sync
//	for i := 0; i < numThreads; i++ {
//		wg.Add(1)
//		go write(i)
//	}
//
//	// Wait for the threads and check if they were successfull
//	wg.Wait()
//	if atomicCounter > 0 {
//		b.Fatalf("%v errors happened during execution", atomicCounter)
//	}
//	// Get fileinfo
//	info, err := f.Stat()
//	if err != nil {
//		b.Fatal(err)
//	}
//
//	// Print some info
//	b.Logf("Number of threads: %v", numThreads)
//	b.Logf("PageSize: %v bytes", pageSize)
//	b.Logf("Filesize after benchmark %v", info.Size())
//}
