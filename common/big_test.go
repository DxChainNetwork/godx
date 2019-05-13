package common

import "testing"

func TestCmp(t *testing.T) {
	tables := []struct {
		a      int64
		b      int64
		result int
	}{
		{200, 100, 1},
		{100, 200, -1},
		{100, 100, 0},
	}

	for _, table := range tables {
		val := NewBigInt(table.a).Cmp(NewBigInt(table.b))
		if val != table.result {
			t.Errorf("error comparing between %d and %d, expected %d, got %d",
				table.a, table.b, table.result, val)
		}
	}
}
