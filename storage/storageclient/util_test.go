// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/crypto/merkle"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
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

func TestNewVision(t *testing.T) {
	s := "{\"parentid\":\"0xd317a81cddcc28a2f3af3707ebb52a24c9649cd10ee9ab2cf07c310f843848a2\",\"unlockconditions\":{\"paymentaddress\":[\"0xb639db6974c87ff799820089761d7bee72d23e1b\",\"0x5f144608ca454a66dd3d7f11089a5ede0721e583\"],\"signaturesrequired\":2},\"newrevisionnumber\":11,\"newfilesize\":41943040,\"newfilemerkleroot\":\"0x2d1cf22f8cd400d267dd2a4868e341609780a9e180c2fd179259fecab71ddd89\",\"newwindowstart\":11530,\"newwindowend\":11770,\"newvalidproofpayback\":[{\"Address\":\"0xb639db6974c87ff799820089761d7bee72d23e1b\",\"Value\":114831385110186666},{\"Address\":\"0x5f144608ca454a66dd3d7f11089a5ede0721e583\",\"Value\":167091225066666000}],\"newmissedproofpayback\":[{\"Address\":\"0xb639db6974c87ff799820089761d7bee72d23e1b\",\"Value\":114831385110186666},{\"Address\":\"0x5f144608ca454a66dd3d7f11089a5ede0721e583\",\"Value\":167091225066666000}],\"newunlockhash\":\"0xa6223cc6f3f529af50c4d5c4ffe376c1ed0b06551c7163cad8f610b9dd41d968\",\"Signatures\":[\"MRGxX5hqr1XUX3wF+4hj7gbZX/Pc7EKHIUhgG+Dx9ycWZp2KTIkFVHMdzbNktQBkiPwEY66/z3tEU0GAjDjTOQA=\",\"urV2psnHQ/rb8FHHiAntU/SGvVu6AMo59AptOPa4QdtlmguHwA0jCtnqYpfbVPXZSejkbSClBA+QPQl+jSFl2gE=\"]}"
	var currentRevision types.StorageContractRevision
	if err := json.Unmarshal([]byte(s), &currentRevision); err != nil {
		t.Fatal(err)
	}

	// calculate price
	price := common.NewBigInt(12345678)
	if currentRevision.NewValidProofOutputs[0].Value.Cmp(price.BigIntPtr()) < 0 {
		t.Fatal("client funds not enough to support download")
	}

	// increase the price fluctuation by 0.2% to mitigate small errors, like different block height
	price = price.MultFloat64(1 + extraRatio)

	newRevision := NewRevision(currentRevision, price.BigIntPtr())

	a := common.NewBigInt(newRevision.NewValidProofOutputs[0].Value.Int64()).Add(common.NewBigInt(newRevision.NewValidProofOutputs[1].Value.Int64()))
	b := common.NewBigInt(currentRevision.NewValidProofOutputs[0].Value.Int64()).Add(common.NewBigInt(currentRevision.NewValidProofOutputs[1].Value.Int64()))

	if a.Cmp(b) != 0 {
		t.Fatal("balance is not equal")
	}

}
