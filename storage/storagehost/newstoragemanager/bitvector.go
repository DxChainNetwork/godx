package newstoragemanager

// BitVector is used to represent a boolean vector of size 64
// the decimal number is considered as binary, and each bit
// indicating the true or false at an index
type BitVector uint64

// isFree check if the value at given index is free
func (vec BitVector) isFree(idx uint16) bool {
	var mask BitVector = 1 << idx
	value := vec & mask
	return value>>idx == 0
}

// setUsage set given index to 1
func (vec *BitVector) setUsage(idx uint16) {
	var mask BitVector = 1 << idx
	*vec = *vec | mask
}

// clearUsage clear given index to 0
func (vec *BitVector) clearUsage(idx uint16) {
	var mask BitVector = 1 << 16
	mask = mask - 1<<idx
	*vec = *vec & mask
}
