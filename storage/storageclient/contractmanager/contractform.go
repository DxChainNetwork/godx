// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/proto"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func (cm *ContractManager) prepareFormContract(neededContracts int, clientRemainingFund common.BigInt) (terminated bool, err error) {
	// get some random hosts for contract formation
	randomHosts, err := cm.randomHostsForContractForm(neededContracts)
	if err != nil {
		return
	}

	cm.lock.RLock()
	contractFund := cm.rentPayment.Fund.DivUint64(cm.rentPayment.StorageHosts).DivUint64(3)
	contractEndHeight := cm.currentPeriod + cm.rentPayment.Period + cm.rentPayment.RenewWindow
	cm.lock.RUnlock()

	// loop through each host and try to form contract with them
	for _, host := range randomHosts {
		// check if the client has enough fund for forming contract
		if contractFund.Cmp(clientRemainingFund) > 0 {
			err = fmt.Errorf("the contract fund %v is larger than client remaining fund %v. Impossible to form contract",
				contractFund, clientRemainingFund)
			return
		}

		// start to form contract
		formCost, contract, errFormContract := cm.formContract(host, contractFund, contractEndHeight)
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

func (cm *ContractManager) formContract(host storage.HostInfo, contractFund common.BigInt, contractEndHeight uint64) (formCost common.BigInt, contract storage.ContractMetaData, err error) {
	// TODO (mzhang): formContract
	return
}

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

func (cm *ContractManager) ContractCreate(params proto.ContractParams) (storage.ContractMetaData, error) {
	// Extract vars from params, for convenience
	allowance, funding, clientPublicKey, startHeight, endHeight, host := params.Allowance, params.Funding, params.ClientPublicKey, params.StartHeight, params.EndHeight, params.Host

	// Calculate the payouts for the client, host, and whole contract
	period := endHeight - startHeight
	expectedStorage := allowance.ExpectedStorage / allowance.Hosts
	clientPayout, hostPayout, _, err := ClientPayoutsPreTax(host, funding, zeroValue, zeroValue, period, expectedStorage)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	uc := types.UnlockConditions{
		PublicKeys: []ecdsa.PublicKey{
			clientPublicKey,
			host.PublicKey,
		},
		SignaturesRequired: 2,
	}

	clientAddr := crypto.PubkeyToAddress(clientPublicKey)
	hostAddr := crypto.PubkeyToAddress(host.PublicKey)

	// Create storage contract
	storageContract := types.StorageContract{
		FileSize:         0,
		FileMerkleRoot:   common.Hash{}, // no proof possible without data
		WindowStart:      endHeight,
		WindowEnd:        endHeight + host.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout}},
		UnlockHash:       uc.UnlockHash(),
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout, Address: clientAddr},
			// Deposit is returned to host
			{Value: hostPayout, Address: hostAddr},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout, Address: clientAddr},
			{Value: hostPayout, Address: hostAddr},
		},
	}

	// Increase Successful/Failed interactions accordingly
	defer func() {

		hostID := PubkeyToEnodeID(&host.PublicKey)
		if err != nil {
			cm.hostManager.IncrementFailedInteractions(hostID)
		} else {
			cm.hostManager.IncrementSuccessfulInteractions(hostID)
		}
	}()

	account := accounts.Account{Address: clientAddr}
	wallet, err := cm.b.AccountManager().Find(account)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("find client account error", err)
	}

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

	// store this contract info to client local
	meta, errInsert := cm.GetStorageContractSet().InsertContract(header, nil)
	if errInsert != nil {
		return storage.ContractMetaData{}, errInsert
	}

	return meta, nil

}
