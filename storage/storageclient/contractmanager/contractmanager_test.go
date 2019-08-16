// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"testing"
)

func TestContractManager_Start(t *testing.T) {
	if _, err := NewFakeContractManager(); err != nil {
		t.Fatalf("contract manater start test failed: %s", err.Error())
	}
}
