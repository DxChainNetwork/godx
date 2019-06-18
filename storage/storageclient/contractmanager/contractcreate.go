// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func (cm *ContractManager) prepareCreateContract(neededContracts int, clientRemainingFund common.BigInt, rentPayment storage.RentPayment) (terminated bool, err error) {

	cm.log.Debug("Prepare to create the contract")

	// get some random hosts for contract formation
	randomHosts, err := cm.randomHostsForContractForm(neededContracts)
	if err != nil {
		return
	}

	cm.log.Debug("randomly acquired hosts from the storage host manager", "amount of storage hosts", len(randomHosts))

	cm.lock.RLock()
	contractFund := rentPayment.Fund.DivUint64(rentPayment.StorageHosts).DivUint64(3)
	contractEndHeight := cm.currentPeriod + rentPayment.Period + rentPayment.RenewWindow
	cm.lock.RUnlock()

	// loop through each host and try to form contract with them
	for _, host := range randomHosts {
		// check if the client has enough fund for forming contract
		if contractFund.Cmp(clientRemainingFund) > 0 {
			err = fmt.Errorf("the contract fund %v is larger than client remaining fund %v. Impossible to create contract",
				contractFund, clientRemainingFund)
			return
		}

		// start to form contract
		formCost, contract, errFormContract := cm.createContract(host, contractFund, contractEndHeight, rentPayment)

		// if contract formation failed, the error do not need to be returned, just try to form the
		// contract with another storage host
		if errFormContract != nil {
			cm.log.Info("trying to form contract with %v, failed: %s", host.EnodeID, err.Error())
			continue
		}

		// update the client remaining fund, and try to change the newly formed contract's status
		clientRemainingFund = clientRemainingFund.Sub(formCost)
		if err = cm.markNewlyFormedContractStats(contract.ID); err != nil {
			return
		}

		// save persistently
		if failedSave := cm.saveSettings(); failedSave != nil {
			cm.log.Warn("after formed the contract, failed to save the contract manager settings")
		}

		// update the number of needed contracts
		neededContracts--
		if neededContracts <= 0 {
			break
		}

		// check if the maintenance termination signal was sent
		if terminated = cm.checkMaintenanceTermination(); terminated {
			break
		}
	}

	return
}

// createContract will try to create the contract with the host that caller passed in:
// 		1. storage host validation
// 		2. form the contract create parameters
// 		3. start to create the contract
// 		4. update the contract manager fields
func (cm *ContractManager) createContract(host storage.HostInfo, contractFund common.BigInt, contractEndHeight uint64, rentPayment storage.RentPayment) (formCost common.BigInt, newlyCreatedContract storage.ContractMetaData, err error) {
	// 1. storage host validation
	// validate the storage price
	if host.StoragePrice.Cmp(maxHostStoragePrice) > 0 {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host: %v, the storage price is too high", host.EnodeID)
		return
	}

	// validate the storage host max deposit
	if host.MaxDeposit.Cmp(maxHostDeposit) > 0 {
		host.MaxDeposit = maxHostDeposit
	}

	// validate the storage host max duration
	if host.MaxDuration < rentPayment.Period {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host: %v, the max duration is smaller than period", host.EnodeID)
		return
	}

	// 2. form the contract create parameters
	// The reason to get the newest blockHeight here is that during the checking time period
	// many blocks may be generated already, which is unfair to the storage client.
	cm.lock.RLock()
	startHeight := cm.blockHeight
	cm.lock.RUnlock()

	// try to get the clientPaymentAddress. If failed, return error directly and set the contract creation cost
	// to be zero
	var clientPaymentAddress common.Address
	if clientPaymentAddress, err = cm.b.GetPaymentAddress(); err != nil {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host: %v, failed to get the clientPayment address: %s", host.EnodeID, err.Error())
		return
	}

	// form the contract create parameters
	params := storage.ContractParams{
		Allowance:            rentPayment,
		HostEnodeUrl:         host.EnodeURL,
		Funding:              contractFund,
		StartHeight:          startHeight,
		EndHeight:            contractEndHeight,
		ClientPaymentAddress: clientPaymentAddress,
		Host:                 host,
	}

	// 3. create the contract
	if newlyCreatedContract, err = cm.ContractCreate(params); err != nil {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host %v: %s", newlyCreatedContract.EnodeID, err.Error())
		return
	}

	// 4. update the contract manager fields
	cm.lock.Lock()
	// check if the storage client have created another contract with the same storage host
	if _, exists := cm.hostToContract[newlyCreatedContract.EnodeID]; exists {
		cm.lock.Unlock()
		formCost = contractFund
		err = fmt.Errorf("client already formed a contract with the same storage host %v", newlyCreatedContract.EnodeID)
		return
	}

	// if not exists, update the host to contract mapping
	cm.hostToContract[newlyCreatedContract.EnodeID] = newlyCreatedContract.ID
	cm.lock.Unlock()

	formCost = contractFund
	return
}

