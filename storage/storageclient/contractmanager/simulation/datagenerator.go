// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Pallinder/go-randomdata"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

func ContractParamsGenerator() storage.ContractParams {
	hostInfo := HostInfoGenerator()
	startHeight := randUint64()
	return storage.ContractParams{
		RentPayment:          RentPaymentGenerator(),
		HostEnodeURL:         hostInfo.EnodeURL,
		Funding:              common.RandomBigInt().Add(hostInfo.ContractPrice),
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

// EnodeIDGenerator will randomly generate enodeID
func EnodeIDGenerator() (id enode.ID) {
	_, _ = rand.Read(id[:])
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
