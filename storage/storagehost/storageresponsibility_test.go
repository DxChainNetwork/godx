package storagehost

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
)

func TestStoreStorageResponsibility(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	so := StorageResponsibility{
		OriginStorageContract: types.StorageContract{
			WindowStart:    1,
			RevisionNumber: 1,
			WindowEnd:      144,
		},
	}

	err := putStorageResponsibility(db, so.OriginStorageContract.RLPHash(), so)
	if err != nil {
		t.Error(err)
	}

	sos, err := getStorageResponsibility(db, so.OriginStorageContract.RLPHash())
	if err != nil {
		t.Error(err)
	}

	if sos.OriginStorageContract.WindowEnd != 144 || sos.OriginStorageContract.WindowStart != 1 || sos.OriginStorageContract.RevisionNumber != 1 {
		t.Error("DB persistence error")
	}

	err = deleteStorageResponsibility(db, so.OriginStorageContract.RLPHash())
	if err != nil {
		t.Error(err)
	}
}

func TestStoreHeight(t *testing.T) {
	db := ethdb.NewMemDatabase()
	var height uint64
	height = 10
	defer db.Close()
	sc1 := types.StorageContract{
		WindowStart:    1,
		RevisionNumber: 1,
		WindowEnd:      144,
	}
	sc2 := types.StorageContract{
		WindowStart:    2,
		RevisionNumber: 1,
		WindowEnd:      144,
	}
	sc3 := types.StorageContract{
		WindowStart:    3,
		RevisionNumber: 1,
		WindowEnd:      144,
	}
	err := storeHeight(db, sc1.RLPHash(), height)
	if err != nil {
		t.Error(err)
	}
	data, err := getHeight(db, height)
	if err != nil {
		t.Error(err)
	} else {
		if len(data) != 32 {
			t.Log(data)
			t.Log(len(data))
			t.Error("DB persistence error")
		}
		if common.BytesToHash(data) != sc1.RLPHash() {
			t.Error("DB persistence error")
		}
	}

	err = storeHeight(db, sc2.RLPHash(), height)
	if err != nil {
		t.Error(err)
	}

	data, err = getHeight(db, height)
	if err != nil {
		t.Error(err)
	} else {
		if len(data) != 64 {
			t.Log(data)
			t.Log(len(data))
			t.Error("DB persistence error")
		}
		if common.BytesToHash(data[:32]) != sc1.RLPHash() || common.BytesToHash(data[32:]) != sc2.RLPHash() {
			t.Error("DB persistence error")
		}
	}

	err = storeHeight(db, sc3.RLPHash(), height)
	if err != nil {
		t.Error(err)
	}

	data, err = getHeight(db, height)
	if err != nil {
		t.Error(err)
	} else {
		if len(data) != 96 {
			t.Log(data)
			t.Log(len(data))
			t.Error("DB persistence error")
		}
		if common.BytesToHash(data[:32]) != sc1.RLPHash() || common.BytesToHash(data[32:64]) != sc2.RLPHash() || common.BytesToHash(data[64:]) != sc3.RLPHash() {
			t.Error("DB persistence error")
		}
	}
}

