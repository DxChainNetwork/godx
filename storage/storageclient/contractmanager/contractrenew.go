// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/proto"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

var (
	zeroValue = new(big.Int).SetInt64(0)

	extraRatio = 0.02
)

// checkForContractRenew will loop through all active contracts and filter out those needs to be renewed.
// There are two types of contract needs to be renewed
// 		1. contracts that are about to expired. they need to be renewed
// 		2. contracts that have insufficient amount of funding, meaning the contract is about to be
// 		   marked as not good for data uploading
func (cm *ContractManager) checkForContractRenew() (closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {
	cm.lock.RLock()
	currentBlockHeight := cm.blockHeight
	rentPayment := cm.rentPayment
	cm.lock.RUnlock()

	// loop through all active contracts, get the priorityRenews and emptyRenews
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// validate the storage host for the contract, check if the host exists or get filtered
		host, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
		if !exists || host.Filtered {
			continue
		}

		// verify if the contract is good for renew
		if !contract.Status.RenewAbility {
			continue
		}

		// for contract that is about to expire, it will be added to the priorityRenews
		// calculate the renewCostEstimation and update the priorityRenews
		if currentBlockHeight+rentPayment.RenewWindow >= contract.EndHeight {
			estimateContractRenewCost := cm.renewCostEstimation(host, contract, currentBlockHeight, rentPayment)
			closeToExpireRenews = append(closeToExpireRenews, contractRenewRecord{
				id:   contract.ID,
				cost: estimateContractRenewCost,
			})
			continue
		}

		// for those contracts has insufficient funding, they should be renewed because otherwise
		// after a while, they will be marked as not good for upload
		sectorStorageCost := host.StoragePrice.MultUint64(contractset.SectorSize * rentPayment.Period)
		sectorUploadBandwidthCost := host.UploadBandwidthPrice.MultUint64(contractset.SectorSize)
		sectorDownloadBandwidthCost := host.DownloadBandwidthPrice.MultUint64(contractset.SectorSize)
		totalSectorCost := sectorUploadBandwidthCost.Add(sectorDownloadBandwidthCost).Add(sectorStorageCost)

		remainingBalancePercentage := contract.ClientBalance.Div(contract.TotalCost).Float64()

		if contract.ClientBalance.Cmp(totalSectorCost.MultUint64(3)) < 0 || remainingBalancePercentage < minContractPaymentRenewalThreshold {
			insufficientFundingRenews = append(insufficientFundingRenews, contractRenewRecord{
				id:   contract.ID,
				cost: contract.TotalCost.MultUint64(2),
			})
		}
	}

	return
}

