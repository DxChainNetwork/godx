// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
)

// ContractRenewNegotiate will try to renew the storage contract with storage host.
// 1. draft the renewed storage contract
// 2. negotiate the drafted renewed storage contract
// 3. create and negotiate storage contract revision
// 4. send the storage contract create transaction, once the storage contract revision negotiation succeed
// 5. commit the contract information, send success message to storage host, and host's response
func (cm *ContractManager) ContractRenewNegotiate(oldContract *contractset.Contract, params storage.ContractParams) (meta storage.ContractMetaData, negotiateErr error) {
	// data preparation
	contractLastRevision := oldContract.Header().LatestContractRevision
	hostInfo, paymentAddress := params.Host, params.ClientPaymentAddress

	// 1. draft the renewStorageContract
	storageContract, err := draftRenewStorageContract(hostInfo, params.Funding, params.StartHeight, params.EndHeight, params.RentPayment, contractLastRevision)
	if err != nil {
		negotiateErr = err
		cm.log.Error("contract renew negotiation failed: failed to create drafted storage contract", "err", err.Error())
		return
	}

	// find the wallet based on the account address, the information is needed to sign
	// the storage contract and storage contract revision
	account := accounts.Account{Address: paymentAddress}
	wallet, err := cm.b.AccountManager().Find(account)
	if err != nil {
		negotiateErr = err
		cm.log.Error("contract renew negotiation failed: failed to get the wallet based on provided account address", "err", err.Error(), "account", account)
		return
	}

	// set up connection
	sp, err := cm.b.SetupConnection(hostInfo.EnodeURL)
	if err != nil {
		negotiateErr = err
		cm.log.Error("contract renew negotiation failed: failed to set up the storage connection", "err", err.Error())
		return
	}

	// handleNegotiationErr will handle errors occurred in the negotiation process
	defer cm.handleNegotiationErr(&negotiateErr, hostInfo.EnodeID, sp)

	// 2. drafted storage contract negotiation
	if storageContract, err = draftStorageContractNegotiate(sp, account, wallet, storageContract, contractLastRevision); err != nil {
		negotiateErr = err
		cm.log.Error("contract renew negotiation failed: failed to negotiate the drafted contract", "err", err.Error())
		return
	}

	// 3. storage contract revision negotiation
	storageContractRevision, err := storageContractRevisionNegotiate(sp, storageContract, contractLastRevision.UnlockConditions, account, wallet)
	if err != nil {
		negotiateErr = err
		cm.log.Error("contract renew negotiation failed: failed to negotiate the storage contract revision", "err", err.Error)
		return
	}

	// 4. send the storage contract create transaction
	if err := sendStorageContractCreateTx(storageContract, paymentAddress, cm.b); err != nil {
		negotiateErr = err
		cm.log.Error("contract renew negotiation failed: failed to send the storage contract create transaction", "err", err.Error())
		return
	}

	// 5. commit the contract information, send success message to host, and handle host's response
	meta, negotiateErr = cm.clientStorageContractCommit(sp, hostInfo.EnodeID, params.StartHeight, params.Funding, hostInfo.ContractPrice, storageContract.ID(), storageContractRevision, oldContract)
	return
}

// draftRenewStorageContract will calculate the payouts and drafted a storage contract for storage host to be reviewed
// 1. calculate the base price and base deposit
// 2. calculate the client and host payouts
// 3. validate the base deposit, avoid negative currency
// 4. get the client and host address, draft the storage contract
func draftRenewStorageContract(hostInfo storage.HostInfo, funding common.BigInt, startHeight uint64, endHeight uint64, rentPayment storage.RentPayment, revision types.StorageContractRevision) (types.StorageContract, error) {
	// 1. calculate the base price and base deposit
	basePrice, baseDeposit := calculateBasePriceAndBaseDeposit(endHeight, hostInfo.WindowSize, hostInfo.StoragePrice, hostInfo.Deposit, revision)

	// 2. calculate the client and host payouts
	clientPayout, hostPayout, hostDeposit, err := calculatePayoutsAndHostDeposit(hostInfo, funding, basePrice, baseDeposit, startHeight, endHeight, rentPayment)
	if err != nil {
		return types.StorageContract{}, fmt.Errorf("failed to calculate client and host payout: %s", err.Error())
	}

	// 3. validate the base deposit
	if hostDeposit.Cmp(baseDeposit) < 0 {
		baseDeposit = hostDeposit
	}

	// 4. get the client and host address, draft the storage contract
	clientAddress, hostAddress := getPaymentAddresses(revision)

	storageContract := types.StorageContract{
		FileSize:         revision.NewFileSize,
		FileMerkleRoot:   revision.NewFileMerkleRoot,
		WindowStart:      endHeight,
		WindowEnd:        endHeight + hostInfo.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout.BigIntPtr(), Address: clientAddress}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout.BigIntPtr(), Address: hostAddress}},
		UnlockHash:       revision.NewUnlockHash,
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout.BigIntPtr(), Address: clientAddress},
			// Deposit is returned to host
			{Value: hostPayout.BigIntPtr(), Address: hostAddress},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout.BigIntPtr(), Address: clientAddress},
			{Value: hostPayout.Sub(baseDeposit).BigIntPtr(), Address: hostAddress},
		},
	}

	return storageContract, nil
}

// calculateBasePriceAndBaseDeposit will calculate the basePrice and baseDeposit
func calculateBasePriceAndBaseDeposit(endHeight uint64, windowSize uint64, storagePrice common.BigInt, deposit common.BigInt, revision types.StorageContractRevision) (basePrice, baseDeposit common.BigInt) {
	if endHeight+windowSize > revision.NewWindowEnd {
		timeExtension := uint64(endHeight+windowSize) - revision.NewWindowEnd
		basePrice = storagePrice.Mult(common.NewBigIntUint64(revision.NewFileSize)).Mult(common.NewBigIntUint64(timeExtension))
		baseDeposit = deposit.Mult(common.NewBigIntUint64(revision.NewFileSize)).Mult(common.NewBigIntUint64(timeExtension))
	}

	return
}

// getPaymentAddresses will extract payment addresses from the contract revision
func getPaymentAddresses(revision types.StorageContractRevision) (clientAddress common.Address, hostAddress common.Address) {
	clientAddress = revision.NewValidProofOutputs[0].Address
	hostAddress = revision.NewValidProofOutputs[1].Address
	return
}
