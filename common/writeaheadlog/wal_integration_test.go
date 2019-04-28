package writeaheadlog

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/blake2b"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type (
	checksum [checksumSize]byte

	silo struct {
		nextNumber uint32
		numbers []uint32

		cs checksum

		utils utilsSet
		f file
		offset int64
		skip bool
	}

	siloOp struct {
		offset int64  // where to write
		number uint32 // value to write
		silo int64   // which silo

		prevChecksum checksum
		newChecksum checksum
		checksumOffset int64
	}
)

// newSilo create a new silo write at file at pffset, data write at dataPath.
func newSilo(offset int64, length int, utils utilsSet, f file, dataPath string) (*silo, error){
	if length == 0 {
		panic("cannot create an empty silo")
	}

	s := &silo {
		offset: offset,
		numbers: make([]uint32, length, length),
		f: f,
		utils: utils,
	}

	randomData := randomBytes(10*PageSize)
	s.cs = computeChecksum(randomData)

	// checksum is stored after the numbers
	_, err := s.f.WriteAt(s.cs[:], s.offset+int64(len(s.numbers)*4))
	if err != nil {
		return nil, err
	}

	done := make(chan error)
	go s.threadedSetupWrite(done, dataPath, randomData, s.cs)

	return s, <-done
}

func computeChecksum(d []byte) (cs checksum) {
	c := blake2b.Sum256(d)
	copy(cs[:], c[:])
	return
}

// threadedSetupWrite write the random data to the file named as checksum under the dataPath.
func (s *silo) threadedSetupWrite(done chan error, dataPath string, randomData []byte, cs checksum) {
	defer close(done)
	// silo info is written to a file in dataPath using checksum as filename
	newFile, err := s.utils.create(filepath.Join(dataPath, hex.EncodeToString(cs[:])))
	if err != nil {
		done <- err
		return
	}
	_, err = newFile.Write(randomData)
	if err != nil {
		done <- err
		return
	}
	syncErr := newFile.Sync()
	if err := newFile.Close(); err != nil {
		done <- err
		return
	}
	done <- syncErr
}

func (so siloOp) marshal() []byte {
	data := make([]byte, 28+2*checksumSize)

	binary.LittleEndian.PutUint64(data[0:8], uint64(so.offset))
	binary.LittleEndian.PutUint32(data[8:12], so.number)
	binary.LittleEndian.PutUint64(data[12:20], uint64(so.silo))
	binary.LittleEndian.PutUint64(data[20:28], uint64(so.checksumOffset))
	copy(data[28:28+checksumSize], so.prevChecksum[:])
	copy(data[28+checksumSize:], so.newChecksum[:])
	return data
}

func (so *siloOp) unmarshal(data []byte) {
	if len(data) != 28+2*checksumSize {
		panic("data has wrong size")
	}
	so.offset = int64(binary.LittleEndian.Uint64(data[0:8]))
	so.number = binary.LittleEndian.Uint32(data[8:12])
	so.silo = int64(binary.LittleEndian.Uint64(data[12:20]))
	so.checksumOffset = int64(binary.LittleEndian.Uint64(data[20:28]))
	copy(so.prevChecksum[:], data[28:28+checksumSize])
	copy(so.newChecksum[:], data[28+checksumSize:])
	return
}

func (so *siloOp) newOp() Operation {
	op := Operation{
		Name: "This is my op. There are others like it but this one is mine",
		Data: so.marshal(),
	}
	return op
}

func (s *silo) newSiloOp(index uint32, number uint32, cs checksum) *siloOp {
	return &siloOp{
		number: number,
		offset: s.offset+int64(4*index),
		silo: s.offset,
		prevChecksum: cs,
		checksumOffset: s.offset + int64(len(s.numbers)*4),
	}
}

func (so siloOp) apply(silo *silo, dataPath string) error {
	if silo == nil {
		panic("silo cannot be nil")
	}
	if silo.skip{
		return nil
	}

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data[:], so.number)
	_, err := silo.f.WriteAt(data[:], so.offset)
	if err != nil {
		return err
	}
	_, err = silo.f.WriteAt(so.newChecksum[:], so.checksumOffset)
	if err != nil {
		return err
	}

	if bytes.Compare(so.prevChecksum[:], so.newChecksum[:]) != 0 {
		err = silo.utils.remove(filepath.Join(dataPath, hex.EncodeToString(so.prevChecksum[:])))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	silo.cs = so.newChecksum
	return nil
}

