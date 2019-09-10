// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/rawdb"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage/coinchargemaintenance"
)

var (
	gasOrigin        = uint64(1000000)
	balanceOrigin    = big.NewInt(1000000000000000000)
	hostCollateral   = new(big.Int).SetInt64(10000000000)
	clientCollateral = new(big.Int).SetInt64(20000000000)
	cost             = big.NewInt(1000000)
)

// AccountInfo is an account in the state
type AccountInfo struct {
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance *big.Int                    `json:"balance" gencodec:"required"`
	Nonce   uint64                      `json:"nonce,omitempty"`
}

type AccountAlloc map[common.Address]AccountInfo

type PrivkeyAddress struct {
	Privkey *ecdsa.PrivateKey
	Address common.Address
}

func TestEVM_CandidateTx(t *testing.T) {
	// mock evm
	evm, _, pas, err := mockEvmAndState(100)
	if err != nil {
		t.Fatal("mockEvmAndState err:", err)
	}

	//mock dposContext
	db := ethdb.NewMemDatabase()
	dposContext, err := types.NewDposContext(db)
	if err != nil {
		t.Fatal("NewDposContext err:", err)
	}

	tests := []struct {
		from            common.Address
		wantErr         error
		wantGas         uint64
		wantDeposit     *big.Int
		wantRewardRatio common.Hash
		wantCandidate   common.Address
		data            []byte
		gas             uint64
		value           *big.Int
	}{
		{
			from:            pas[0].Address,
			wantErr:         nil,
			wantGas:         40000,
			wantDeposit:     new(big.Int).SetUint64(10000),
			wantRewardRatio: common.BytesToHash([]byte("0x50")),
			wantCandidate:   pas[0].Address,
			data:            []byte("0x50"),
			gas:             100000,
			value:           new(big.Int).SetUint64(10000),
		},
		{
			from:            pas[0].Address,
			wantErr:         errDuplicateCandidateTx,
			wantGas:         100000,
			wantDeposit:     new(big.Int).SetUint64(10000),
			wantRewardRatio: common.BytesToHash([]byte("0x50")),
			wantCandidate:   pas[0].Address,
			data:            []byte("0x99"),
			gas:             100000,
			value:           new(big.Int).SetUint64(20000),
		},
	}
	for _, test := range tests {
		_, gas, err := evm.CandidateTx(test.from, test.data, test.gas, test.value, dposContext)
		if err != test.wantErr {
			t.Errorf("wanted error: %v, got: %v", test.wantErr, err)
		} else if gas != test.wantGas {
			t.Errorf("wanted gas: %d,got: %d", test.wantGas, gas)
		} else if rewardRatio := evm.StateDB.GetState(test.from, dpos.KeyRewardRatioNumerator); rewardRatio != test.wantRewardRatio {
			t.Errorf("wanted rewardRatio: %v,got: %v", test.wantRewardRatio, rewardRatio)
		} else if deposit := evm.StateDB.GetState(test.from, dpos.KeyCandidateDeposit); deposit != common.BigToHash(test.wantDeposit) {
			t.Errorf("wanted candidateDeposit: %v,got: %v", common.BigToHash(test.wantDeposit), deposit)
		}

		data, err := dposContext.CandidateTrie().TryGet(test.from.Bytes())
		if err != nil {
			t.Error("CandidateTrie err:", err)
		}
		if common.BytesToAddress(data) != test.wantCandidate {
			t.Error("CandidateTrie data", data)
		}
	}

}

