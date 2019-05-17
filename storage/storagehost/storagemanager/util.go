package storagemanager

import (
	"errors"
	"math"
	"math/rand"
	"time"
)

// TODO: utils random function all need exhausted testing

// RandomFolderIndex get a randoms folder index from 0 (include)
// to 65535 (include). The algorithm aim at finding a unused index
// which is the closet to the random number
func randomFolderIndex(folders map[uint16]*storageFolder) (uint16, error) {
	// generate a random seeds by time
	// TODO: check if seeds could be handle in better way
	rand.Seed(time.Now().UTC().UnixNano())

	// start with two pointers, ptr1 get a random number
	// ptr2 point to the number larger than ptr1
	ptr1 := rand.Intn(math.MaxUint16)
	ptr2 := ptr1

	// first loop try to extends pointers as much as possible
	for ptr1 >= 0 && ptr2 < math.MaxUint16 {
		if _, exits := folders[uint16(ptr1)]; !exits {
			return uint16(ptr1), nil
		}
		ptr1--
		if _, exits := folders[uint16(ptr2)]; !exits {
			return uint16(ptr2), nil
		}
		ptr2++
	}

	// find the number in this range that is left over
	for ptr1 >= 0 {
		if _, exits := folders[uint16(ptr1)]; !exits {
			return uint16(ptr1), nil
		}
		ptr1--
	}

	// find the number in this range that is left over
	for ptr2 < math.MaxUint16 {
		if _, exits := folders[uint16(ptr2)]; !exits {
			return uint16(ptr2), nil
		}
		ptr2++
	}

	// if there is unused index, return an error to it
	return uint16(0), errors.New("folders reach the maximum limitation")
}

func randomAvailableFolder(sfs []*storageFolder) (*storageFolder, int) {
	// TODO: check if seeds could be handle in better way
	rand.Seed(time.Now().UTC().UnixNano())

	// permute the index in random order
	for _, index := range rand.Perm(len(sfs)) {
		sf := sfs[index]

		// if the number of sector in the folder already reach the maximum number
		if sf.sectors >= uint64(len(sf.usage))*granularity {
			continue
		}

		// if the folder is already locked
		if !sf.folderLock.TryLock() {
			continue
		}

		return sf, index
	}

	return nil, -1
}

// return: the index [0, len(usage) == number of group of sector], error
func randomFreeSector(usage []BitVector) (uint32, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	var idx = -1
	// permute the usage in random order
	for _, i := range rand.Perm(len(usage)) {
		// find the first one which the usage not filled
		if usage[i] != math.MaxUint64 {
			idx = i
			break
		}
	}

	// if all things are filled, through exception
	if idx == -1 {
		return 0, errors.New("the given usage are all filled")
	}

	// find the usage which is free

	// start with two pointers, ptr1 get a random number
	// ptr2 point to the number larger than ptr1
	ptr1 := rand.Intn(granularity)
	ptr2 := ptr1

	// first loop try to extends pointers as much as possible
	for ptr1 >= 0 && ptr2 < granularity {
		// if ptr1 is free
		if usage[idx].isFree(uint16(ptr1)) {
			return uint32(idx*granularity + ptr1), nil
		}
		ptr1--

		// check if ptr2 is free
		if usage[idx].isFree(uint16(ptr2)) {
			return uint32(idx*granularity + ptr2), nil
		}

		ptr2++
	}

	// find the number in this range that is left over
	for ptr1 >= 0 {
		// if ptr1 is free
		if usage[idx].isFree(uint16(ptr1)) {
			return uint32(idx*granularity + ptr1), nil
		}
		ptr1--
	}

	// find the number in this range that is left over
	for ptr2 < math.MaxUint16 {
		// check if ptr2 is free
		if usage[idx].isFree(uint16(ptr2)) {
			return uint32(idx*granularity + ptr2), nil
		}

		ptr2++
	}

	// if at the end, nothing could be found
	return 0, errors.New("the given usage are all filled")
}