func TestStorageHost_CreateContractConfirmed(t *testing.T) {
	sc := &types.StorageContract{
		FileSize:       0,
		FileMerkleRoot: common.Hash{},
		WindowStart:    144,
		WindowEnd:      288,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{
			Value:   new(big.Int).SetInt64(116666666666666670),
			Address: common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b"),
		}},
		HostCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{
			Value:   new(big.Int).SetInt64(166666666666666000),
			Address: common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d"),
		}},
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: new(big.Int).SetInt64(116666666666666670), Address: common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b")},
			// Deposit is returned to host
			{Value: new(big.Int).SetInt64(166666666666666000), Address: common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d")},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: new(big.Int).SetInt64(116666666666666670), Address: common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b")},
			// Deposit is returned to host
			{Value: new(big.Int).SetInt64(166666666666666000), Address: common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d")},
		},
		UnlockHash:     common.HexToHash("0x71f9c62325481eb604ac3b7402033f460829c3b3654a646b16f96064859a641d"),
		RevisionNumber: 0,
		Signatures: [][]byte{
			[]byte("ExX8JhDlk4LTEMRbeKOylwrY5jjE26u3r2Ug7Jczx9Y5lxiT9wYOvALXHtcruvpHwVRsABDgjXuOk1DpPls1UgA="),
			[]byte("HZtc8/tyErh/PBqKRm/ByuFG870vig94Ag7JYTCZKalNsVM3x3YHUKBTOMqCamYitUN6dUB+b5LWeEHMjyoHfQA="),
		},
	}
	sr := types.StorageContractRevision{
		ParentID: sc.RLPHash(),
		UnlockConditions: types.UnlockConditions{
			PaymentAddresses: []common.Address{
				common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b"),
				common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d"),
			},
			SignaturesRequired: 2,
		},
		NewRevisionNumber:     1,
		NewFileSize:           sc.FileSize,
		NewFileMerkleRoot:     sc.FileMerkleRoot,
		NewWindowStart:        sc.WindowStart,
		NewWindowEnd:          sc.WindowEnd,
		NewValidProofOutputs:  sc.ValidProofOutputs,
		NewMissedProofOutputs: sc.MissedProofOutputs,
		NewUnlockHash:         sc.UnlockHash,
		Signatures: [][]byte{
			[]byte("KwLpK6iIWtysvSKNt0LI1jaPPaAVuFYsAVInxzFPfnMZX/eVEFzBhuGLAfoZvEWPcjJsp0KT4OCLPthWC10upwE="),
			[]byte("0LmlCN+x1nAnD6raOdcj9kCRikfydQW690lo6qHpufFa0k5HwK/Mb8kN5WrXYzCYXn6CvX2a5OO0eOwB053wjwE="),
		},
	}
	so := StorageResponsibility{
		OriginStorageContract:   *sc,
		NegotiationBlockNumber:  15,
		CreateContractConfirmed: false,
		ResponsibilityStatus:    0,
	}
	so.StorageContractRevisions = append(so.StorageContractRevisions, sr)

	h := new(StorageHost)
	db, err := openDB("./db")
	defer db.Close()
	if err != nil {
		t.Error("openDB:", err)
		return
	}

	h.db = db
	h.blockHeight = 10
	h.lockedStorageResponsibility = make(map[common.Hash]*TryMutex)

	err = putStorageResponsibility(h.db, sc.RLPHash(), so)
	if err != nil {
		t.Error("putStorageResponsibility:", err)
	}

	h.handleTaskItem(sc.RLPHash())

	data, err := getHeight(h.db, h.blockHeight+3)
	if err != nil {
		t.Error("getHeight", err)
	}
	taskItems := byteToHash(data)
	if len(taskItems) != 1 {
		t.Error("queueTaskItem length fail")
	}
	if !bytes.Equal(taskItems[0].Bytes(), so.id().Bytes()) {
		t.Error("queueTaskItem fail")
	}
}

