package memorymanager

import "fmt"

// MaxMemoryAvailable returns max memory allowed
func (mm *MemoryManager) MemoryLimit() uint64 {
	return mm.limit
}

// MemoryAvailable returns current memory available
func (mm *MemoryManager) MemoryAvailable() uint64 {
	return mm.available
}

// MaxMemoryModify allows user to expand or shrink the current memory limit
func (mm *MemoryManager) SetMemoryLimit(amount uint64) string {
	if amount < mm.limit {
		mm.shrinkMaxMemory(amount)
		return fmt.Sprintf("shrinked the max memory available to %d", amount)
	} else if amount > mm.limit {
		mm.expandMaxMemory(amount)
		return fmt.Sprintf("expaneded the max memory available to %d", amount)
	}

	return "no changes detected, max memory available remains the same"
}

// expandMaxMemory allows user to manually expand the max memory available
//  	1. change the max memory available
//  	2. check and handle memory underflow
//  	3. increase the available memory
//  	4. check waitlist
func (mm *MemoryManager) expandMaxMemory(amount uint64) {

	mm.lock.Lock()
	defer mm.lock.Unlock()

	// 1. change max memory available
	amountIncreased := amount - mm.limit
	mm.limit = amount

	// 2. check and handle memory underflow
	if mm.underflow > 0 {
		if amountIncreased >= mm.underflow {
			amountIncreased -= mm.underflow
			mm.underflow = 0
		} else {
			mm.underflow -= amountIncreased
			amountIncreased = 0
		}
	}

	// 3. increase the available memory
	mm.available += amountIncreased

	// 4. check waitlist
	mm.waitlistCheck()
}

// shrinkMaxMemory allows user to manually shrink max memory available
// 		1. Change max memory available
// 		2. check and handle memory underflow. Change available memory
func (mm *MemoryManager) shrinkMaxMemory(amount uint64) {
	mm.lock.Lock()
	defer mm.lock.Unlock()

	// 1. change max memory available
	amountDecreased := mm.limit - amount
	mm.limit = amount

	// 2. check and handle memory underflow. Change available memory
	if mm.underflow > 0 {
		mm.underflow += amountDecreased
	} else if amountDecreased > mm.available {
		mm.underflow += amountDecreased - mm.available
		mm.available = 0
	} else {
		mm.available -= amountDecreased
	}
}
