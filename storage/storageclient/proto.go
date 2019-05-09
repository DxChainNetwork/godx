package storageclient

import (
	"gitlab.com/NebulousLabs/Sia/modules"
	"math/big"
)

type ContractParams struct {
	Allowance    modules.Allowance
	HostEnodeUrl string
	Funding      *big.Int
	StartHeight  uint64
	EndHeight    uint64
}