// resetFailedRenews will update the failedRenews list, which only includes the failedRenews
// in the current renew lists
func (cm *ContractManager) resetFailedRenews(closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {
	cm.lock.Lock()
	filteredFailedRenews := make(map[storage.ContractID]uint64)

	// loop through the closeToExpireRenews, get the failedRenews
	for _, renewRecord := range closeToExpireRenews {
		contractID := renewRecord.id
		if _, exists := cm.failedRenews[contractID]; exists {
			filteredFailedRenews[contractID] = cm.failedRenews[contractID]
		}
	}

	// loop through
	for _, renewRecord := range insufficientFundingRenews {
		contractID := renewRecord.id
		if _, exists := cm.failedRenews[contractID]; exists {
			filteredFailedRenews[contractID] = cm.failedRenews[contractID]
		}
	}

	// reset the failedRenews
	cm.failedRenews = filteredFailedRenews
	cm.lock.Unlock()
}

// prepareContractRenew will loop through all record in the renewRecords and start to renew
// each contract. Before contract renewing get started, the fund will be validated first.
func (cm *ContractManager) prepareContractRenew(renewRecords []contractRenewRecord, clientRemainingFund common.BigInt) (remainingFund common.BigInt, terminate bool) {
	// get the data needed
	cm.lock.RLock()
	rentPayment := cm.rentPayment
	blockHeight := cm.blockHeight
	currentPeriod := cm.currentPeriod
	contractEndHeight := cm.currentPeriod + cm.rentPayment.Period + cm.rentPayment.RenewWindow
	cm.lock.RUnlock()

	for _, record := range renewRecords {
		// verify that the cost needed for contract renew does not exceed the clientRemainingFund
		if clientRemainingFund.Cmp(record.cost) < 0 {
			continue
		}

		// renew the contract, get the spending for the renew
		renewCost, err := cm.renewContract(record, currentPeriod, rentPayment, blockHeight, contractEndHeight)
		if err != nil {
			cm.log.Error(fmt.Sprintf("contract renew failed. id: %v, err : %v", record.id, err.Error()))
		}

		// update the remaining fund
		remainingFund = clientRemainingFund.Sub(renewCost)

		// check maintenance termination
		if terminate = cm.checkMaintenanceTermination(); terminate {
			return
		}
	}

	return
}

// renewContract will start to perform contract renew operation
func (cm *ContractManager) renewContract(record contractRenewRecord, currentPeriod uint64, rentPayment storage.RentPayment, blockHeight uint64, contractEndHeight uint64) (renewCost common.BigInt, err error) {
	// TODO (mzhang): renewContarct
	return
}

func (cm *ContractManager) ContractRenew(oldContract *contractset.Contract, params proto.ContractParams) (storage.ContractMetaData, error) {

	contract := oldContract.Header()

	lastRev := contract.GetLatestContractRevision()

	// Extract vars from params, for convenience
	allowance, funding, clientPublicKey, startHeight, endHeight, host := params.Allowance, params.Funding, params.ClientPublicKey, params.StartHeight, params.EndHeight, params.Host

	var basePrice, baseCollateral *big.Int
	if endHeight+host.WindowSize > lastRev.NewWindowEnd {
		timeExtension := uint64((endHeight + host.WindowSize) - lastRev.NewWindowEnd)
		basePrice = new(big.Int).Mul(host.StoragePrice, new(big.Int).SetUint64(lastRev.NewFileSize))
		basePrice = new(big.Int).Mul(basePrice, new(big.Int).SetUint64(timeExtension))
		// cost of already uploaded data that needs to be covered by the renewed contract.
		baseCollateral = new(big.Int).Mul(host.Collateral, new(big.Int).SetUint64(lastRev.NewFileSize))
		baseCollateral = new(big.Int).Mul(baseCollateral, new(big.Int).SetUint64(timeExtension))
		// same as basePrice.
	}

	// Calculate the payouts for the client, host, and whole contract
	period := endHeight - startHeight
	expectedStorage := allowance.ExpectedStorage / allowance.Hosts
	clientPayout, hostPayout, hostCollateral, err := ClientPayoutsPreTax(host, funding, basePrice, baseCollateral, period, expectedStorage)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// check for negative currency
	if hostCollateral.Cmp(baseCollateral) < 0 {
		baseCollateral = hostCollateral
	}

	clientAddr := crypto.PubkeyToAddress(clientPublicKey)
	hostAddr := crypto.PubkeyToAddress(host.PublicKey)
	var hostMiss *big.Int
	hostMiss = new(big.Int).Sub(hostCollateral, baseCollateral)
	hostMiss = new(big.Int).Add(hostMiss, host.ContractPrice)
	// Create storage contract
	storageContract := types.StorageContract{
		FileSize:         lastRev.NewFileSize,
		FileMerkleRoot:   lastRev.NewFileMerkleRoot, // no proof possible without data
		WindowStart:      endHeight,
		WindowEnd:        endHeight + host.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout}},
		UnlockHash:       lastRev.NewUnlockHash,
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout, Address: clientAddr},
			// Deposit is returned to host
			{Value: hostPayout, Address: hostAddr},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout, Address: clientAddr},
			{Value: hostMiss, Address: hostAddr},
		},
	}

	// Increase Successful/Failed interactions accordingly
	defer func() {
		if err != nil {
			cm.hostManager.IncrementFailedInteractions(contract.EnodeID)
		} else {
			cm.hostManager.IncrementSuccessfulInteractions(contract.EnodeID)
		}
	}()

	account := accounts.Account{Address: clientAddr}
	wallet, err := cm.b.AccountManager().Find(account)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("find client account error", err)
	}

	// Setup connection with storage host
	session, err := cm.b.SetupConnection(host.NetAddress)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("setup connection with host failed", err)
	}
	defer cm.b.Disconnect(session, host.NetAddress)

	clientContractSign, err := wallet.SignHash(account, storageContract.RLPHash().Bytes())
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("contract sign by client failed", err)
	}

	// Send the ContractCreate request
	req := storage.ContractCreateRequest{
		StorageContract: storageContract,
		Sign:            clientContractSign,
	}

	if err := session.SendStorageContractCreation(req); err != nil {
		return storage.ContractMetaData{}, err
	}

	var hostSign []byte
	msg, err := session.ReadMsg()
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return storage.ContractMetaData{}, negotiationErr
	}

	if err := msg.Decode(&hostSign); err != nil {
		return storage.ContractMetaData{}, err
	}

	storageContract.Signatures[0] = clientContractSign
	storageContract.Signatures[1] = hostSign

	// Assemble init revision and sign it
	storageContractRevision := types.StorageContractRevision{
		ParentID:              storageContract.RLPHash(),
		UnlockConditions:      lastRev.UnlockConditions,
		NewRevisionNumber:     1,
		NewFileSize:           storageContract.FileSize,
		NewFileMerkleRoot:     storageContract.FileMerkleRoot,
		NewWindowStart:        storageContract.WindowStart,
		NewWindowEnd:          storageContract.WindowEnd,
		NewValidProofOutputs:  storageContract.ValidProofOutputs,
		NewMissedProofOutputs: storageContract.MissedProofOutputs,
		NewUnlockHash:         storageContract.UnlockHash,
	}

	clientRevisionSign, err := wallet.SignHash(account, storageContractRevision.RLPHash().Bytes())
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("client sign revision error", err)
	}
	storageContractRevision.Signatures = [][]byte{clientRevisionSign}

	if err := session.SendStorageContractCreationClientRevisionSign(clientRevisionSign); err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("send revision sign by client error", err)
	}

	var hostRevisionSign []byte
	msg, err = session.ReadMsg()
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return storage.ContractMetaData{}, negotiationErr
	}

	if err := msg.Decode(&hostRevisionSign); err != nil {
		return storage.ContractMetaData{}, err
	}

	scBytes, err := rlp.EncodeToBytes(storageContract)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	if _, err := storage.SendFormContractTX(cm.b, clientAddr, scBytes); err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("Send storage contract transaction error", err)
	}

	// wrap some information about this contract
	header := contractset.ContractHeader{
		ID:                     storage.ContractID(storageContract.ID()),
		EnodeID:                PubkeyToEnodeID(&host.PublicKey),
		StartHeight:            startHeight,
		EndHeight:              endHeight,
		TotalCost:              common.NewBigInt(funding.Int64()),
		ContractFee:            common.NewBigInt(host.ContractPrice.Int64()),
		LatestContractRevision: storageContractRevision,
		Status: storage.ContractStatus{
			UploadAbility: true,
			RenewAbility:  true,
		},
	}

	oldRoots, errRoots := oldContract.MerkleRoots()
	if errRoots != nil {
		return storage.ContractMetaData{}, errRoots
	}

	// store this contract info to client local
	contractMetaData, errInsert := cm.GetStorageContractSet().InsertContract(header, oldRoots)
	if errInsert != nil {
		return storage.ContractMetaData{}, errInsert
	}
	return contractMetaData, nil
}

