package storagemanager

import (
	"errors"
	"math"
	"math/rand"
	"time"
)

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