//使用之前请注意，这个函数是没办法发送交易的，会直接Panic。
//需要注释掉storage/storagehost/storageresponsibility.go:508
//这一部分发送交易的代码
func TestStorageHost_StorageRevisionConfirmed(t *testing.T) {
	sc := &types.StorageContract{
		FileSize:       0,
		FileMerkleRoot: common.Hash{},
		WindowStart:    11526,
		WindowEnd:      11766,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{
			Value:   new(big.Int).SetInt64(116666666666666670),
			Address: common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b"),
		}},
		HostCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{
			Value:   new(big.Int).SetInt64(166666666666666000),
			Address: common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d"),
		}},
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: new(big.Int).SetInt64(116666666666666670), Address: common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b")},
			// Deposit is returned to host
			{Value: new(big.Int).SetInt64(166666666666666000), Address: common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d")},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: new(big.Int).SetInt64(116666666666666670), Address: common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b")},
			// Deposit is returned to host
			{Value: new(big.Int).SetInt64(166666666666666000), Address: common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d")},
		},
		UnlockHash:     common.HexToHash("0x71f9c62325481eb604ac3b7402033f460829c3b3654a646b16f96064859a641d"),
		RevisionNumber: 0,
		Signatures: [][]byte{
			[]byte("ExX8JhDlk4LTEMRbeKOylwrY5jjE26u3r2Ug7Jczx9Y5lxiT9wYOvALXHtcruvpHwVRsABDgjXuOk1DpPls1UgA="),
			[]byte("HZtc8/tyErh/PBqKRm/ByuFG870vig94Ag7JYTCZKalNsVM3x3YHUKBTOMqCamYitUN6dUB+b5LWeEHMjyoHfQA="),
		},
	}
	sr := types.StorageContractRevision{
		ParentID: sc.RLPHash(),
		UnlockConditions: types.UnlockConditions{
			PaymentAddresses: []common.Address{
				common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b"),
				common.HexToAddress("0xd1e2e5efcaa42d8daaf985a4dba39e4dddd5c30d"),
			},
			SignaturesRequired: 2,
		},
		NewRevisionNumber:     1,
		NewFileSize:           sc.FileSize,
		NewFileMerkleRoot:     sc.FileMerkleRoot,
		NewWindowStart:        sc.WindowStart,
		NewWindowEnd:          sc.WindowEnd,
		NewValidProofOutputs:  sc.ValidProofOutputs,
		NewMissedProofOutputs: sc.MissedProofOutputs,
		NewUnlockHash:         sc.UnlockHash,
		Signatures: [][]byte{
			[]byte("KwLpK6iIWtysvSKNt0LI1jaPPaAVuFYsAVInxzFPfnMZX/eVEFzBhuGLAfoZvEWPcjJsp0KT4OCLPthWC10upwE="),
			[]byte("0LmlCN+x1nAnD6raOdcj9kCRikfydQW690lo6qHpufFa0k5HwK/Mb8kN5WrXYzCYXn6CvX2a5OO0eOwB053wjwE="),
		},
	}
	so := StorageResponsibility{
		OriginStorageContract:    *sc,
		NegotiationBlockNumber:   15,
		CreateContractConfirmed:  true,
		StorageRevisionConfirmed: false,
		ResponsibilityStatus:     0,
	}
	so.StorageContractRevisions = append(so.StorageContractRevisions, sr)

	h := new(StorageHost)
	db, err := openDB("./db")
	defer db.Close()
	if err != nil {
		t.Error("openDB:", err)
		return
	}

	h.db = db
	h.blockHeight = 10000
	h.log = log.New()
	h.lockedStorageResponsibility = make(map[common.Hash]*TryMutex)

	err = putStorageResponsibility(h.db, sc.RLPHash(), so)
	if err != nil {
		t.Error("putStorageResponsibility:", err)
	}
	h.handleTaskItem(sc.RLPHash())

	data, err := getHeight(h.db, h.blockHeight+3)
	if err != nil {
		t.Error("getHeight", err)
	}
	taskItems := byteToHash(data)
	if len(taskItems) != 1 {
		t.Error("queueTaskItem length fail")
	}
	if !bytes.Equal(taskItems[0].Bytes(), so.id().Bytes()) {
		t.Error("queueTaskItem fail")
	}
}

func byteToHash(data []byte) []common.Hash {
	var taskItems []common.Hash
	knownActionItems := make(map[common.Hash]struct{})
	responsibilityIDs := make([]common.Hash, len(data)/common.HashLength)
	for i := 0; i < len(data); i += common.HashLength {
		copy(responsibilityIDs[i/common.HashLength][:], data[i:i+common.HashLength])
	}
	for _, soid := range responsibilityIDs {
		_, exists := knownActionItems[soid]
		if !exists {
			taskItems = append(taskItems, soid)
			knownActionItems[soid] = struct{}{}
		}
	}
	return taskItems
}
