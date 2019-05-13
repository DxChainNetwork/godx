package dxfile

import "testing"

//func TestDxFile_SegmentHealth(t *testing.T) {
//	tests := []struct {
//		numSectors uint32
//		minSectors uint32
//		numOnlineNotRenew int
//		numRenewOffline int
//		numRenewOnline int
//		expectHealth Health
//	} {
//		{
//			numSectors: 30,
//			minSectors: 10,
//			numOnlineNotRenew: 10,
//			numRenewOffline: 10,
//
//		},
//	}
//}

func TestCmpHealth(t *testing.T) {
	tests := []struct {
		h1  uint32
		h2  uint32
		res int
	}{
		{200, 200, 0},
		{200, 100, -1},
		{100, 200, 1},
		{100, 99, -1},
		{99, 100, 1},
		{199, 100, 1},
		{100, 199, -1},
		{0, 99, -1},
		{99, 0, 1},
	}
	for _, test := range tests {
		res := cmpHealth(test.h1, test.h2)
		if res != test.res {
			t.Errorf("compare health unexpected value: %d, %d -> %d", test.h1, test.h2, res)
		}
	}
}

func TestIsStuckHealth(t *testing.T) {
	tests := []struct {
		health uint32
		stuck  bool
	}{
		{99, true},
		{200, false},
	}
	for _, test := range tests {
		if res := isStuckHealth(test.health); res != test.stuck {
			t.Errorf("unexpected %d -> %v", test.health, res)
		}
	}
}
