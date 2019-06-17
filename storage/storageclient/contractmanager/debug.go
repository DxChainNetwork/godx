// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"math/big"
)

// InsertRandomActiveContracts will create some random contracts and inserted
// into active contract list
func (cm *ContractManager) InsertRandomActiveContracts(amount int) (err error) {
	for i := 0; i < amount; i++ {
		contract := randomContractGenerator(100)
		if _, err = cm.activeContracts.InsertContract(contract, randomRootsGenerator(10)); err != nil {
			return
		}
	}
	return
}

// randomContractGenerator will randomly generate a storage contract
func randomContractGenerator(contractEndHeight uint64) (ch contractset.ContractHeader) {
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: randomEnodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			NewWindowStart:    contractEndHeight,
			ParentID:          randomHashGenerator(),
			NewRevisionNumber: 15,
			NewValidProofOutputs: []types.DxcoinCharge{
				{randomAddressGenerator(), big.NewInt(0)},
			},
			UnlockConditions: types.UnlockConditions{
				PaymentAddresses: []common.Address{
					randomAddressGenerator(),
					randomAddressGenerator(),
				},
			},
		},
		PrivateKey:   "12345678910",
		StartHeight:  100,
		DownloadCost: common.RandomBigInt(),
		UploadCost:   common.RandomBigInt(),
		TotalCost:    common.RandomBigInt(),
		StorageCost:  common.RandomBigInt(),
		GasFee:       common.RandomBigInt(),
		ContractFee:  common.RandomBigInt(),
		Status: storage.ContractStatus{
			UploadAbility: true,
			RenewAbility:  true,
			Canceled:      false,
		},
	}

	return
}

// randomRootsGenerator will randomly generate merkle roots for the storage contract
func randomRootsGenerator(rootCount int) (roots []common.Hash) {
	for i := 0; i < rootCount; i++ {
		roots = append(roots, randomHashGenerator())
	}
	return
}

// randomEnodeIDGenerator will randomly generator some enode id
func randomEnodeIDGenerator() (id enode.ID) {
	rand.Read(id[:])
	return
}

// randomAddressGenerator will randomly generate some addresses used for DxcoinCharge
func randomAddressGenerator() (a common.Address) {
	rand.Read(a[:])
	return
}

// storageContractIDGenerator will randomly generate storage contract id
func storageContractIDGenerator() (id storage.ContractID) {
	rand.Read(id[:])
	return
}

// randomHashGenerator will randomly generate common.Hash value
func randomHashGenerator() (h common.Hash) {
	rand.Read(h[:])
	return
}