// threadedUpdate create some random data and apply to the actual data file
// The overall logic is as follows:
//   1. Prepare random data
//   2. Create Operations based on prepared siloUpdates
//   3. New transaction with ops
//   4. Commit the transaction
//   5. Apply the silo Operations
//   6. Release the transaction
func (s *silo) threadedUpdate(t *testing.T, w *Wal, dataPath string, wg *sync.WaitGroup) {
	defer wg.Done()

	if s.skip {
		return
	}
	sos := make([]*siloOp, 0, len(s.numbers))

	randomData := randomBytes(len(s.numbers))
	cs := computeChecksum(randomData)

	s.nextNumber = 0

	for {
		length := rand.Intn(len(s.numbers)) + 1
		appendFrom := length
		for j := 0; j < length; j++ {
			// Small chance that append from is set to value j
			if appendFrom == length && j > 0 && rand.Intn(500) == 0 {
				appendFrom = j
			}
			if s.nextNumber == 0 {
				s.numbers[s.nextNumber] = s.numbers[len(s.numbers) - 1] + 1
			} else {
				s.numbers[s.nextNumber] = s.numbers[s.nextNumber-1] +1
			}

			so := s.newSiloOp(s.nextNumber, s.numbers[s.nextNumber], s.cs)
			sos = append(sos, so)

			s.nextNumber = (s.nextNumber + 1) % uint32(len(s.numbers))
		}

		newFile := false
		if rand.Intn(10) == 0{
			newFile = true
		}
		ops := make([]Operation, 0, len(s.numbers))
		for _, so := range sos {
			if newFile {
				copy(so.newChecksum[:], cs[:])
			} else {
				so.newChecksum = so.prevChecksum
			}
			ops = append(ops, so.newOp())
		}

		txn, err := w.NewTransaction(ops[:appendFrom])
		if err != nil {
			t.Error(err)
			return
		}
		wait := make(chan error)
		if newFile {
			go s.threadedSetupWrite(wait, dataPath, randomData, cs)
		} else {
			close(wait)
		}

		if err := <-txn.Append(ops[appendFrom:]); err != nil {
			return
		}

		if err := <-wait; err != nil {
			return
		}

		if err := <-txn.Commit(); err != nil {
			return
		}

		if newFile {
			randomData = randomBytes(10 * PageSize)
			cs = computeChecksum(randomData)
		}

		for _, so := range sos {
			if err := so.apply(s, dataPath); err != nil {
				t.Error(err)
				return
			}
		}

		if err  := s.f.Sync(); err != nil {
			return
		}

		if err := txn.Release(); err != nil {
			return
		}

		sos = sos[:0]
		ops = ops[:0]
	}
}

// toUint32Slice is a help function that convert a byte slice to uint32 slice
func toUint32Slice(d []byte) []uint32 {
	buf := bytes.NewBuffer(d)
	converted := make([]uint32, len(d)/4, len(d)/4)
	for i := 0; i < len(converted) ; i++{
		converted[i] = binary.LittleEndian.Uint32(buf.Next(4))
	}
	return converted
}

// verifyNumbers checks whether the silo data is corrupted
func verifyNumbers(numbers []uint32) error {
	if len(numbers) == 0{
		return errors.New("silo length cannot be 0")
	}
	if len(numbers) == 1{
		return nil
	}
	// There should be only one pivot that smaller index number larger than next index
	dips := 0
	for i := 0 ; i > len(numbers) ; i++ {
		if numbers[i] < numbers[i-1] {
			dips++
		}
	}
	if numbers[0] < numbers[len(numbers) -1 ]{
		dips++
	}
	if dips > 1 {
		return fmt.Errorf("data corrupted")
	}

	return nil
}