func TestEVM_CandidateCancelTx(t *testing.T) {
	// mock evm
	evm, _, pas, err := mockEvmAndState(100)
	if err != nil {
		t.Fatal("mockEvmAndState err:", err)
	}
	evm.Context.Time = new(big.Int).SetUint64(86400)

	//mock dposContext
	db := ethdb.NewMemDatabase()
	dposContext, err := types.NewDposContext(db)
	if err != nil {
		t.Fatal("NewDposContext err:", err)
	}

	currentEpochID := dpos.CalculateEpochID(evm.Time.Int64())
	epochIDStr := strconv.FormatInt(currentEpochID, 10)
	thawingAddress := common.BytesToAddress([]byte(dpos.PrefixThawingAddr + epochIDStr))

	tests := []struct {
		from                 common.Address
		wantErr              error
		wantGas              uint64
		wantCandidateThawing common.Hash
		wantCandidate        common.Address
		gas                  uint64
		thawingAddress       common.Address
		keyCandidateThawing  string
	}{
		{
			from:                 pas[0].Address,
			wantErr:              nil,
			wantGas:              60000,
			wantCandidateThawing: common.BytesToHash(pas[0].Address.Bytes()),
			wantCandidate:        common.Address{},
			gas:                  100000,
			thawingAddress:       thawingAddress,
			keyCandidateThawing:  dpos.PrefixCandidateThawing + pas[0].Address.String(),
		},
	}

	for _, test := range tests {
		_, gas, err := evm.CandidateCancelTx(test.from, test.gas, dposContext)
		if err != test.wantErr {
			t.Errorf("wanted error: %v, got: %v", test.wantErr, err)
		} else if gas != test.wantGas {
			t.Errorf("wanted gas: %d,got: %d", test.wantGas, gas)
		}

		candidateThawing := evm.StateDB.GetState(test.thawingAddress, common.BytesToHash([]byte(test.keyCandidateThawing)))
		if candidateThawing != (common.Hash{}) {
			t.Errorf("wanted candidateThawing: %v,got: %v", (common.Hash{}), candidateThawing)
		}

		evm.StateDB.SetState(test.thawingAddress, common.BytesToHash([]byte(test.keyCandidateThawing)), common.BytesToHash(pas[0].Address.Bytes()))
		candidateThawing = evm.StateDB.GetState(test.thawingAddress, common.BytesToHash([]byte(test.keyCandidateThawing)))
		if candidateThawing != test.wantCandidateThawing {
			t.Errorf("wanted candidateThawing: %v,got: %v", test.wantCandidateThawing, candidateThawing)
		}

		data, err := dposContext.CandidateTrie().TryGet(test.from.Bytes())
		if err != nil {
			t.Error("CandidateTrie err:", err)
		}
		if common.BytesToAddress(data) != test.wantCandidate {
			t.Error("CandidateTrie data", data)
		}
	}
}

func TestEVM_VoteTx(t *testing.T) {
	// mock evm
	evm, _, addresses, err := mockEvmAndState(100)
	if err != nil {
		t.Fatalf("failed to create evm and state,err: %v", err)
	}

	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	dposContext, err := types.NewDposContext(db)
	if err != nil {
		t.Fatalf("failed to create dpos context,err: %v", err)
	}

	from := addresses[0].Address
	candidateList := []common.Address{
		common.HexToAddress("0x31de5dbe50885d9632935dec507f806baf1027c0"),
		common.HexToAddress("0xcde55147efd18f79774676d5a8674d94d00b4c9a"),
		common.HexToAddress("0x6de5af2854ad0f9d5b7d0b749fffa3f7a57d7b9d"),
	}

	for _, addr := range candidateList {
		err = dposContext.CandidateTrie().TryUpdate(addr.Bytes(), addr.Bytes())
		if err != nil {
			t.Fatalf("failed to insert candidate,error: %v", err)
		}
	}

	data, err := rlp.EncodeToBytes(candidateList)
	if err != nil {
		t.Fatalf("failed to rlp encode candidate list,err: %v", err)
	}

	_, gasRemain, err := evm.VoteTx(from, dposContext, data, 1000000, new(big.Int).SetInt64(1e18))
	if err != nil {
		t.Errorf("failed to execute vote tx,error: %v", err)
	}

	if gasRemain != 1000000-1000-20000*4 {
		t.Errorf("wanted gas remain: %d,got: %d", (1000000 - 1000 - 20000*4), gasRemain)
	}
}

func TestEVM_CancelVoteTx(t *testing.T) {
	// mock evm
	evm, _, addresses, err := mockEvmAndState(100)
	if err != nil {
		t.Fatalf("failed to create evm and state,err: %v", err)
	}

	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	dposContext, err := types.NewDposContext(db)
	if err != nil {
		t.Fatalf("failed to create dpos context,err: %v", err)
	}

	from := addresses[0].Address
	candidateList := []common.Address{
		common.HexToAddress("0x31de5dbe50885d9632935dec507f806baf1027c0"),
		common.HexToAddress("0xcde55147efd18f79774676d5a8674d94d00b4c9a"),
		common.HexToAddress("0x6de5af2854ad0f9d5b7d0b749fffa3f7a57d7b9d"),
	}

	for _, addr := range candidateList {
		err = dposContext.DelegateTrie().TryUpdate(append(addr.Bytes(), from.Bytes()...), from.Bytes())
		if err != nil {
			t.Fatalf("failed to insert delegate,error: %v", err)
		}
	}

	data, err := rlp.EncodeToBytes(candidateList)
	if err != nil {
		t.Fatalf("failed to rlp encode candidate list,err: %v", err)
	}

	err = dposContext.VoteTrie().TryUpdate(from.Bytes(), data)
	if err != nil {
		t.Fatalf("failed to insert delegate,error: %v", err)
	}

	evm.Time = new(big.Int).SetInt64(time.Now().Unix())
	_, gasRemain, err := evm.CancelVoteTx(from, dposContext, 1000000)
	if err != nil {
		t.Errorf("failed to execute vote tx,error: %v", err)
	}

	if gasRemain != 1000000-20000*2 {
		t.Errorf("wanted gas remain: %d,got: %d", (1000000 - 20000*2), gasRemain)
	}
}

