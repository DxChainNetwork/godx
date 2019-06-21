// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import "testing"


func TestSetConnFlag(t *testing.T) {
	var flag connFlag

	flag |= storageContractConn

	if (flag & storageContractConn) == 0 {
		t.Fatal("flag set error")
	}
	t.Log(int32(flag))
}