// randomHostsForContractForm will randomly retrieve some storage hosts from the storage host pool
func (cm *ContractManager) randomHostsForContractForm(neededContracts int) (randomHosts []storage.HostInfo, err error) {
	// for all active contracts, the storage host will be added to be blacklist
	// for all active contracts which are not canceled, good for uploading, and renewing
	// the storage host will be added to the addressBlackList
	var blackList []enode.ID
	var addressBlackList []enode.ID
	activeContracts := cm.activeContracts.RetrieveAllContractsMetaData()

	cm.lock.RLock()
	for _, contract := range activeContracts {
		blackList = append(blackList, contract.EnodeID)

		// update the addressBlackList
		if contract.Status.UploadAbility && contract.Status.RenewAbility && !contract.Status.Canceled {
			addressBlackList = append(addressBlackList, contract.EnodeID)
		}
	}
	cm.lock.RUnlock()

	// randomly retrieve some hosts
	return cm.hostManager.RetrieveRandomHosts(neededContracts*randomStorageHostsFactor+randomStorageHostsBackup, blackList, addressBlackList)
}

// ContractCreate will try to create the contract with the storage host manager provided
// by the caller
func (cm *ContractManager) ContractCreate(params storage.ContractParams) (md storage.ContractMetaData, err error) {
	allowance, funding, clientPaymentAddress, startHeight, endHeight, host := params.Allowance, params.Funding, params.ClientPaymentAddress, params.StartHeight, params.EndHeight, params.Host

	// Calculate the payouts for the client, host, and whole contract
	period := endHeight - startHeight
	expectedStorage := allowance.ExpectedStorage / allowance.StorageHosts
	clientPayout, hostPayout, _, err := ClientPayoutsPreTax(host, funding, common.BigInt0, common.BigInt0, period, expectedStorage)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	uc := types.UnlockConditions{
		PaymentAddresses: []common.Address{
			clientPaymentAddress,
			host.PaymentAddress,
		},
		SignaturesRequired: 2,
	}

	// Create storage contract
	storageContract := types.StorageContract{
		FileSize:         0,
		FileMerkleRoot:   common.Hash{}, // no proof possible without data
		WindowStart:      endHeight,
		WindowEnd:        endHeight + host.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout.BigIntPtr()}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout.BigIntPtr()}},
		UnlockHash:       uc.UnlockHash(),
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout.BigIntPtr(), Address: clientPaymentAddress},
			// Deposit is returned to host
			{Value: hostPayout.BigIntPtr(), Address: host.PaymentAddress},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout.BigIntPtr(), Address: clientPaymentAddress},
			{Value: hostPayout.BigIntPtr(), Address: host.PaymentAddress},
		},
	}

	// Increase Successful/Failed interactions accordingly
	defer func() {

		hostID := PubkeyToEnodeID(&host.NodePubKey)
		if err != nil {
			cm.hostManager.IncrementFailedInteractions(hostID)
			err = common.ErrExtend(err, ErrHostFault)
		} else {
			cm.hostManager.IncrementSuccessfulInteractions(hostID)
		}
	}()

	//Find the wallet based on the account address
	account := accounts.Account{Address: clientPaymentAddress}
	wallet, err := cm.b.AccountManager().Find(account)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("find client account error", err)
	}

	session, err := cm.b.SetupConnection(host.EnodeURL)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("setup connection with host failed", err)
	}
	defer cm.b.Disconnect(session, host.EnodeURL)

	//Sign the hash of the storage contract
	clientContractSign, err := wallet.SignHash(account, storageContract.RLPHash().Bytes())
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("contract sign by client failed", err)
	}

	// Send the ContractCreate request
	req := storage.ContractCreateRequest{
		StorageContract: storageContract,
		Sign:            clientContractSign,
		Renew:           false,
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
		UnlockConditions:      uc,
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

	if _, err := cm.b.SendStorageContractCreateTx(clientPaymentAddress, scBytes); err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("Send storage contract creation transaction error", err)
	}

	// wrap some information about this contract
	header := contractset.ContractHeader{
		ID:                     storage.ContractID(storageContract.ID()),
		EnodeID:                PubkeyToEnodeID(&host.NodePubKey),
		StartHeight:            startHeight,
		TotalCost:              funding,
		ContractFee:            host.ContractPrice,
		LatestContractRevision: storageContractRevision,
		Status: storage.ContractStatus{
			UploadAbility: true,
			RenewAbility:  true,
		},
	}

	// store this contract info to client local
	meta, err := cm.GetStorageContractSet().InsertContract(header, nil)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	return meta, nil

}