func TestEVM_HostAnnounceTx(t *testing.T) {

	// generate pub\priv key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate private key,error: %v", err)
	}

	// mock host node
	hostNode := enode.NewV4(&privateKey.PublicKey, net.IP{127, 0, 0, 1}, int(8888), int(8888))

	// mock a new host announce data
	mockHostAnnounce := types.HostAnnouncement{
		NetAddress: hostNode.String(),
	}
	sign, err := crypto.Sign(mockHostAnnounce.RLPHash().Bytes(), privateKey)
	if err != nil {
		t.Errorf("failed to sign host announce,error: %v", err)
	}
	mockHostAnnounce.Signature = sign

	// mock evm
	hostAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	accounts := mockAccountAlloc([]common.Address{hostAddress})
	stateDB := mockState(ethdb.NewMemDatabase(), accounts)
	evm := NewEVM(Context{}, stateDB, params.MainnetChainConfig, Config{})

	rlpBytes, err := rlp.EncodeToBytes(mockHostAnnounce)
	if err != nil {
		t.Errorf("failed to rlp host announce,error: %v", err)
	}

	_, gasLeft, err := evm.HostAnnounceTx(AccountRef{}, rlpBytes, gasOrigin)
	if err != nil {
		t.Errorf("failed to execute host announce tx,error: %v", err)
	}

	// check whether gas left is right
	if gasLeft != gasOrigin-params.DecodeGas-params.CheckMultiSignaturesGas {
		t.Errorf("gas left is not right after executing host announce tx,wanted %d,getted %d", gasOrigin-params.DecodeGas-params.CheckMultiSignaturesGas, gasLeft)
	}
}

