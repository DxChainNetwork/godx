// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/crypto/merkle"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

var (
	actions = []storage.UploadAction{
		{
			Type: storage.UploadActionAppend,
			Data: []byte("dxchain"),
		},

		{
			Type: "haha",
			Data: []byte("dxchain"),
		},
	}

	leafRanges = []merkle.SubTreeLimit{
		{
			Left:  123,
			Right: 234,
		},
		{
			Left:  456,
			Right: 789,
		},
	}

	leafHashes = []common.Hash{
		common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000223233"),
		common.HexToHash("0x00000000000000000000000000000000000000000000000000000000004524ac"),
	}
)

func TestCalculateProofRanges(t *testing.T) {
	calculatedRanges := CalculateProofRanges(actions, 5)
	if !reflect.DeepEqual(calculatedRanges, []merkle.SubTreeLimit{}) {
		t.Errorf("wanted %v, getted %v", []merkle.SubTreeLimit{}, calculatedRanges)
	}
}

func TestModifyLeaves(t *testing.T) {
	modifiedLeafs := ModifyLeaves(leafHashes, actions, 5)
	if modifiedLeafs == nil {
		t.Error("get nil leaf hashes")
	}

	if reflect.DeepEqual(modifiedLeafs, []common.Hash{}) {
		t.Errorf("getted %v", []common.Hash{})
	}

	if len(modifiedLeafs) != 3 {
		t.Errorf("wanted length: %v, getted length %v", 3, len(modifiedLeafs))
	}
}

func TestModifyProofRanges(t *testing.T) {
	modifiedRanges := ModifyProofRanges(leafRanges, actions, 5)
	if modifiedRanges == nil {
		t.Error("get nil leaf range")
	}

	if reflect.DeepEqual(modifiedRanges, []merkle.SubTreeLimit{}) {
		t.Errorf("getted %v", []merkle.SubTreeLimit{})
	}

	if len(modifiedRanges) != 3 {
		t.Errorf("wanted length: %v, getted length %v", 3, len(modifiedRanges))
	}
}
