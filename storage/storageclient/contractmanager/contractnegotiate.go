// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/p2p/enode"

	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"

	"github.com/DxChainNetwork/godx/rlp"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
)

func (cm *ContractManager) ContractCreateNegotiate(params storage.ContractParams) (storage.ContractMetaData, error) {
	hostInfo, funding, paymentAddress := params.Host, params.Funding, params.ClientPaymentAddress

	// form unlock condition
	uc := formUnlockCondition(paymentAddress, hostInfo.PaymentAddress)

	// draft the storage contract
	storageContract, err := draftStorageContract(hostInfo, params.RentPayment, funding, params.StartHeight, params.EndHeight, paymentAddress, uc)
	if err != nil {
		return storage.ContractMetaData{}, fmt.Errorf("contract create negotiation failed: %s", err.Error())
	}

	// find the wallet based on the account address
	account := accounts.Account{Address: paymentAddress}
	wallet, err := cm.b.AccountManager().Find(account)
	if err != nil {
		cm.log.Error("contract create negotiation failed: failed to find the account address", "err", err.Error(), "address", paymentAddress)
		return storage.ContractMetaData{}, fmt.Errorf("contract create negotiation failed, failed to find the account address: %s", err.Error())
	}

	// set up the connection
	// NOTE: after set up the connection, all errors will be returned as the original error message
	sp, err := cm.b.SetupConnection(hostInfo.EnodeURL)
	if err != nil {
		cm.log.Error("contract create negotiation failed: failed to set up the connection", "err", err.Error())
		return storage.ContractMetaData{}, fmt.Errorf("contract create negotiation failed: %s", err.Error())
	}

	// TODO (mzhang): defer handle the negotiation error

	// draft storage contract negotiation
	if storageContract, err = draftStorageContractNegotiate(sp, account, wallet, storageContract); err != nil {
		cm.log.Error("contract create negotiation failed: failed to negotiate the drafted storage contract", "err", err.Error())
		return storage.ContractMetaData{}, err
	}

	// storage contract revision negotiate
	storageContractRevision, err := storageContractRevisionNegotiate(sp, storageContract, uc, account, wallet)
	if err != nil {
		cm.log.Error("contract create negotiation failed: failed to negotiate the storage contract revision", "err", err.Error())
		return storage.ContractMetaData{}, err
	}

	// send the storage contract create transaction
	if err := sendStorageContractCreateTx(storageContract, paymentAddress, cm.b); err != nil {
		cm.log.Error("contract create negotiation failed: failed to send the storage contract create transaction", "err", err.Error())
		return storage.ContractMetaData{}, err
	}

	// commit the contract information, send success message to host, and handle host's response
	return cm.clientStorageContractCommit(sp, hostInfo.EnodeID, params.StartHeight, params.Funding, hostInfo.ContractPrice, storageContract.ID(), storageContractRevision)

}

// clientNegotiateCommit will form and save the contract information persistently
// If the information is commit successfully, the success message will be sent to the
// storage host. Client will also wait and handle host's response.
// 1. form contract header
// 2. save the header information locally
// 3. send success message and handle the response from the storage host
func (cm *ContractManager) clientStorageContractCommit(sp storage.Peer, enodeID enode.ID, startHeight uint64, funding common.BigInt, contractPrice common.BigInt, contractID common.Hash, contractRevision types.StorageContractRevision) (storage.ContractMetaData, error) {
	// 1. form the contract header
	header := contractset.ContractHeader{
		ID:                     storage.ContractID(contractID),
		EnodeID:                enodeID,
		StartHeight:            startHeight,
		TotalCost:              funding,
		ContractFee:            contractPrice,
		LatestContractRevision: contractRevision,
		Status: storage.ContractStatus{
			UploadAbility: true,
			RenewAbility:  true,
		},
	}

	// 2. save the header information locally
	meta, err := cm.GetStorageContractSet().InsertContract(header, nil)
	if err != nil {
		return storage.ContractMetaData{}, common.ErrCompose(err, storage.ErrClientCommit)
	}

	// 3. send the success message and handle the response sent from the storage host
	if err := sendSuccessMsgAndHandleResp(sp, cm.GetStorageContractSet(), header.ID); err != nil {
		return storage.ContractMetaData{}, err
	}

	return meta, nil
}

