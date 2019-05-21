// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// contractHeader specifies contract's general information
type contractHeader struct {
	// TODO (mzhang): added one more field, sync with HZ office
	// reason: convenience, a contract has only one ID anyway
	//
	// TODO (mzhang): double check with HZ to see if the information
	// can be acquired somewhere else
	ID      storage.ContractID
	EnodeID enode.ID

	// TODO (mzhang): communicate to HZ office, this field is needed to access
	// all the contracts signed by the client
	StorageTransaction types.Transaction

	// TODO (mzhang): changed the private key field type, sync with HZ office
	PrivateKey *ecdsa.PrivateKey

	StartHeight uint64

	// contract cost
	UploadCost   common.BigInt
	DownloadCost common.BigInt
	StorageCost  common.BigInt
	TotalCost    common.BigInt
	GasFee       common.BigInt
	ContractFee  common.BigInt

	// status specifies if the contract is good for file uploading or renewing.
	// it also specifies if the contract is canceled
	Status storage.ContractStatus
}
