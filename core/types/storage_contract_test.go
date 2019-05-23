package types

import (
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
)

func TestResolveStorageContractTransaction(t *testing.T) {
	to := common.HexToAddress("0x1")
	amount := new(big.Int).SetInt64(10000)
	gasPrice := new(big.Int).SetInt64(1e6)

	// test resolving host announce tx
	ha := HostAnnouncement{
		NetAddress: "127.0.0.1:8080",
	}
	scSetOrigin := StorageContractSet{
		HostAnnounce: ha,
	}
	ha_payload, err := rlp.EncodeToBytes(&scSetOrigin)
	if err != nil {
		t.Errorf("failed to rlp encode host announce tx: %v", err)
	}
	tx := NewTransaction(1, to, amount, 1000000, gasPrice, ha_payload)
	scSet, _ := ResolveStorageContractSet(tx)
	if scSet == nil {
		t.Errorf("failed to resolve host announcement: return nil")
	} else if scSet.HostAnnounce.NetAddress != ha.NetAddress {
		t.Errorf("something wrong for resolving host announcement")
	}

	// test resolving form contract tx
	sc := StorageContract{
		FileSize: 64,
	}
	scSetOrigin = StorageContractSet{
		StorageContract: sc,
	}
	sc_payload, err := rlp.EncodeToBytes(scSetOrigin)
	if err != nil {
		t.Errorf("failed to rlp encode form contract tx: %v", err)
	}
	tx = NewTransaction(1, to, amount, 1000000, gasPrice, sc_payload)
	scSet1, _ := ResolveStorageContractSet(tx)
	if scSet1 == nil {
		t.Errorf("failed to resolve form contract: return nil")
	} else if scSet1.StorageContract.FileSize != sc.FileSize {
		t.Errorf("something wrong for resolving form contract")
	}
}