func TestEVM_CreateContractTx(t *testing.T) {

	// mock evm, state, client and host address ...
	evm, stateDB, prvAndAddresses, err := mockEvmAndState(1000)
	if err != nil {
		t.Error(err)
	}

	clientAddress := prvAndAddresses[0].Address
	hostAddress := prvAndAddresses[1].Address

	// mock a new storage contract data
	sc, err := mockStorageContract(prvAndAddresses)
	if err != nil {
		t.Error(err)
	}

	rlpBytes, err := rlp.EncodeToBytes(sc)
	if err != nil {
		t.Errorf("failed to rlp storage contract,error: %v", err)
	}

	_, gasLeft, err := evm.CreateContractTx(AccountRef{}, rlpBytes, gasOrigin)
	if err != nil {
		t.Errorf("failed to execute storage contract tx,error: %v", err)
	}

	// check whether gas left is right
	if gasLeft != gasOrigin-params.DecodeGas-params.CheckFileGas {
		t.Errorf("gas left is not right after executing storage contract tx,wanted %d,getted %d", gasOrigin-params.DecodeGas-params.CheckFileGas, gasLeft)
	}

	// check storage contract data whether is written into state
	scID := sc.ID()
	contractAddr := common.BytesToAddress(scID[12:])
	if !stateDB.Exist(contractAddr) {
		t.Error("this storage contract account not register into state")
	}

	windowEndStr := strconv.FormatUint(sc.WindowEnd, 10)
	statusAddr := common.BytesToAddress([]byte(coinchargemaintenance.StrPrefixExpSC + windowEndStr))
	if !stateDB.Exist(statusAddr) {
		t.Error("this status account not register into state")
	}

	statusFlag := stateDB.GetState(statusAddr, scID)
	if !bytes.Equal(statusFlag.Bytes()[11:12], coinchargemaintenance.NotProofedStatus) {
		t.Errorf("write wrong contract status into state,wanted %v,getted %v", coinchargemaintenance.NotProofedStatus, statusFlag.Bytes())
	}

	clientCollateral := sc.ClientCollateral.Value
	hostCollateral := sc.HostCollateral.Value

	clientCollateralHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientCollateral)
	clientCollateralValue := new(big.Int).SetBytes(clientCollateralHash.Bytes()).Uint64()
	if clientCollateralValue != clientCollateral.Uint64() {
		t.Errorf("write wrong client collateral data into state,wanted %v,getted %v", clientCollateral.Uint64(), clientCollateralValue)
	}

	hostCollateralHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostCollateral)
	hostCollateralValue := new(big.Int).SetBytes(hostCollateralHash.Bytes()).Uint64()
	if hostCollateralValue != hostCollateral.Uint64() {
		t.Errorf("write wrong host collateral data into state,wanted %v,getted %v", hostCollateral.Uint64(), hostCollateralValue)
	}

	fileSizeHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyFileSize)
	fileSize := new(big.Int).SetBytes(fileSizeHash.Bytes()).Uint64()
	if fileSize != 0 {
		t.Errorf("write wrong file size data into state,wanted %v,getted %v", 0, fileSize)
	}

	unlockHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyUnlockHash)
	if unlockHash != sc.UnlockHash {
		t.Errorf("write wrong unlock hash data into state,wanted %v,getted %v", sc.UnlockHash, unlockHash)
	}

	fileMerkleRoot := stateDB.GetState(contractAddr, coinchargemaintenance.KeyFileMerkleRoot)
	if fileMerkleRoot != sc.FileMerkleRoot {
		t.Errorf("write wrong file merkle root data into state,wanted %v,getted %v", sc.FileMerkleRoot, fileMerkleRoot)
	}

	revisionNumHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyRevisionNumber)
	revisionNum := new(big.Int).SetBytes(revisionNumHash.Bytes()).Uint64()
	if revisionNum != sc.RevisionNumber {
		t.Errorf("write wrong revision num data into state,wanted %v,getted %v", sc.RevisionNumber, revisionNum)
	}

	windowStartHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyWindowStart)
	windowStart := new(big.Int).SetBytes(windowStartHash.Bytes()).Uint64()
	if windowStart != sc.WindowStart {
		t.Errorf("write wrong window start data into state,wanted %v,getted %v", sc.WindowStart, windowStart)
	}

	windowEndHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyWindowEnd)
	windowEnd := new(big.Int).SetBytes(windowEndHash.Bytes()).Uint64()
	if windowEnd != sc.WindowEnd {
		t.Errorf("write wrong window end data into state,wanted %v,getted %v", sc.WindowEnd, windowEnd)
	}

	clientVpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientValidProofOutput)
	clientVpo := new(big.Int).SetBytes(clientVpoHash.Bytes()).Uint64()
	if clientVpo != sc.ValidProofOutputs[0].Value.Uint64() {
		t.Errorf("write wrong client valid proof outputs data into state,wanted %v,getted %v", sc.ValidProofOutputs[0].Value.Uint64(), clientVpo)
	}

	hostVpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostValidProofOutput)
	hostVpo := new(big.Int).SetBytes(hostVpoHash.Bytes()).Uint64()
	if hostVpo != sc.ValidProofOutputs[1].Value.Uint64() {
		t.Errorf("write wrong host valid proof outputs data into state,wanted %v,getted %v", sc.ValidProofOutputs[1].Value.Uint64(), hostVpo)
	}

	clientMpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientMissedProofOutput)
	clientMpo := new(big.Int).SetBytes(clientMpoHash.Bytes()).Uint64()
	if clientMpo != sc.MissedProofOutputs[0].Value.Uint64() {
		t.Errorf("write wrong client missed proof outputs data into state,wanted %v,getted %v", sc.MissedProofOutputs[0].Value.Uint64(), clientMpo)
	}

	hostMpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostMissedProofOutput)
	hostMpo := new(big.Int).SetBytes(hostMpoHash.Bytes()).Uint64()
	if hostMpo != sc.MissedProofOutputs[1].Value.Uint64() {
		t.Errorf("write wrong host missed proof outputs data into state,wanted %v,getted %v", sc.MissedProofOutputs[1].Value.Uint64(), hostMpo)
	}

	clientAddressHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientAddress)
	ca := common.BytesToAddress(clientAddressHash.Bytes())
	if ca != clientAddress {
		t.Errorf("write wrong client address data into state,wanted %v,getted %v", clientAddress, ca)
	}

	hostAddressHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostAddress)
	ha := common.BytesToAddress(hostAddressHash.Bytes())
	if ha != hostAddress {
		t.Errorf("write wrong host address data into state,wanted %v,getted %v", hostAddress, ha)
	}

	// check client and host balance
	clientBalance := stateDB.GetBalance(clientAddress)
	if clientBalance.Int64() != balanceOrigin.Int64()-clientCollateral.Int64() {
		t.Errorf("client balance is not right after executing storage contract tx,wanted %d,getted %d", balanceOrigin.Int64()-clientCollateral.Int64(), clientBalance.Int64())
	}

	hostBalance := stateDB.GetBalance(hostAddress)
	if hostBalance.Int64() != balanceOrigin.Int64()-hostCollateral.Int64() {
		t.Errorf("host balance is not right after executing storage contract tx,wanted %d,getted %d", balanceOrigin.Int64()-hostCollateral.Int64(), hostBalance.Int64())
	}
}

