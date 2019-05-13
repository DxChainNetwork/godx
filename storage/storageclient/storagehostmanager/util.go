package storagehostmanager

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/Pallinder/go-randomdata"
)

func hostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := enodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
		},
		IP:       ip,
		EnodeID:  id,
		EnodeURL: fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
	}
}

func enodeIDGenerator() enode.ID {
	id := make([]byte, 32)
	rand.Read(id)
	var result [32]byte
	copy(result[:], id[:32])
	return enode.ID(result)
}
