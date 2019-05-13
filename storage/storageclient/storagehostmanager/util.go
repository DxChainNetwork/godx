package storagehostmanager

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/Pallinder/go-randomdata"
	"time"
)

func hostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := enodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
			Deposit: common.NewBigInt(100),
			ContractPrice: common.RandomBigInt(),
			DownloadBandwidthPrice: common.RandomBigInt(),
			StoragePrice: common.RandomBigInt(),
			UploadBandwidthPrice: common.RandomBigInt(),
			SectorAccessPrice: common.RandomBigInt(),
			RemainingStorage: 100,
		},
		IP:       ip,
		EnodeID:  id,
		EnodeURL: fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
	}
}

func activeHostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := enodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
			Deposit: common.RandomBigInt(),
			ContractPrice: common.RandomBigInt(),
			DownloadBandwidthPrice: common.RandomBigInt(),
			StoragePrice: common.RandomBigInt(),
			UploadBandwidthPrice: common.RandomBigInt(),
			SectorAccessPrice: common.RandomBigInt(),
			RemainingStorage: 100,
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

func enodeIDGenerator() enode.ID {
	id := make([]byte, 32)
	rand.Read(id)
	var result [32]byte
	copy(result[:], id[:32])
	return enode.ID(result)
}