func TestEVM_CommitRevisionTx(t *testing.T) {

	// mock evm, state, client and host address ...
	evm, stateDB, prvAndAddresses, err := mockEvmAndState(1000)
	if err != nil {
		t.Error(err)
	}

	prvKeyClient := prvAndAddresses[0].Privkey
	prvKeyHost := prvAndAddresses[1].Privkey

	// 1) write storage contract into state
	// mock a new storage contract data
	sc, err := mockStorageContract(prvAndAddresses)
	if err != nil {
		t.Error(err)
	}

	// mock writing storage contract data into state
	mockWriteStorageContractIntoState(*sc, stateDB)

	// 2) modify storage contract by commit revision tx
	// mock storage revision data
	scr, err := mockStorageRevision(*sc, cost, prvKeyClient, prvKeyHost)
	if err != nil {
		t.Error(err)
	}

	rlpBytes2, err := rlp.EncodeToBytes(scr)
	if err != nil {
		t.Error(err)
	}

	_, gasLeft, err := evm.CommitRevisionTx(AccountRef{}, rlpBytes2, gasOrigin)
	if err != nil {
		t.Errorf("failed to execute commit revision tx,error: %v", err)
	}

	// check left gas is right after executing commit revision tx
	if gasLeft != gasOrigin-params.DecodeGas-params.CheckFileGas {
		t.Errorf("gas left is not right after executing commit revision tx,wanted %d,getted %d", gasOrigin-params.DecodeGas-params.CheckFileGas, gasLeft)
	}

	// check storage contract data whether is updated
	contractAddr := common.BytesToAddress(scr.ParentID[12:])
	if !stateDB.Exist(contractAddr) {
		t.Error("this storage contract account not register into state")
	}

	fileSizeHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyFileSize)
	fileSize := new(big.Int).SetBytes(fileSizeHash.Bytes()).Uint64()
	if fileSize != 1000 {
		t.Errorf("failed to update file size data into state,wanted %v,getted %v", 1000, fileSize)
	}

	fileMerkleRoot := stateDB.GetState(contractAddr, coinchargemaintenance.KeyFileMerkleRoot)
	if fileMerkleRoot != common.HexToHash("0x20198404b29fdc225c1ad7df48da3e16c08f8c9fb50c1768ce08baeba57b3bd7") {
		t.Errorf("failed to update file merkle root data into state,wanted %v,getted %v", common.HexToHash("0x20198404b29fdc225c1ad7df48da3e16c08f8c9fb50c1768ce08baeba57b3bd7"), fileMerkleRoot)
	}

	revisionNumHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyRevisionNumber)
	revisionNum := new(big.Int).SetBytes(revisionNumHash.Bytes()).Uint64()
	if revisionNum != 2 {
		t.Errorf("failed to update revision num data into state,wanted %v,getted %v", 2, revisionNum)
	}

	clientVpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientValidProofOutput)
	clientVpo := new(big.Int).SetBytes(clientVpoHash.Bytes()).Uint64()
	if clientVpo != new(big.Int).Sub(clientCollateral, cost).Uint64() {
		t.Errorf("failed to update client valid proof outputs data into state,wanted %v,getted %v", new(big.Int).Sub(clientCollateral, cost).Uint64(), clientVpo)
	}

	hostVpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostValidProofOutput)
	hostVpo := new(big.Int).SetBytes(hostVpoHash.Bytes()).Uint64()
	if hostVpo != new(big.Int).Add(hostCollateral, cost).Uint64() {
		t.Errorf("failed to update host valid proof outputs data into state,wanted %v,getted %v", new(big.Int).Add(hostCollateral, cost).Uint64(), hostVpo)
	}

	clientMpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientMissedProofOutput)
	clientMpo := new(big.Int).SetBytes(clientMpoHash.Bytes()).Uint64()
	if clientMpo != new(big.Int).Sub(clientCollateral, cost).Uint64() {
		t.Errorf("failed to update client missed proof outputs data into state,wanted %v,getted %v", new(big.Int).Sub(clientCollateral, cost).Uint64(), clientMpo)
	}

	hostMpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostMissedProofOutput)
	hostMpo := new(big.Int).SetBytes(hostMpoHash.Bytes()).Uint64()
	if hostMpo != hostCollateral.Uint64() {
		t.Errorf("failed to update host missed proof outputs data into state,wanted %v,getted %v", hostCollateral.Uint64(), hostMpo)
	}

}