// calculate Enode.ID, reference:
// p2p/discover/node.go:41
// p2p/discover/node.go:59
func PubkeyToEnodeID(pubkey *ecdsa.PublicKey) enode.ID {
	var pubBytes [64]byte
	math.ReadBits(pubkey.X, pubBytes[:len(pubBytes)/2])
	math.ReadBits(pubkey.Y, pubBytes[len(pubBytes)/2:])
	return enode.ID(crypto.Keccak256Hash(pubBytes[:]))
}

// calculate client and host collateral
func ClientPayoutsPreTax(host proto.StorageHostEntry, funding, basePrice, baseCollateral *big.Int, period, expectedStorage uint64) (clientPayout, hostPayout, hostCollateral *big.Int, err error) {

	// Divide by zero check.
	if host.StoragePrice.Sign() == 0 {
		host.StoragePrice.SetInt64(1)
	}

	// Underflow check.
	if funding.Cmp(host.ContractPrice) <= 0 {
		err = errors.New("underflow detected, funding < contractPrice")
		return
	}

	// Calculate clientPayout.
	clientPayout = new(big.Int).Sub(funding, host.ContractPrice)
	clientPayout = clientPayout.Sub(clientPayout, basePrice)

	// Calculate hostCollateral
	maxStorageSizeTime := new(big.Int).Div(clientPayout, host.StoragePrice)
	maxStorageSizeTime = maxStorageSizeTime.Mul(maxStorageSizeTime, host.Collateral)
	hostCollateral = maxStorageSizeTime.Add(maxStorageSizeTime, baseCollateral)
	host.Collateral = host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(period))
	host.Collateral = host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(expectedStorage))
	maxClientCollateral := host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(5))
	if hostCollateral.Cmp(maxClientCollateral) > 0 {
		hostCollateral = maxClientCollateral
	}

	// Don't add more collateral than the host is willing to put into a single
	// contract.
	if hostCollateral.Cmp(host.MaxCollateral) > 0 {
		hostCollateral = host.MaxCollateral
	}

	// Calculate hostPayout.
	hostCollateral.Add(hostCollateral, host.ContractPrice)
	hostPayout = hostCollateral.Add(hostCollateral, basePrice)
	return
}

func (cm *ContractManager) managedRenew(contract *contractset.Contract, contractFunding *big.Int, newEndHeight uint64, allowance proto.Allowance, entry proto.StorageHostEntry, hostEnodeUrl string, clientPublic ecdsa.PublicKey) (storage.ContractMetaData, error) {

	//Check if the storage contract ID meets the renew condition
	status, ok := cm.managedContractStatus(contract.Header().ID)
	if !ok || !status.RenewAbility {
		return storage.ContractMetaData{}, errors.New("Condition not satisfied")
	}

	cm.lock.RLock()
	//Calculate the required parameters
	//TODO ClientPublicKey、HostEnodeUrl、Host、Allowance ?
	params := proto.ContractParams{
		Allowance:       allowance,
		Host:            entry,
		Funding:         contractFunding,
		StartHeight:     cm.blockHeight,
		EndHeight:       newEndHeight,
		HostEnodeUrl:    hostEnodeUrl,
		ClientPublicKey: clientPublic,
	}
	cm.lock.RUnlock()

	newContract, errRenew := cm.ContractRenew(contract, params)
	if errRenew != nil {
		return storage.ContractMetaData{}, errRenew
	}

	return newContract, nil
}

func (cm *ContractManager) managedContractStatus(id storage.ContractID) (storage.ContractStatus, bool) {
	//Concurrently secure access to contract status
	mc, exists := cm.activeContracts.Acquire(id)
	if !exists {
		return storage.ContractStatus{}, false
	}

	return mc.Status(), true
}
