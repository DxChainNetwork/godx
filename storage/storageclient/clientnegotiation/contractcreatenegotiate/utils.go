// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiate

import (
	"fmt"
	"reflect"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/clientnegotiation"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	dberrors "github.com/syndtr/goleveldb/leveldb/errors"
)

// draftStorageContractNegotiate will negotiate the storage contract drafted by storage client with
// storage host
func draftStorageContractNegotiate(sp storage.Peer, account accounts.Account, wallet accounts.Wallet, storageContract types.StorageContract, revision types.StorageContractRevision) (types.StorageContract, error) {
	// client sign the storage contract
	clientSignedContract, err := signedClientContract(wallet, account, storageContract.RLPHash().Bytes())
	if err != nil {
		return types.StorageContract{}, err
	}

	// send contract create request
	if err := formAndSendContractCreateRequest(sp, storageContract, clientSignedContract, revision); err != nil {
		return types.StorageContract{}, err
	}

	// wait and handle the storage host response
	hostSignedContract, err := waitAndHandleHostSignResp(sp)
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
	hostRevisionSign, err := waitAndHandleHostSignResp(sp)
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// update and return storage contract revision
	storageContractRevision.Signatures = append(storageContractRevision.Signatures, hostRevisionSign)
	return storageContractRevision, nil
}

// sendStorageContractCreateTx will encode the storage contract and send it as the transaction
// error belongs to storage client negotiate error
// Error belongs to clientNegotiationError
func sendStorageContractCreateTx(storageContract types.StorageContract, clientPaymentAddress common.Address, cp clientnegotiation.ContractCreateProtocol) error {
	// rlp encode the storage contract
	scEncode, err := rlp.EncodeToBytes(storageContract)
	if err != nil {
		return common.ErrCompose(err, storage.ErrClientNegotiate)
	}

	// send the transaction, if error, it should be classified as
	// client negotiate error
	if _, err := cp.SendStorageContractCreateTx(clientPaymentAddress, scEncode); err != nil {
		return common.ErrCompose(err, storage.ErrClientNegotiate)
	}

	return nil
}

// clientNegotiateCommit will form and save the contract information persistently
// If the information is commit successfully, the success message will be sent to the
// storage host. Client will also wait and handle host's response.
// 1. form contract header
// 2. if the oldContract is not nil, meaning the merkle roots can be acquired from the old contract
// 3. save the header information locally
// 4. send success message and handle the response from the storage host
func clientStorageContractCommit(cp clientnegotiation.ContractCreateProtocol, sp storage.Peer, enodeID enode.ID, startHeight uint64, funding common.BigInt, contractPrice common.BigInt, contractID common.Hash, contractRevision types.StorageContractRevision, oldContract *contractset.Contract) (storage.ContractMetaData, error) {
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

	// 2. if the oldContract is not nil, meaning the merkle roots
	// can be acquired from the old contract
	var oldRoots []common.Hash = nil
	var err error
	if oldContract != nil {
		oldRoots, err = oldContract.MerkleRoots()
		if err != nil && err != dberrors.ErrNotFound {
			err = common.ErrCompose(err, storage.ErrClientNegotiate)
			return storage.ContractMetaData{}, err
		} else if err == dberrors.ErrNotFound {
			oldRoots = []common.Hash{}
		}
	}

	// 3. save the header information locally
	meta, err := cp.InsertContract(header, oldRoots)
	if err != nil {
		return storage.ContractMetaData{}, common.ErrCompose(err, storage.ErrClientCommit)
	}

	// 4. send the success message and handle the response sent from the storage host
	if err := sendSuccessMsgAndHandleResp(sp, cp.GetStorageContractSet(), header.ID); err != nil {
		return storage.ContractMetaData{}, err
	}
	return meta, nil
}

