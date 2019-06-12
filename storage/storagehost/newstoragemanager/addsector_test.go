// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"testing"
)

func TestMerkleRoot(t *testing.T) {
	data1 := []byte{1}
	data2 := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	fmt.Printf("%x\n", merkle.Root(data1))
	fmt.Printf("%x\n", merkle.Root(data2))
}