func TestEVM_StorageProofTx(t *testing.T) {

	// mock evm, state, client and host address ...
	evm, stateDB, prvAndAddresses, err := mockEvmAndState(1101)
	if err != nil {
		t.Error(err)
	}

	// mock block hash at height 1101 into DB
	db := stateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	mockBlockHash := common.HexToHash("0x877c3a381d5ad88ca76a7b3e33ab1611939de59c56c0506efb9021593618f6ab")
	rawdb.WriteCanonicalHash(db, mockBlockHash, uint64(1000))

	// 1) write storage contract into state
	// mock a new storage contract data
	sc, err := mockStorageContract(prvAndAddresses)
	if err != nil {
		t.Error(err)
	}

	// mock writing storage contract data into state
	mockWriteStorageContractIntoState(*sc, stateDB)

	// mock storage proof
	sp, err := mockStorageProof(prvAndAddresses[1].Privkey, sc.ID())
	if err != nil {
		t.Error(err)
	}

	rlpBytes, err := rlp.EncodeToBytes(sp)
	if err != nil {
		t.Error(err)
	}

	_, gasLeft, err := evm.StorageProofTx(AccountRef{}, rlpBytes, gasOrigin)
	if err != nil {
		t.Errorf("failed to execute storage proof tx,error: %v", err)
	}

	// check left gas is right after executing commit revision tx
	if gasLeft != gasOrigin-params.DecodeGas-params.CheckFileGas {
		t.Errorf("gas left is not right after executing storage proof tx,wanted %d,getted %d", gasOrigin-params.DecodeGas-params.CheckFileGas, gasLeft)
	}

	// retrieve origin storage contract data
	contractAddr := common.BytesToAddress(sp.ParentID[12:])
	clientVpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyClientValidProofOutput)
	hostVpoHash := stateDB.GetState(contractAddr, coinchargemaintenance.KeyHostValidProofOutput)

	// check contract status
	windowEndStr := strconv.FormatUint(1101, 10)
	statusAddr := common.BytesToAddress([]byte(coinchargemaintenance.StrPrefixExpSC + windowEndStr))
	statusHash := stateDB.GetState(statusAddr, sp.ParentID)
	if !bytes.Equal(statusHash.Bytes()[11:12], coinchargemaintenance.ProofedStatus) {
		t.Errorf("failed to set contract proofed after executing storage proof tx")
	}

	// check client and host balance
	clientBalance := stateDB.GetBalance(prvAndAddresses[0].Address)
	clientVpo := new(big.Int).SetBytes(clientVpoHash.Bytes())
	if clientBalance.Int64() != balanceOrigin.Int64()+clientVpo.Int64() {
		t.Errorf("client balance is not right after executing storage proof tx,wanted %d,getted %d", balanceOrigin.Int64()+clientVpo.Int64(), clientBalance.Int64())
	}

	hostBalance := stateDB.GetBalance(prvAndAddresses[1].Address)
	hostVpo := new(big.Int).SetBytes(hostVpoHash.Bytes())
	if hostBalance.Int64() != balanceOrigin.Int64()+hostVpo.Int64() {
		t.Errorf("host balance is not right after executing storage proof tx,wanted %d,getted %d", balanceOrigin.Int64()+hostCollateral.Int64(), hostBalance.Int64())
	}

}

func mockAccountAlloc(addrs []common.Address) AccountAlloc {
	accounts := make(AccountAlloc)
	for _, addr := range addrs {
		accounts[addr] = AccountInfo{
			Nonce:   1,
			Balance: balanceOrigin,
			Storage: make(map[common.Hash]common.Hash),
		}
	}
	return accounts
}

func mockState(db ethdb.Database, accounts AccountAlloc) *state.StateDB {
	sdb := state.NewDatabase(db)
	stateDB, _ := state.New(common.Hash{}, sdb)
	for addr, a := range accounts {
		stateDB.SetNonce(addr, a.Nonce)
		stateDB.SetBalance(addr, a.Balance)
		for k, v := range a.Storage {
			stateDB.SetState(addr, k, v)
		}
	}
	root, _ := stateDB.Commit(false)
	stateDB, _ = state.New(root, sdb)
	return stateDB
}