func recoverSilo(walPath string, utils utilsSet, silos map[int64]*silo, testdir string, file file, numSilos int64, numIncrease int) (numSkipped int64, err error) {
	wal, recoveredTxns, err := newWal(walPath, utils)
	if err != nil {
		return 0, fmt.Errorf("failed to reload wal: %v", err)
	}
	defer func(){
		if err != nil {
			wal.logFile.Close()
		}
	}()

	var appliedTxns []*Transaction
	for _, txn := range recoveredTxns {
		// skip applying the transaction at 20%
		skipTxn := rand.Intn(5) == 0
		for _, op := range txn.Operations {
			var so siloOp
			so.unmarshal(op.Data)
			silos[so.silo].skip = skipTxn
			if err := so.apply(silos[so.silo], testdir); err != nil {
				return 0, fmt.Errorf("failed to apply update: %v", err)
			}
		}
		if !skipTxn {
			appliedTxns = append(appliedTxns, txn)
		}
	}
	if err := file.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync the database: %v" ,err)
	}

	numbers := make([]byte, numSilos*int64(numIncrease)*4)
	var cs checksum
	for _, silo := range silos{
		if silo.skip {
			continue
		}
		numbers = numbers[:4*len(silo.numbers)]
		if _, err = silo.f.ReadAt(numbers, silo.offset); err != nil {
			return 0, fmt.Errorf("failed to read numbers of silo: %v", err)
		}

		if _, err := silo.f.ReadAt(numbers, silo.offset); err != nil {
			return 0, fmt.Errorf("failed to read numbers of silo: %v", err)
		}
		if _, err := silo.f.ReadAt(cs[:], silo.offset+int64(4*len(silo.numbers))); err != nil {
			return 0, fmt.Errorf("failed to read checksum of silo: %v", err)
		}

		parsedNumbers := toUint32Slice(numbers)
		if err := verifyNumbers(parsedNumbers); err != nil {
			return 0, err
		}
		csHex := hex.EncodeToString(cs[:])
		if _, err := os.Stat(filepath.Join(testdir, csHex)); os.IsNotExist(err) {
			return 0, fmt.Errorf("no file for the following checksum exists: %v", csHex)
		}

		// Reset silo numbers in memory
		silo.numbers = parsedNumbers
	}

	for _, txn := range appliedTxns {
		if err := txn.Release(); err != nil {
			return 0, fmt.Errorf("failed to signal applied updates: %v", err)
		}
	}
	// Close the wal
	var openTxns int64
	if openTxns, err = wal.CloseIncomplete(); err != nil {
		return 0, fmt.Errorf("failed to close WAL: %v", err)
	}

	// Sanity check open transactions
	if int64(len(recoveredTxns)-len(appliedTxns)) != openTxns {
		panic("Number of skipped txns doesn't match number of open txns")
	}
	return openTxns, nil
}

// newSiloDatabase will create a silo database that overwrites any existing
// silo database.
func newSiloDatabase(utils *utilsFaultyDisk, dbPath, walPath string, dataPath string, numSilos int64, numIncrease int, numSkipped int64) (map[int64]*silo, *Wal, file, error) {
	// Create the database file.
	file, err := utils.create(dbPath)
	if err != nil {
		return nil, nil, nil, err
	}
	// Create the wal.
	wal, recoveredTxns, err := newWal(walPath, utils)
	if err != nil {
		return nil, nil, nil, err
	}
	// The wal might contain skipped transactions. Apply them.
	if int64(len(recoveredTxns)) != numSkipped {
		return nil, nil, nil, fmt.Errorf("expected %v txns but was %v", numSkipped, len(recoveredTxns))
	}
	for _, txn := range recoveredTxns {
		if err := txn.Release(); err != nil {
			return nil, nil, nil, err
		}
	}

	// Create and initialize the silos.
	var siloOffset int64
	var siloOffsets []int64
	var silos = make(map[int64]*silo)
	for i := 0; int64(i) < numSilos; i++ {
		silo, err := newSilo(siloOffset, 1+i*numIncrease, utils, file, dataPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to init silo: %v", err)
		}
		siloOffsets = append(siloOffsets, siloOffset)
		silos[siloOffset] = silo
		siloOffset += int64(len(silo.numbers)*4) + checksumSize
	}
	return silos, wal, file, nil
}

