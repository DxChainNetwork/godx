package storagehostmanager

import "testing"

func TestStorageHostManager_AgeAdjustment(t *testing.T) {
	shm := newHostManagerTestData()
	hi := hostInfoGenerator()

	tables := []struct {
		blockHeight uint64
		firstSeen   uint64
		result      float64
	}{
		{uint64(1), uint64(10), float64(1)},
		{uint64(10), uint64(10), float64(1) * 2 / 3 / 2 / 2 / 2 / 3 / 3 / 3 / 3},
	}

	for _, table := range tables {
		shm.blockHeight = table.blockHeight
		hi.FirstSeen = table.firstSeen
		adjustment := shm.ageAdjustment(hi)
		if adjustment != table.result {
			t.Errorf("error, blockHeight %v, firstseen %v. expected %v, got %v",
				table.blockHeight, table.firstSeen, table.result, adjustment)
		}
	}
}
