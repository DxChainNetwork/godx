// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"

	"github.com/Pallinder/go-randomdata"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// randomContractGenerator will randomly generate a storage contract
func ContractGenerator(contractEndHeight uint64) (ch contractset.ContractHeader) {
	clientAddress := AddressGenerator()
	hostAddress := AddressGenerator()
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      ContractIDGenerator(),
		EnodeID: EnodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			NewWindowStart:    contractEndHeight,
			ParentID:          HashGenerator(),
			NewRevisionNumber: 15,
			NewValidProofOutputs: []types.DxcoinCharge{
				{clientAddress, big.NewInt(100000)},
				{hostAddress, big.NewInt(100000)},
			},
			UnlockConditions: types.UnlockConditions{
				PaymentAddresses: []common.Address{
					clientAddress,
					hostAddress,
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

func ContractParamsGenerator() storage.ContractParams {
	hostInfo := HostInfoGenerator()
	startHeight := randUint64()
	return storage.ContractParams{
		RentPayment:          RentPaymentGenerator(),
		HostEnodeURL:         hostInfo.EnodeURL,
		Funding:              common.RandomBigInt().MultUint64(100000000000000000),
		StartHeight:          startHeight,
		EndHeight:            startHeight + randUint64(),
		ClientPaymentAddress: common.Address{},
		Host:                 hostInfo,
	}
}

// RentPaymentGenerator will randomly generate rentPayment information
func RentPaymentGenerator() (rentPayment storage.RentPayment) {
	rentPayment = storage.RentPayment{
		Fund:               common.RandomBigInt(),
		StorageHosts:       randUint64(),
		Period:             randUint64(),
		RenewWindow:        randUint64(),
		ExpectedStorage:    randUint64(),
		ExpectedUpload:     randUint64(),
		ExpectedDownload:   randUint64(),
		ExpectedRedundancy: randFloat64(),
	}

	return
}

// HostInfoGenerator will randomly generate hostInfo information
func HostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := EnodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.NewBigInt(100),
			ContractPrice:          common.RandomBigInt(),
			DownloadBandwidthPrice: common.RandomBigInt(),
			StoragePrice:           common.RandomBigInt(),
			UploadBandwidthPrice:   common.RandomBigInt(),
			SectorAccessPrice:      common.RandomBigInt(),
			RemainingStorage:       100,
		},
		IP:       ip,
		EnodeID:  id,
		EnodeURL: fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
	}
}

// UploadActionsGenerator will randomly generate a list of storage uploadActions
func UploadActionsGenerator(amount int, actionType string) (uploadActions []storage.UploadAction) {
	for i := 0; i < amount; i++ {
		action := storage.UploadAction{
			Type: actionType,
			A:    0,
			B:    0,
			Data: []byte{},
		}

		uploadActions = append(uploadActions, action)
	}
	return
}

// EnodeIDGenerator will randomly generate enodeID
func EnodeIDGenerator() (id enode.ID) {
	_, _ = rand.Read(id[:])
	return
}

// randomRootsGenerator will randomly generate merkle roots for the storage contract
func RootsGenerator(rootCount int) (roots []common.Hash) {
	for i := 0; i < rootCount; i++ {
		roots = append(roots, HashGenerator())
	}
	return
}

// randomAddressGenerator will randomly generate some addresses used for DxcoinCharge
func AddressGenerator() (a common.Address) {
	rand.Read(a[:])
	return
}

// storageContractIDGenerator will randomly generate storage contract id
func ContractIDGenerator() (id storage.ContractID) {
	rand.Read(id[:])
	return
}

// randomHashGenerator will randomly generate common.Hash value
func HashGenerator() (h common.Hash) {
	rand.Read(h[:])
	return
}

// randByteSlice will generate a random byte slice
func randByteSlice() (byteSlice []byte) {
	rand.Read(byteSlice[:])
	return
}

func randUint64() (randUint uint64) {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint64()
}

func randFloat64() (randFloat float64) {
	rand.Seed(time.Now().UnixNano())
	return rand.Float64()
}

func randInt64() (randBool int64) {
	rand.Seed(time.Now().UnixNano())
	return int64(rand.Int())
}