// Special Types Error:
// 1. ErrClientNegotiate   ->  send negotiation failed message, wait response
// 2. ErrClientCommit      ->  send commit failed message, wait response
// 3. ErrHostCommit		   ->  sendACK, wait response, punish host, check and update the connection
// 4. ErrHostNegotiate     ->  punish host, check and update the connection
func handleNegotiationErr(np clientnegotiation.NegotiationError, err error, hostID enode.ID, sp storage.Peer, it storagehostmanager.InteractionType) {
	// if no error, reward the host and return directly
	if err == nil {
		fmt.Println("NO ERRORRRRR")
		np.IncrementSuccessfulInteractions(hostID, it)
		return
	}

	fmt.Println(err)

	// otherwise, based on the error type, handle it differently
	switch {
	case common.ErrContains(err, storage.ErrClientNegotiate):
		_ = sp.SendClientNegotiateErrorMsg()
	case common.ErrContains(err, storage.ErrClientCommit):
		_ = sp.SendClientCommitFailedMsg()
	case common.ErrContains(err, storage.ErrHostNegotiate):
		np.IncrementFailedInteractions(hostID, it)
		np.CheckAndUpdateConnection(sp.PeerNode())
		return
	case common.ErrContains(err, storage.ErrHostCommit):
		_ = sp.SendClientAckMsg()
		np.IncrementFailedInteractions(hostID, it)
		np.CheckAndUpdateConnection(sp.PeerNode())
		_ = sp.SendClientAckMsg()
	default:
		return
	}

	// wait until host sent back ACK message
	if msg, respErr := sp.ClientWaitContractResp(); respErr != nil || msg.Code != storage.HostAckMsg {
		log.Error("handleNegotiateErr error", "type", err, "err", respErr, "msgCode", msg.Code)
	}
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
func formAndSendContractCreateRequest(sp storage.Peer, storageContract types.StorageContract, clientSignedContract []byte, revision types.StorageContractRevision) error {
	// form the contract create request. If the contract revision passed in is empty, meaning
	// the request is not the renew request. Otherwise the request is renew request
	var req storage.ContractCreateRequest
	if reflect.DeepEqual(revision, types.StorageContractRevision{}) {
		req = storage.ContractCreateRequest{
			StorageContract: storageContract,
			Sign:            clientSignedContract,
			Renew:           false,
		}
	} else {
		req = storage.ContractCreateRequest{
			StorageContract: storageContract,
			Sign:            clientSignedContract,
			Renew:           true,
			OldContractID:   revision.ParentID,
		}
	}

	// send contract creation request to storage host
	if err := sp.RequestContractCreation(req); err != nil {
		return fmt.Errorf("failed to send the contract creation request: %s", err.Error())
	}

	return nil
}

// waitAndHandleHostSignResp will wait the host response from the storage host
// check the response and handle it accordingly
// Error belongs to hostNegotiationError
func waitAndHandleHostSignResp(sp storage.Peer) (hostSign []byte, err error) {
	// wait until the message was sent by the storage host
	msg, err := sp.ClientWaitContractResp()
	if err != nil {
		err = fmt.Errorf("contract create read message error: %s", err.Error())
		return
	}

	// check error message code
	if err = hostRespMsgCodeValidation(msg); err != nil {
		return
	}

	// decode the message from the storage host
	if err = msg.Decode(&hostSign); err != nil {
		err = common.ErrCompose(storage.ErrHostNegotiate, err)
		return
	}

	return
}

// hostRespMsgCodeValidation will validate the host response message code
func hostRespMsgCodeValidation(msg p2p.Msg) error {
	// check if the msg code is HostBusyHandleReqMsg
	if msg.Code == storage.HostBusyHandleReqMsg {
		return storage.ErrHostBusyHandleReq
	}

	// check if the msg code is negotiation error
	if msg.Code == storage.HostNegotiateErrorMsg {
		return storage.ErrHostNegotiate
	}

	return nil
}

// signedClientContract will create signed version of the storage contract by storage client
// Error belongs to clientNegotiationError
func signedClientContract(wallet accounts.Wallet, account accounts.Account, storageContractHash []byte) ([]byte, error) {
	// storage client sign the storage contract
	signedContract, err := wallet.SignHash(account, storageContractHash)
	if err != nil {
		err = fmt.Errorf("failed to sign the storage contract: %s", err.Error())
		return []byte{}, common.ErrCompose(err, storage.ErrClientNegotiate)
	}

	return signedContract, nil
}

// sendSuccessMsgAndHandleResp will send commit success message to storage host
// and wait for host's response
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
