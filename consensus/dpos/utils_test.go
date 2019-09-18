// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import "testing"

func TestHashUint64Conversion(t *testing.T) {
	tests := []uint64{
		0, 1, 2, 10, 100000, 18446744073709551615,
	}
	for _, test := range tests {
		hash := uint64ToHash(test)
		value := hashToUint64(hash)
		if value != test {
			t.Errorf("expect %v, got %v", test, value)
		}
	}
}
