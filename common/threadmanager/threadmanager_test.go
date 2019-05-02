package threadmanager

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestThreadManagerStopEarly tests that a thread group can correctly interrupt
// an ongoing process.
func TestThreadManagerStopEarly(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	var tg ThreadManager
	for i := 0; i < 10; i++ {
		err := tg.Add()
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			defer tg.Done()
			select {
			case <-time.After(1 * time.Second):
			case <-tg.GetStopChan():
			}
		}()
	}
	start := time.Now()
	err := tg.Stop()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	} else if elapsed > 500*time.Millisecond {
		t.Fatal("Stop did not interrupt goroutines")
	}
}

// TestThreadManagerWait tests that a thread group will correctly wait for
// existing processes to halt.
func TestThreadManagerWait(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	var tg ThreadManager
	for i := 0; i < 10; i++ {
		err := tg.Add()
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			defer tg.Done()
			time.Sleep(time.Second)
		}()
	}
	start := time.Now()
	err := tg.Stop()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	} else if elapsed < time.Millisecond*950 {
		t.Fatal("Stop did not wait for goroutines:", elapsed)
	}
}

// TestThreadManagerStop tests the behavior of a ThreadManager after Stop has been
// called.
func TestThreadManagerStop(t *testing.T) {
	// Create a thread group and stop it.
	var tg ThreadManager
	// Create an array to track the order of execution for PushStopFn and PushDeferFn
	// calls.
	var stopCalls []int

	// isOff should return false
	if tg.isOff() {
		t.Error("isOff returns true on unstopped ThreadManager")
	}
	// The cannel provided by GetStopChan should be open.
	select {
	case <-tg.GetStopChan():
		t.Error("stop chan appears to be closed")
	default:
	}

	// PushStopFn and PushDeferFn should queue their functions, but not call them.
	// 'Add' and 'Done' are setup around the PushStopFn functions, to make sure
	// that the PushStopFn functions are called before waiting for all calls to
	// 'Done' to come through.
	//
	// Note: the practice of calling Add outside of PushStopFn and Done inside of
	// PushStopFn is a bad one - any call to tg.Flush() will cause a deadlock
	// because the stop functions will not be called but tg.Flush will be
	// waiting for the thread group counter to reach zero.
	err := tg.Add()
	if err != nil {
		t.Fatal(err)
	}
	err = tg.Add()
	if err != nil {
		t.Fatal(err)
	}
	err = tg.PushStopFn(func() error {
		tg.Done()
		stopCalls = append(stopCalls, 1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = tg.PushStopFn(func() error {
		tg.Done()
		stopCalls = append(stopCalls, 2)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = tg.PushDeferFn(func() error {
		stopCalls = append(stopCalls, 10)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = tg.PushDeferFn(func() error {
		stopCalls = append(stopCalls, 20)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// None of the stop calls should have been called yet.
	if len(stopCalls) != 0 {
		t.Fatal("Stop calls were called too early")
	}

	// Stop the thread group.
	err = tg.Stop()
	if err != nil {
		t.Fatal(err)
	}
	// isOff should return true.
	if !tg.isOff() {
		t.Error("isOff returns false on stopped ThreadManager")
	}
	// The cannel provided by GetStopChan should be closed.
	select {
	case <-tg.GetStopChan():
	default:
		t.Error("stop chan appears to be closed")
	}
	// The PushStopFn calls should have been called first, in reverse order, and
	// the PushDeferFn calls should have been called second, in reverse order.
	if len(stopCalls) != 4 {
		t.Fatal("Stop did not call the stopping functions correctly")
	}
	if stopCalls[0] != 2 {
		t.Error("Stop called the stopping functions in the wrong order")
	}
	if stopCalls[1] != 1 {
		t.Error("Stop called the stopping functions in the wrong order")
	}
	if stopCalls[2] != 20 {
		t.Error("Stop called the stopping functions in the wrong order")
	}
	if stopCalls[3] != 10 {
		t.Error("Stop called the stopping functions in the wrong order")
	}

	// Add and Stop should return errors.
	err = tg.Add()
	if err != ErrStopped {
		t.Error("expected ErrStopped, got", err)
	}
	err = tg.Stop()
	if err != ErrStopped {
		t.Error("expected ErrStopped, got", err)
	}

	// PushStopFn and PushDeferFn should call their functions immediately now that
	// the thread group has stopped.
	onStopCalled := false
	err = tg.PushStopFn(func() error {
		onStopCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("PushStopFn should return an error after being called after stop")
	}

	if !onStopCalled {
		t.Error("PushStopFn function not called immediately despite the thread group being closed already.")
	}
	afterStopCalled := false
	err = tg.PushDeferFn(func() error {
		afterStopCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("PushDeferFn should return an error after being called after stop")
	}
	if !afterStopCalled {
		t.Error("PushDeferFn function not called immediately despite the thread group being closed already.")
	}
}

// TestThreadManagerConcurrentAdd tests that Add can be called concurrently with Stop.
func TestThreadManagerpConcurrentAdd(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	var tg ThreadManager
	for i := 0; i < 1000; i++ {
		go func() {
			err := tg.Add()
			if err != nil {
				return
			}
			defer tg.Done()

			select {
			case <-time.After(100 * time.Millisecond):
			case <-tg.GetStopChan():
			}
		}()
	}
	time.Sleep(25 * time.Millisecond)
	err := tg.Stop()
	if err != nil {
		t.Fatal(err)
	}
}

// TestThreadManagerOnce tests that a zero-valued ThreadManager's stopChan is
// properly initialized.
func TestThreadManagerOnce(t *testing.T) {
	tg := new(ThreadManager)
	if tg.stopChan != nil {
		t.Error("expected nil stopChan")
	}

	// these methods should cause stopChan to be initialized
	tg.GetStopChan()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by GetStopChan")
	}

	tg = new(ThreadManager)
	tg.isOff()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by isOff")
	}

	tg = new(ThreadManager)
	tg.Add()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by Add")
	}

	tg = new(ThreadManager)
	tg.Stop()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by Stop")
	}
}

// TestThreadManagerOnStop tests that Stop calls functions registered with
// PushStopFn.
func TestThreadManagerOnStop(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	// create ThreadManager and register the closer
	var tg ThreadManager
	err = tg.PushStopFn(func() error { return l.Close() })
	if err != nil {
		t.Fatal(err)
	}

	// send on channel when listener is closed
	var closed bool
	tg.Add()
	go func() {
		defer tg.Done()
		_, err := l.Accept()
		closed = err != nil
	}()

	tg.Stop()
	if !closed {
		t.Fatal("Stop did not close listener")
	}
}

// TestThreadManagerRace tests that calling ThreadManager methods concurrently
// does not trigger the race detector.
func TestThreadManagerRace(t *testing.T) {
	var tg ThreadManager
	go tg.GetStopChan()
	go func() {
		if tg.Add() == nil {
			tg.Done()
		}
	}()
	err := tg.Stop()
	if err != nil {
		t.Fatal(err)
	}
}

// TestThreadManagerCloseAfterStop checks that an PushDeferFn function is
// correctly called after the thread is stopped.
func TestThreadManagerClosedAfterStop(t *testing.T) {
	var tg ThreadManager
	var closed bool
	err := tg.PushDeferFn(func() error { closed = true; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if closed {
		t.Fatal("close function should not have been called yet")
	}
	if err := tg.Stop(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("close function should have been called")
	}

	// Stop has already been called, so the close function should be called
	// immediately
	closed = false
	err = tg.PushDeferFn(func() error { closed = true; return nil })
	if err == nil {
		t.Fatal("PushDeferFn should return an error after stop")
	}
	if !closed {
		t.Fatal("close function should have been called immediately")
	}
}

// TestThreadManagerNetworkExample tries to use a thread group as it might be
// expected to be used by a networking module.
func TestThreadManagerNetworkExample(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	var tg ThreadManager

	// Open a listener, and queue the shutdown.
	listenerCleanedUp := false
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	// Open a thread to accept calls from the listener.
	handlerFinishedChan := make(chan struct{})
	go func() {
		// Threadgroup shutdown should stall until listener closes.
		err := tg.Add()
		if err != nil {
			// Testing is non-deterministic, sometimes Stop() will be called
			// before the listener fully starts up.
			close(handlerFinishedChan)
			return
		}
		defer tg.Done()

		for {
			_, err := listener.Accept()
			if err != nil {
				break
			}
		}
		close(handlerFinishedChan)
	}()
	err = tg.PushStopFn(func() error {
		err := listener.Close()
		if err != nil {
			return err
		}
		<-handlerFinishedChan

		listenerCleanedUp = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a thread that does some stuff which takes time, and then closes.
	threadFinished := false
	err = tg.Add()
	if err != nil {
		t.Fatal(err)
	}
	go func() error {
		time.Sleep(time.Second)
		threadFinished = true
		tg.Done()
		return nil
	}()

	// Create a thread that does some stuff which takes time, and then closes.
	// Use Stop to wait for the threead to finish and then check that all
	// resources have closed.
	threadFinished2 := false
	err = tg.Add()
	if err != nil {
		t.Fatal(err)
	}
	go func() error {
		time.Sleep(time.Second)
		threadFinished2 = true
		tg.Done()
		return nil
	}()

	// Let the listener run for a bit.
	time.Sleep(100 * time.Millisecond)
	err = tg.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if !threadFinished || !threadFinished2 || !listenerCleanedUp {
		t.Error("stop did not block until all running resources had closed")
	}
}

// TestNestedAdd will call Add repeatedly from the same goroutine, then call
// stop concurrently.
func TestNestedAdd(t *testing.T) {
	var tg ThreadManager
	go func() {
		for i := 0; i < 1000; i++ {
			err := tg.Add()
			if err == nil {
				defer tg.Done()
			}
		}
	}()

	time.Sleep(10 * time.Millisecond)
	tg.Stop()
}

// TestAddOnStop checks that you can safely call PushStopFn from under the
// protection of an Add call.
func TestAddOnStop(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	var tg ThreadManager
	var data int
	addChan := make(chan struct{})
	stopChan := make(chan struct{})
	err := tg.PushStopFn(func() error {
		close(stopChan)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		err := tg.Add()
		if err != nil {
			t.Fatal(err)
		}
		close(addChan)

		// Wait for the call to 'Stop' to be called in the parent thread, and
		// then queue a bunch of 'PushStopFn' and 'PushDeferFn' functions before
		// calling 'Done'.
		<-stopChan
		for i := 0; i < 10; i++ {
			err = tg.PushStopFn(func() error {
				data++
				return nil
			})
			if err == nil {
				t.Fatal("PushStopFn should return an error when being called after stop")
			}
			err = tg.PushDeferFn(func() error {
				data++
				return nil
			})
			if err == nil {
				t.Fatal("PushDeferFn should return an error when being called after stop")
			}
		}
		tg.Done()
	}()

	// Wait for 'Add' to be called in the above thread, to guarantee that
	// PushStopFn and PushDeferFn will be called after 'Add' and 'Stop' have been
	// called together.
	<-addChan
	err = tg.Stop()
	if err != nil {
		t.Fatal(err)
	}

	if data != 20 {
		t.Error("20 calls were made to increment data, but value is", data)
	}
}

// BenchmarkThreadManager times how long it takes to add a ton of threads and
// trigger goroutines that call Done.
func BenchmarkThreadManager(b *testing.B) {
	var tg ThreadManager
	for i := 0; i < b.N; i++ {
		tg.Add()
		go tg.Done()
	}
	tg.Stop()
}

// BenchmarkWaitGroup times how long it takes to add a ton of threads to a wait
// group and trigger goroutines that call Done.
func BenchmarkWaitGroup(b *testing.B) {
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go wg.Done()
	}
	wg.Wait()
}
