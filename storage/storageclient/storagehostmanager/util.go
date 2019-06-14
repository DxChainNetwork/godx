// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
	"github.com/Pallinder/go-randomdata"
)

// StorageHostRank will be used to show the rankings of the storage host
// learnt by the storage client
type StorageHostRank struct {
	storagehosttree.EvaluationDetail
	EnodeID string
}

// hostInfoGenerator will randomly generate storage host information
func hostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := enodeIDGenerator()
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

// activeHostInfoGenerator will randomly generate active storage host information
func activeHostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := enodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.RandomBigInt(),
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
		ScanRecords: storage.HostPoolScans{
			storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			},
		},
	}
}

func hostInfoGeneratorIPID(ip string, id enode.ID, ipChanged time.Time) storage.HostInfo {
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.RandomBigInt(),
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
		ScanRecords: storage.HostPoolScans{
			storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			},
		},
		LastIPNetWorkChange: ipChanged,
	}
}

func hostInfoGeneratorHighEvaluation(id enode.ID) storage.HostInfo {
	ip := randomdata.IpV4Address()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.NewBigIntUint64(18446744073709551615).MultUint64(18446744073709551615).MultUint64(18446744073709551615).MultUint64(18446744073709551615),
			ContractPrice:          common.BigInt1,
			DownloadBandwidthPrice: common.BigInt1,
			StoragePrice:           common.BigInt1,
			UploadBandwidthPrice:   common.BigInt1,
			SectorAccessPrice:      common.BigInt1,
			RemainingStorage:       200 * 20e10,
			MaxDeposit:             common.PtrBigInt(new(big.Int).Exp(big.NewInt(1000), big.NewInt(1000), nil)).MultUint64(10e10),
		},
		IP:       ip,
		EnodeID:  id,
		EnodeURL: fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
		ScanRecords: storage.HostPoolScans{
			storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			},
		},
		HistoricSuccessfulInteractions: 500,
		HistoricFailedInteractions:     0,
		FirstSeen:                      0,
	}
}

func hostInfoGeneratorHighEvaluationOffline(id enode.ID) storage.HostInfo {
	ip := randomdata.IpV4Address()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.NewBigInt(1000 * 20003e10),
			ContractPrice:          common.BigInt1,
			DownloadBandwidthPrice: common.BigInt1,
			StoragePrice:           common.BigInt1,
			UploadBandwidthPrice:   common.BigInt1,
			SectorAccessPrice:      common.BigInt1,
			RemainingStorage:       200 * 20e10,
		},
		IP:                             ip,
		EnodeID:                        id,
		EnodeURL:                       fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
		HistoricSuccessfulInteractions: 500,
		HistoricFailedInteractions:     0,
		FirstSeen:                      0,
	}
}

func hostInfoGeneratorLowEvaluation(id enode.ID) storage.HostInfo {
	ip := randomdata.IpV4Address()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.NewBigInt(5),
			ContractPrice:          common.NewBigInt(1000 * 20003e10),
			DownloadBandwidthPrice: common.NewBigInt(1000 * 20003e10),
			StoragePrice:           common.NewBigInt(1000 * 20003e10),
			UploadBandwidthPrice:   common.NewBigInt(1000 * 20003e10),
			SectorAccessPrice:      common.NewBigInt(1000 * 20003e10),
			RemainingStorage:       1,
		},
		IP:                             ip,
		EnodeID:                        id,
		EnodeURL:                       fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
		HistoricSuccessfulInteractions: 0,
		HistoricFailedInteractions:     500,
		FirstSeen:                      0,
	}
}

// enodeIDGenerator will randomly generate enode ID
func enodeIDGenerator() (id enode.ID) {
	rand.Read(id[:])
	return
}