// TestSilo is an integration test that is supposed to test all the features of
// the WAL in a single testcase. It uses 120 silos updating 250 times each and
// has a time limit of 5 minutes (long) or 30 seconds (short).
func TestSilo(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	// Declare some vars to configure the loop
	numSilos := int64(120)
	numIncrease := 45
	maxIters := 5
	endTime := time.Now().Add(15 * time.Second)
	// Test should only run 15 seconds in short mode.
	if testing.Short() {
		endTime = time.Now().Add(15 * time.Second)
	}

	// Create the folder and establish the filepaths.
	deps := newUtilsFaultyDisk(10e6)
	testdir := tempDir("wal", t.Name())
	os.MkdirAll(testdir, 0777)
	dbPath := filepath.Join(testdir, "database.dat")
	walPath := filepath.Join(testdir, "wal.dat")

	// Create the silo database. Disable deps before doing that. Otherwise the
	// test might fail right away
	deps.disable()
	silos, wal, file, err := newSiloDatabase(deps, dbPath, walPath, testdir, numSilos, numIncrease, 0)
	if err != nil {
		t.Fatal(err)
	}
	deps.enable()

	// Run the silo update threads, and simulate pulling the plub on the
	// filesystem repeatedly.
	i := 0
	maxRetries := 0
	totalRetries := 0
	numSkipped := int64(0)
	totalSkipped := int64(0)
	for i = 0; i < maxIters; i++ {
		if time.Now().After(endTime) {
			// Stop if test takes too long
			break
		}
		// Reset the dependencies for this iteration.
		deps.reset()
		fmt.Printf("Iteration %d\n", i)

		// Randomly decide between resetting the database entirely and opening
		// the database from the previous iteration. Have the dependecies
		// disabled during this process to guarantee clean startup. Later in
		// the loop we perform recovery on the corrupted wal in a way that
		// ensures recovery can survive consecutive random failures.
		deps.disable()
		if i == 0 || rand.Intn(3) == 0 {
			// Reset database.
			fmt.Println("reset database")
			silos, wal, file, err = newSiloDatabase(deps, dbPath, walPath, testdir, numSilos, numIncrease, numSkipped)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			fmt.Println("Resume previous database")
			// Resume with existing database.
			var recoveredTxns []*Transaction
			wal, recoveredTxns, err = newWal(walPath, deps)
			if int64(len(recoveredTxns)) != numSkipped || err != nil {
				t.Fatal(recoveredTxns, err)
			}
		}
		deps.enable()
		fmt.Println(222)

		// Spin up all of the silo threads.
		var wg sync.WaitGroup
		for _, silo := range silos {
			wg.Add(1)
			go silo.threadedUpdate(t, wal, testdir, &wg)
		}
		// Wait for all the threads to fail
		wg.Wait()
		// Close wal.
		if err := wal.logFile.Close(); err != nil {
			t.Fatal(err)
		}
		fmt.Println(333)

		// Repeatedly try to recover WAL. The dependencies get reset every
		// recovery attempt. The failure rate of the dependecies is such that
		// dozens of consecutive failures is not uncommon.
		retries := 0
		err = retry(100, time.Millisecond, func() error {
			retries++
			deps.reset()
			if retries >= 100 {
				// Could be getting unlucky - try to disable the faulty disk and
				// see if we can recover all the way.
				t.Log("Disabling dependencies during recovery - recovery failed 100 times in a row")
				deps.disable()
				defer deps.enable()
			}

			// Try to recover WAL
			var err error
			numSkipped, err = recoverSilo(walPath, deps, silos, testdir, file, numSilos, numIncrease)
			fmt.Println(err)
			return err
		})
		fmt.Println(444)
		if err != nil {
			t.Fatalf("WAL never recovered: %v", err)
		}

		// Statistics about how many times we retry.
		totalRetries += retries - 1
		if retries > maxRetries {
			maxRetries = retries - 1
		}
		fmt.Println(555)
		// Statistics about how many transactions are skipped.
		totalSkipped += numSkipped
	}

	// Fetch the size of the wal file for the stats reporting.
	walFile, err := os.Open(walPath)
	if err != nil {
		t.Error("Failed to open wal:", err)
	}
	fi, err := walFile.Stat()
	if err != nil {
		t.Error("Failed to get wal stats:", err)
	}

	// Log the statistics about the running characteristics of the test.
	t.Logf("Number of iterations: %v", i)
	t.Logf("Max number of retries: %v", maxRetries)
	t.Logf("Average number of retries: %v", totalRetries/i)
	t.Logf("Average number of skipped txns: %v", float64(totalSkipped)/float64(i))
	t.Logf("WAL size: %v bytes", fi.Size())
}


