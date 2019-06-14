package common

import (
	"sync"
	"testing"
	"time"
)

func TestTryLock_LockUnlock(t *testing.T) {
	var wg sync.WaitGroup
	var lock sync.RWMutex

	triggered := false
	var tLock TryLock

	tLock.Lock()
	a := 10

	go func() {
		wg.Add(1)

		// add Lock to solve race condition
		lock.Lock()
		triggered = true
		lock.Unlock()

		tLock.Lock()
		a = 1000
		tLock.Unlock()
		wg.Done()
	}()

CONDITION:
	for {
		select {
		case <-time.After(time.Second):
			// add lock to solve race condition
			lock.Lock()
			if !triggered {
				continue CONDITION
			}
			lock.Unlock()

			if a != 10 {
				t.Fatalf("lock does not work, the value of `a` should still be 10 instead of %v", a)
			}
			tLock.Unlock()
			break CONDITION
		}
	}

	wg.Wait()

	if a != 1000 {
		t.Fatalf("unlock does not work, the value of `a` should be changed to 1000, instead of %v", a)
	}
}

func TestTryLock_TryToLock(t *testing.T) {
	var tLock TryLock
	locked := tLock.TryLock()
	if !locked {
		t.Fatalf("the TryLock should be locked")
	}

	locked = tLock.TryLock()
	if locked {
		t.Fatalf("the TryLock should be failed to lock")
	}

	tLock.Unlock()
}