func mockClientAndHostAddress() ([]PrivkeyAddress, error) {
	// mock host address
	prvKeyHost, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate public/private key pairs for storage host: %v", err)
	}

	// mock client address
	prvKeyClient, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate public/private key pairs for storage client: %v", err)
	}

	hostAddress := crypto.PubkeyToAddress(prvKeyHost.PublicKey)
	clientAddress := crypto.PubkeyToAddress(prvKeyClient.PublicKey)
	return []PrivkeyAddress{{prvKeyClient, clientAddress}, {prvKeyHost, hostAddress}}, nil
}

func mockEvmAndState(currentHeight uint64) (*EVM, *state.StateDB, []PrivkeyAddress, error) {
	prvAndAddresses, err := mockClientAndHostAddress()
	if err != nil {
		return nil, nil, nil, err
	}
	clientAddress := prvAndAddresses[0].Address
	hostAddress := prvAndAddresses[1].Address

	// mock evm
	accounts := mockAccountAlloc([]common.Address{clientAddress, hostAddress})
	stateDB := mockState(ethdb.NewMemDatabase(), accounts)
	ctx := Context{
		BlockNumber: new(big.Int).SetUint64(currentHeight),
	}
	evm := NewEVM(ctx, stateDB, params.MainnetChainConfig, Config{})
	return evm, stateDB, prvAndAddresses, err
}

func mockStorageContract(prvAndAddresses []PrivkeyAddress) (*types.StorageContract, error) {
	clientAddress := prvAndAddresses[0].Address
	prvKeyClient := prvAndAddresses[0].Privkey
	hostAddress := prvAndAddresses[1].Address
	prvKeyHost := prvAndAddresses[1].Privkey

	hostCharge := types.DxcoinCharge{
		Value:   hostCollateral,
		Address: hostAddress,
	}

	clientCharge := types.DxcoinCharge{
		Value:   clientCollateral,
		Address: clientAddress,
	}

	uc := types.UnlockConditions{
		PaymentAddresses:   []common.Address{clientAddress, hostAddress},
		SignaturesRequired: 2,
	}

	sc := &types.StorageContract{
		FileSize:         0,
		FileMerkleRoot:   common.Hash{},
		WindowStart:      uint64(1001),
		WindowEnd:        uint64(1101),
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: clientCharge},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: hostCharge},
		UnlockHash:       uc.UnlockHash(),
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			clientCharge,
			hostCharge,
		},
		MissedProofOutputs: []types.DxcoinCharge{
			clientCharge,
			hostCharge,
		},
	}

	signByClient, err := crypto.Sign(sc.RLPHash().Bytes(), prvKeyClient)
	if err != nil {
		return nil, fmt.Errorf("client failed to sign storage contract,error: %v", err)
	}

	signByHost, err := crypto.Sign(sc.RLPHash().Bytes(), prvKeyHost)
	if err != nil {
		return nil, fmt.Errorf("host failed to sign storage contract,error: %v", err)
	}

	sc.Signatures = [][]byte{signByClient, signByHost}
	return sc, nil
}

func mockStorageRevision(sc types.StorageContract, cost *big.Int, prvKeyClient, prvKeyHost *ecdsa.PrivateKey) (*types.StorageContractRevision, error) {
	scr := &types.StorageContractRevision{
		ParentID: sc.ID(),
		UnlockConditions: types.UnlockConditions{
			PaymentAddresses: []common.Address{
				sc.ClientCollateral.Address,
				sc.HostCollateral.Address,
			},
			SignaturesRequired: 2,
		},
		NewRevisionNumber: 2,
		NewFileSize:       1000,
		NewFileMerkleRoot: common.HexToHash("0x20198404b29fdc225c1ad7df48da3e16c08f8c9fb50c1768ce08baeba57b3bd7"),
		NewWindowStart:    sc.WindowStart,
		NewWindowEnd:      sc.WindowEnd,
		NewValidProofOutputs: []types.DxcoinCharge{
			{Address: sc.ValidProofOutputs[0].Address},
			{Address: sc.ValidProofOutputs[1].Address},
		},
		NewMissedProofOutputs: []types.DxcoinCharge{
			{Address: sc.MissedProofOutputs[0].Address},
			{Address: sc.MissedProofOutputs[1].Address},
		},
		NewUnlockHash: sc.UnlockHash,
	}

	// move valid payout from client to host
	scr.NewValidProofOutputs[0].Value = new(big.Int).Sub(sc.ValidProofOutputs[0].Value, cost)
	scr.NewValidProofOutputs[1].Value = new(big.Int).Add(sc.ValidProofOutputs[1].Value, cost)

	// move missed payout from client to void, mean that will burn missed payout of client
	scr.NewMissedProofOutputs[0].Value = new(big.Int).Sub(sc.MissedProofOutputs[0].Value, cost)
	scr.NewMissedProofOutputs[1].Value = sc.MissedProofOutputs[1].Value

	signByClient, err := crypto.Sign(scr.RLPHash().Bytes(), prvKeyClient)
	if err != nil {
		return nil, fmt.Errorf("client failed to sign storage contract,error: %v", err)
	}

	signByHost, err := crypto.Sign(scr.RLPHash().Bytes(), prvKeyHost)
	if err != nil {
		return nil, fmt.Errorf("host failed to sign storage contract,error: %v", err)
	}

	scr.Signatures = [][]byte{signByClient, signByHost}
	return scr, nil
}

