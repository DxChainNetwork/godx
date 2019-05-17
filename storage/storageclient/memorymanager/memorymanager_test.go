// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package memorymanager

import (
	"sync"
	"testing"
	"time"
)

var stopChan = make(chan struct{})

func TestMemoryManager_Request_Lock(t *testing.T) {
	done := make(chan struct{}, 1)
	mm := New(10000, stopChan)

	// underflow
	mm.Request(20000, false)
	if mm.available != 0 || mm.underflow != 10000 {
		t.Errorf("error, expected underflow 1000, got %d. expected memory left 0, got %d",
			mm.underflow, mm.available)
	}

	go func() {
		mm.Request(5000, true)
		done <- struct{}{}
	}()

	select {
	case <-done:
		t.Errorf("the memory should lock the process, therefore, this statement should not be triggered")
	case <-time.After(1 * time.Second):
		if len(mm.priorityWaitlist) != 1 {
			t.Errorf("expected to have one memoery request in the priority waitlist, instead, got %d",
				len(mm.priorityWaitlist))
		}
		if mm.priorityWaitlist[0].amount != 5000 {
			t.Errorf("expected to have memoery request with amount 5000, instead, got %d",
				mm.priorityWaitlist[0].amount)
		}
	}
}

func TestMemoryManager_Request_NonLock(t *testing.T) {
	var wg = sync.WaitGroup{}
	lock := true
	mm := New(10000, stopChan)

	mm.Request(5000, false)
	if mm.available != 5000 || mm.underflow != 0 {
		t.Errorf("error, expected underflow 0, got %d. expected memory left 5000, got %d",
			mm.underflow, mm.available)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		mm.Request(5000, true)
		lock = false
	}()

	wg.Wait()

	if lock {
		t.Errorf("error, the process should not be locked")
	}

	if mm.available != 0 || mm.underflow != 0 {
		t.Errorf("error, expected underflow 0, got %d, expected memory left 0, got %d",
			mm.underflow, mm.available)
	}
}

func TestMemoryManager_Return(t *testing.T) {
	done := make(chan struct{}, 1)
	mm := New(100000, stopChan)

	mm.Request(100000, true)

	go func() {
		mm.Request(5000, false)
		done <- struct{}{}
	}()

	mm.Return(100000)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Errorf("error: memory request is expected to be successfully")
	}
}
