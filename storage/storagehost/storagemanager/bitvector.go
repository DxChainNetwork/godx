package storagemanager

type BitVector uint64

// check if the value at given index is free
func (vec BitVector) isFree(idx uint16) bool {
	var mask BitVector = 1 << idx
	value := vec & mask
	return value>>idx == 0
}

func (vec *BitVector) setUsage(idx uint16) {
	var mask BitVector = 1 << idx
	*vec = *vec | mask
}

func (vec *BitVector) clearUsage(idx uint16) {
	var mask BitVector = 1 << 16
	mask = mask - 1<<idx
	*vec = *vec & mask
}