func mockWriteStorageContractIntoState(sc types.StorageContract, state *state.StateDB) {

	// create the expired storage contract status address (e.g. "expired_storage_contract_1500")
	windowEndStr := strconv.FormatUint(sc.WindowEnd, 10)
	statusAddr := common.BytesToAddress([]byte(coinchargemaintenance.StrPrefixExpSC + windowEndStr))

	// create storage contract address, directly use the contract ID
	scID := sc.ID()
	contractAddr := common.BytesToAddress(scID[12:])

	// create status and contract account
	state.CreateAccount(statusAddr)
	state.CreateAccount(contractAddr)

	// mark this new storage contract as not proofed
	state.SetState(statusAddr, scID, common.BytesToHash(append(coinchargemaintenance.NotProofedStatus, contractAddr[:]...)))

	// store storage contract in this contractAddr's state
	state.SetState(contractAddr, coinchargemaintenance.KeyClientAddress, common.BytesToHash(sc.ClientCollateral.Address.Bytes()))
	state.SetState(contractAddr, coinchargemaintenance.KeyHostAddress, common.BytesToHash(sc.HostCollateral.Address.Bytes()))

	state.SetState(contractAddr, coinchargemaintenance.KeyClientCollateral, common.BytesToHash(sc.ClientCollateral.Value.Bytes()))
	state.SetState(contractAddr, coinchargemaintenance.KeyHostCollateral, common.BytesToHash(sc.HostCollateral.Value.Bytes()))

	uintBytes := Uint64ToBytes(sc.FileSize)
	state.SetState(contractAddr, coinchargemaintenance.KeyFileSize, common.BytesToHash(uintBytes))

	state.SetState(contractAddr, coinchargemaintenance.KeyUnlockHash, sc.UnlockHash)
	state.SetState(contractAddr, coinchargemaintenance.KeyFileMerkleRoot, sc.FileMerkleRoot)

	uintBytes = Uint64ToBytes(sc.RevisionNumber)
	state.SetState(contractAddr, coinchargemaintenance.KeyRevisionNumber, common.BytesToHash(uintBytes))

	uintBytes = Uint64ToBytes(sc.WindowStart)
	state.SetState(contractAddr, coinchargemaintenance.KeyWindowStart, common.BytesToHash(uintBytes))

	uintBytes = Uint64ToBytes(sc.WindowEnd)
	state.SetState(contractAddr, coinchargemaintenance.KeyWindowEnd, common.BytesToHash(uintBytes))

	state.SetState(contractAddr, coinchargemaintenance.KeyClientValidProofOutput, common.BytesToHash(sc.ValidProofOutputs[0].Value.Bytes()))
	state.SetState(contractAddr, coinchargemaintenance.KeyHostValidProofOutput, common.BytesToHash(sc.ValidProofOutputs[1].Value.Bytes()))

	state.SetState(contractAddr, coinchargemaintenance.KeyClientMissedProofOutput, common.BytesToHash(sc.MissedProofOutputs[0].Value.Bytes()))
	state.SetState(contractAddr, coinchargemaintenance.KeyHostMissedProofOutput, common.BytesToHash(sc.MissedProofOutputs[1].Value.Bytes()))
}

func mockStorageProof(prvKeyHost *ecdsa.PrivateKey, parentID common.Hash) (*types.StorageProof, error) {
	sp := &types.StorageProof{
		ParentID: parentID,
		Segment:  [64]byte{},
	}

	sig, err := crypto.Sign(sp.RLPHash().Bytes(), prvKeyHost)
	if err != nil {
		return nil, err
	}

	sp.Signature = sig
	return sp, nil
}
