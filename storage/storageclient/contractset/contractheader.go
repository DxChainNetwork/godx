// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"errors"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// ContractHeader specifies contract's general information
type ContractHeader struct {
	ID      storage.ContractID
	EnodeID enode.ID

	// TODO (mzhang): latest contract revision information, sync with HZ Office
	LatestContractRevision types.StorageContractRevision

	// TODO (mzhang): changed the private key field type, sync with HZ Office
	// Convert the PrivateKey from string to ecdsa private key
	// and convert the ecdsa private key to string
	//
	//Encode Private Key to String: hex.EncodeToString(crypto.FromECDSA(k.PrivateKey))
	//Change the string back to private key: privkey, err := crypto.HexToECDSA(keyJSON.PrivateKey)
	PrivateKey string

	StartHeight uint64
	EndHeight   uint64

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

func (ch *ContractHeader) validation() (err error) {
	if ch.LatestContractRevision.NewRevisionNumber > 0 &&
		len(ch.LatestContractRevision.NewValidProofOutputs) > 0 &&
		len(ch.LatestContractRevision.UnlockConditions.PaymentAddresses) == 2 {
		return
	}

	err = errors.New("invalid contract header")
	return
}

func (ch *ContractHeader) GetLatestContractRevision() types.StorageContractRevision {
	return ch.LatestContractRevision
}