func sendSuccessMsgAndHandleResp(sp storage.Peer, contractSet *contractset.StorageContractSet, contractID storage.ContractID) error {
	// send the storage client commit success
	_ = sp.SendClientCommitSuccessMsg()

	// wait and handle the response. If error, roll back the contract set
	msg, err := sp.ClientWaitContractResp()
	if err != nil {
		_ = rollbackContractSet(contractSet, contractID)
		return fmt.Errorf("after the client commit success message was sent, failed to get response from the host: %s", err.Error())
	}

	// handle the msg based on its' code
	switch msg.Code {
	case storage.HostAckMsg:
		return nil
	default:
		_ = rollbackContractSet(contractSet, contractID)
		return storage.ErrHostCommit
	}
}

// draftStorageContract will draft a storage contract based on the information provided
func draftStorageContract(hostInfo storage.HostInfo, rentPayment storage.RentPayment, funding common.BigInt, startHeight uint64, endHeight uint64, paymentAddress common.Address, uc types.UnlockConditions) (types.StorageContract, error) {
	// calculate the client and host payouts
	baseDeposit := common.BigInt0
	basePrice := common.BigInt0
	clientPayout, hostPayout, err := calculatePayouts(hostInfo.ContractPrice, funding, basePrice, hostInfo.StoragePrice, hostInfo.Deposit, hostInfo.MaxDeposit, baseDeposit, startHeight, endHeight, rentPayment)
	if err != nil {
		err = fmt.Errorf("failed to draft the storage contract: %s", err.Error())
		return types.StorageContract{}, err
	}

	// draft the storage contract
	storageContract := types.StorageContract{
		FileSize:         0,
		FileMerkleRoot:   common.Hash{}, // no proof possible without data
		WindowStart:      endHeight,
		WindowEnd:        endHeight + hostInfo.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout.BigIntPtr(), Address: paymentAddress}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout.BigIntPtr(), Address: hostInfo.PaymentAddress}},
		UnlockHash:       uc.UnlockHash(),
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout.BigIntPtr(), Address: paymentAddress},
			// Deposit is returned to host
			{Value: hostPayout.BigIntPtr(), Address: hostInfo.PaymentAddress},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout.BigIntPtr(), Address: paymentAddress},
			{Value: hostPayout.BigIntPtr(), Address: hostInfo.PaymentAddress},
		},
	}

	return storageContract, nil
}

// draftStorageContractNegotiate will negotiate the storage contract drafted by storage client with
// storage host
func draftStorageContractNegotiate(sp storage.Peer, account accounts.Account, wallet accounts.Wallet, storageContract types.StorageContract) (types.StorageContract, error) {
	// client sign the storage contract
	clientSignedContract, err := signedClientContract(wallet, account, storageContract.RLPHash().Bytes())
	if err != nil {
		return types.StorageContract{}, err
	}

	// send contract create request
	if err := formAndSendContractCreateRequest(sp, storageContract, clientSignedContract); err != nil {
		return types.StorageContract{}, err
	}

	// wait and handle the storage host response
	hostSignedContract, err := waitAndHandleHostResp(sp)
	if err != nil {
		return types.StorageContract{}, err
	}

	// update the storage contract and return
	storageContract.Signatures = [][]byte{clientSignedContract, hostSignedContract}
	return storageContract, nil
}

// storageContractRevisionNegotiate will form the storage contract revision, send it to host for verification
// then the updated storage contract revision will be returned
func storageContractRevisionNegotiate(sp storage.Peer, storageContract types.StorageContract, uc types.UnlockConditions, account accounts.Account, wallet accounts.Wallet) (types.StorageContractRevision, error) {
	// form the storage contract revision
	storageContractRevision, err := formStorageContractRevision(storageContract, uc, account, wallet)
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// send the signed revision to storage host, if the storage client
	// failed to send the contract create revision sign, then it is client's fault
	if err := sp.SendContractCreateClientRevisionSign(storageContractRevision.Signatures[0]); err != nil {
		err = common.ErrCompose(err, storage.ErrClientNegotiate)
		return types.StorageContractRevision{}, err
	}

	// wait for host's response
	hostRevisionSign, err := waitAndHandleHostResp(sp)
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// update and return storage contract revision
	storageContractRevision.Signatures = append(storageContractRevision.Signatures, hostRevisionSign)
	return storageContractRevision, nil
}

