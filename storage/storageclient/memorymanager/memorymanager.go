package memorymanager

import (
	"github.com/DxChainNetwork/godx/log"
	"sync"
)

// memoryManager manages the memory requested by the user,
// blocking any process which needs memory but memory available is not enough
// once finished using the memory, those memory need to be returned
type MemoryManager struct {
	available        uint64
	limit            uint64
	underflow        uint64
	waitlist         []*memoryRequest
	priorityWaitlist []*memoryRequest
	lock             sync.Mutex
	stop             <-chan struct{}
}

// memoryRequest defines the amount memory requested
// and whether if this request is accomplished successfully
type memoryRequest struct {
	amount uint64
	done   chan struct{}
}

// newMemoryManager create and initialize new memory manager object used to acquire
// memory. If the amount of memory required is not available, the process will be blocked
// until memory became available
func New(limit uint64, stopChan <-chan struct{}) *MemoryManager {
	return &MemoryManager{
		available: limit,
		limit:     limit,
		stop:      stopChan,
	}
}

// Request will try to get memory requested, return true if succeed
// if failed, the storageClient process will be blocked until the memory
// can be allocated
func (mm *MemoryManager) Request(amount uint64, priority bool) bool {
	mm.lock.Lock()

	// try to get the memory requested
	if len(mm.waitlist) == 0 && mm.try(amount) {
		mm.lock.Unlock()
		return true
	}

	// not enough memory, add the request into waitlist
	memRequest := &memoryRequest{
		amount: amount,
		done:   make(chan struct{}),
	}

	if priority {
		mm.priorityWaitlist = append(mm.priorityWaitlist, memRequest)
	} else {
		mm.waitlist = append(mm.waitlist, memRequest)
	}
	mm.lock.Unlock()

	// block until memory is available
	select {
	case <-memRequest.done:
		return true
	case <-mm.stop:
		return false
	}
}

// Return will return memory requested and processing memory requests in the waitlist
func (mm *MemoryManager) Return(amount uint64) {
	mm.lock.Lock()
	defer mm.lock.Unlock()

	// return memory requested, give back underflowing memory if needed
	if mm.underflow > 0 && amount <= mm.underflow {
		mm.underflow -= amount
		return
	} else if mm.underflow > 0 && amount > mm.underflow {
		amount -= mm.underflow
		mm.underflow = 0
	}

	mm.available += amount

	mm.waitlistCheck()
}

// try will try to get memory requested, return true if succeed
// 1. if there are enough memory, return true
// 2. if there are not enough memory, and memory is not used at all, return true as well
func (mm *MemoryManager) try(amount uint64) bool {
	if mm.available >= amount {
		mm.available -= amount
		return true
	} else if mm.available == mm.limit {
		// give all the memory requested, record underflow memory amount
		mm.available = 0
		mm.underflow = amount - mm.limit
		return true
	}
	return false
}

// waitlistCheck will check and handle memory requests stored in the waitlist and priority waitlist
func (mm *MemoryManager) waitlistCheck() {
	// available memory validation
	if mm.available > mm.limit {
		log.Warn("returned too many memory, possibly caused by max memory allowed shrinking")
		mm.available = mm.limit
	}

	// try to process memory requests stored in the priority waitlist
	for len(mm.priorityWaitlist) > 0 {
		if !mm.try(mm.priorityWaitlist[0].amount) {
			return
		}
		close(mm.priorityWaitlist[0].done)
		mm.priorityWaitlist = mm.priorityWaitlist[1:]
	}

	// try to process memory requests stored in the waitlist
	for len(mm.waitlist) > 0 {
		if !mm.try(mm.priorityWaitlist[0].amount) {
			return
		}
		close(mm.waitlist[0].done)
		mm.waitlist = mm.waitlist[1:]
	}
}