// sendStorageContractCreateTx will encode the storage contract and send it as the transaction
// error belongs to storage client negotiate error
func sendStorageContractCreateTx(storageContract types.StorageContract, clientPaymentAddress common.Address, b storage.ClientBackend) error {
	// rlp encode the storage contract
	scEncode, err := rlp.EncodeToBytes(storageContract)
	if err != nil {
		return common.ErrCompose(err, storage.ErrClientNegotiate)
	}

	// send the transaction, if error, it should be classified as
	// client negotiate error
	if _, err := b.SendStorageContractCreateTx(clientPaymentAddress, scEncode); err != nil {
		return common.ErrCompose(err, storage.ErrClientNegotiate)
	}

	return nil
}

// formStorageContractRevision will form the storage contract revision. Moreover, the client
// will sign the revision as well
func formStorageContractRevision(storageContract types.StorageContract, uc types.UnlockConditions, account accounts.Account, wallet accounts.Wallet) (types.StorageContractRevision, error) {
	// form the storage contract revision
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

	// client sign the storage contract revision
	clientSignedRevision, err := signedClientContract(wallet, account, storageContractRevision.RLPHash().Bytes())
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// update and return the storage contract revision
	storageContractRevision.Signatures = append(storageContractRevision.Signatures, clientSignedRevision)
	return storageContractRevision, nil
}

// formAndSendContractCreateRequest will form the contract create request and send it
// to the storage host
func formAndSendContractCreateRequest(sp storage.Peer, storageContract types.StorageContract, clientSignedContract []byte) error {
	// form the contract create request
	req := storage.ContractCreateRequest{
		StorageContract: storageContract,
		Sign:            clientSignedContract,
		Renew:           false,
	}

	// send contract creation request to storage host
	if err := sp.RequestContractCreation(req); err != nil {
		return fmt.Errorf("failed to send the contract creation request: %s", err.Error())
	}

	return nil
}

// waitAndHandleHostResp will wait the host response from the storage host
// check the response and handle it accordingly
func waitAndHandleHostResp(sp storage.Peer) (hostSign []byte, err error) {
	// wait until the message was sent by the storage host
	msg, err := sp.ClientWaitContractResp()
	if err != nil {
		err = fmt.Errorf("contract create read message error: %s", err.Error())
		return
	}

	// check error message code
	if msg.Code == storage.HostBusyHandleReqMsg {
		err = storage.ErrHostBusyHandleReq
		return
	}

	if msg.Code == storage.HostNegotiateErrorMsg {
		err = storage.ErrHostNegotiate
		return
	}

	// decode the message from the storage host
	if err = msg.Decode(&hostSign); err != nil {
		err = common.ErrCompose(storage.ErrHostNegotiate, err)
		return
	}

	return
}

// signedClientContract will create signed version of the storage contract by storage client
// Error belongs to client negotiate error
func signedClientContract(wallet accounts.Wallet, account accounts.Account, storageContractHash []byte) ([]byte, error) {
	// storage client sign the storage contract
	signedContract, err := wallet.SignHash(account, storageContractHash)
	if err != nil {
		err = fmt.Errorf("failed to sign the storage contract: %s", err.Error())
		return []byte{}, common.ErrCompose(err, storage.ErrClientNegotiate)
	}

	return signedContract, nil
}

// formUnlockCondition will create unlock condition for drafted storage contract and
// storage contract revision
func formUnlockCondition(clientPaymentAddress common.Address, hostPaymentAddress common.Address) types.UnlockConditions {
	uc := types.UnlockConditions{
		PaymentAddresses: []common.Address{
			clientPaymentAddress,
			hostPaymentAddress,
		},
		SignaturesRequired: 2,
	}
	return uc
}

// rollbackContractSet will delete saved contract information based on the
// contractID
func rollbackContractSet(contractSet *contractset.StorageContractSet, id storage.ContractID) error {
	if c, exist := contractSet.Acquire(id); exist {
		if err := contractSet.Delete(c); err != nil {
			return err
		}
	}
	return nil
}
