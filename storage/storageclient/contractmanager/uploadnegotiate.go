// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"

	"github.com/DxChainNetwork/godx/crypto/merkle"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/common"

	"github.com/DxChainNetwork/godx/p2p/enode"

	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"

	"github.com/DxChainNetwork/godx/storage"
)

func (cm *ContractManager) UploadNegotiate(sp storage.Peer, actions []storage.UploadAction, hostInfo storage.HostInfo) (negotiateErr error) {
	// get the contract based on hostID, return at the end
	contract, err := cm.getContractBasedOnHostID(hostInfo.EnodeID)
	if err != nil {
		cm.log.Error("upload negotiation failed, failed to get the contract", "err", err.Error())
		return fmt.Errorf("upload neogtiation failed, failed to get the contract: %s", err.Error())
	}

	// return the contract at the end
	defer cm.contractReturn(contract)

	// form the new upload contract revision
	currentBlockHeight := cm.GetBlockHeight()
	latestContractRevision := contract.Header().LatestContractRevision
	bandwidthPrice, storagePrice, deposit, newFileSize := calculatePricesAndNewFileSize(latestContractRevision, currentBlockHeight, hostInfo, actions)
	uploadRevision, err := formUploadContractRevision(hostInfo, latestContractRevision, bandwidthPrice, storagePrice, deposit, newFileSize)
	if err != nil {
		cm.log.Error("Client upload negotiation failed, failed to create upload contract revision", "err", err.Error())
		negotiateErr = err
		return
	}

	// handle the negotiation error
	defer cm.handleNegotiationErr(&negotiateErr, hostInfo.EnodeID, sp)

	// form the upload request and start upload request negotiation
	uploadMerkleProof, err := uploadRequestNegotiation(sp, latestContractRevision.ParentID, uploadRevision, actions)
	if err != nil {
		cm.log.Error("Client upload negotiation failed, failed to negotiate upload request", "err", err.Error())
		negotiateErr = err
		return
	}

	// verify merkle proof and form new merkle root
	uploadRevision, err = verifyAndUpdateMerkleRoot(latestContractRevision, actions, uploadMerkleProof, uploadRevision)
	if err != nil {
		cm.log.Error("Client upload negotiation failed, failed to verify the merkle root", "err", err.Error())
		negotiateErr = err
		return
	}

	// client sign the upload revision and negotiate the signed revision
	uploadRevision, err = uploadContractRevisionNegotiation(sp, uploadRevision, cm.b.AccountManager())
	if err != nil {
		cm.log.Error("Client upload negotiation failed, failed to negotiate uploadContractRevision", "err", err.Error())
		negotiateErr = err
		return
	}

	// storage client commit the storage contract revision
	negotiateErr = storageContractRevisionCommit(sp, uploadRevision, contract, storagePrice, bandwidthPrice)
	return
}

// getContractBasedOnHostID will return the contract information based on provided hostID
func (cm *ContractManager) getContractBasedOnHostID(hostID enode.ID) (*contractset.Contract, error) {
	// get the contractID based on the hostID
	contractID, exist := cm.activeContracts.GetContractIDByHostID(hostID)
	if !exist {
		return nil, fmt.Errorf("contract does not exist based on the hostID provided")
	}

	// get the contract based on the contractID
	contract, exist := cm.activeContracts.Acquire(contractID)
	if !exist {
		return nil, fmt.Errorf("contract does not exist based on the contractID provided")
	}

	return contract, nil
}

// formUploadContractRevision will calculate the necessary prices and form a new upload contract revision
func formUploadContractRevision(hostInfo storage.HostInfo, latestContractRevision types.StorageContractRevision, bandwidthPrice common.BigInt, storagePrice common.BigInt, deposit common.BigInt, newFileSize uint64) (types.StorageContractRevision, error) {
	// estimate the cost and validate the contract revision payback
	cost := bandwidthPrice.Add(storagePrice).Add(hostInfo.BaseRPCPrice)
	if err := contractRevisionPaybackValidation(latestContractRevision, cost, deposit); err != nil {
		return types.StorageContractRevision{}, err
	}

	// form and update the new contract revision
	uploadContractRevision := newContractRevision(latestContractRevision, cost.BigIntPtr())
	uploadContractRevision.NewMissedProofOutputs[1].Value = uploadContractRevision.NewMissedProofOutputs[1].Value.Sub(uploadContractRevision.NewMissedProofOutputs[1].Value, deposit.BigIntPtr())
	uploadContractRevision.NewFileSize = newFileSize

	return uploadContractRevision, nil
}

// uploadRequestNegotiation will negotiate the upload request drafted by the storage client
// if everything works as expected, the merkleProof will be returned by storage host
func uploadRequestNegotiation(sp storage.Peer, contractID common.Hash, uploadRevision types.StorageContractRevision, uploadActions []storage.UploadAction) (storage.UploadMerkleProof, error) {
	// form the upload request
	uploadReq := formUploadRequest(contractID, uploadRevision, uploadActions)

	// send the contract upload request
	if err := sp.RequestContractUpload(uploadReq); err != nil {
		return storage.UploadMerkleProof{}, err
	}

	// wait and parse the upload merkle proof response from the storage host
	return waitAndParseUploadMerkleProofResp(sp)
}

// uploadContractRevisionNegotiation will sign the uploadContractRevision by storage client
// send it to storage host, wait and get the host signed storage contract revision
func uploadContractRevisionNegotiation(sp storage.Peer, uploadRevision types.StorageContractRevision, am storage.ClientAccountManager) (types.StorageContractRevision, error) {
	// get the client revision sign
	clientRevisionSign, err := clientSignUploadContractRevision(uploadRevision, am)
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// send the signed contract to storage host
	if err := sp.SendContractUploadClientRevisionSign(clientRevisionSign); err != nil {
		err = fmt.Errorf("client failed to send contract upload client revision sign: %s", err.Error())
		return types.StorageContractRevision{}, common.ErrCompose(storage.ErrClientNegotiate, err)
	}

	// get the storage host signed revision
	hostRevisionSign, err := waitAndHandleHostSignResp(sp)
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// update the contract revision
	uploadRevision.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}
	return uploadRevision, nil
}

// storageContractRevisionCommit will commit the storage upload contract revision
func storageContractRevisionCommit(sp storage.Peer, uploadRevision types.StorageContractRevision, contract *contractset.Contract, storagePrice, bandwidthPrice common.BigInt) error {
	// commit the upload contract revision
	unCommitContractHeader := contract.Header()
	if err := contract.CommitRevision(uploadRevision, storagePrice, bandwidthPrice); err != nil {
		err = fmt.Errorf("client failed to commit the upload contract revision: %s", err.Error())
		return common.ErrCompose(storage.ErrClientCommit, err)
	}
	return sendAndHandleClientCommitSuccessMsg(sp, contract, unCommitContractHeader)

}

// sendAndHandleClientCommitSuccessMsg will send the message to storage host which indicates that
// storageClient has successfully committed the upload storage revision
func sendAndHandleClientCommitSuccessMsg(sp storage.Peer, contract *contractset.Contract, contractHeader contractset.ContractHeader) error {
	// client send the commit success message
	_ = sp.SendClientCommitSuccessMsg()

	// wait for host acknowledgement message until timeout
	msg, err := sp.ClientWaitContractResp()
	if err != nil {
		_ = contract.RollbackUndoMem(contractHeader)
		return fmt.Errorf("after client commit success message was sent, failed to get message from host: %s", err.Error())
	}

	// handle the message based on its' code
	switch msg.Code {
	case storage.HostAckMsg:
		return nil
	default:
		_ = contract.RollbackUndoMem(contractHeader)
		return storage.ErrHostCommit
	}
}

// clientSignUploadContractRevision will sign the upload storage contract revision by storage client
func clientSignUploadContractRevision(uploadRevision types.StorageContractRevision, am storage.ClientAccountManager) ([]byte, error) {
	// get the storage client's account and wallet
	clientPaymentAddress := uploadRevision.NewValidProofOutputs[0].Address
	clientAccount := accounts.Account{Address: clientPaymentAddress}
	clientWallet, err := am.Find(clientAccount)
	if err != nil {
		err = fmt.Errorf("failed to get the client wallet: %s", err.Error())
		return []byte{}, common.ErrCompose(storage.ErrClientNegotiate, err)
	}

	// sign the uploadContractRevision
	clientRevisionSign, err := clientWallet.SignHash(clientAccount, uploadRevision.RLPHash().Bytes())
	if err != nil {
		err = fmt.Errorf("client failed to sign the upload contract revision: %s", err.Error())
		return []byte{}, common.ErrCompose(storage.ErrClientNegotiate, err)
	}

	return clientRevisionSign, nil
}

// verifyAndUpdateMerkleRoot will verify both old and new merkle root and update the uploadRevision
func verifyAndUpdateMerkleRoot(latestRevision types.StorageContractRevision, uploadActions []storage.UploadAction, uploadMerkleProof storage.UploadMerkleProof, uploadRevision types.StorageContractRevision) (types.StorageContractRevision, error) {
	// verify the merkle proof with the old root
	numSectors := latestRevision.NewFileSize / storage.SectorSize
	proofRanges := calculateProofRanges(uploadActions, numSectors)
	proofHashes, leafHashes := uploadMerkleProof.OldSubtreeHashes, uploadMerkleProof.OldLeafHashes
	oldRoot := latestRevision.NewFileMerkleRoot
	if err := merkle.Sha256VerifyDiffProof(proofRanges, numSectors, proofHashes, leafHashes, oldRoot); err != nil {
		err = fmt.Errorf("failed to verify the merkle proof with the old root: %s", err.Error())
		return types.StorageContractRevision{}, common.ErrCompose(storage.ErrHostNegotiate, err)
	}

	// modify the leaves and verify the new root
	leafHashes = modifyLeaves(leafHashes, uploadActions, numSectors)
	proofRanges = modifyProofRanges(proofRanges, uploadActions, numSectors)
	newRoot := uploadMerkleProof.NewMerkleRoot
	if err := merkle.Sha256VerifyDiffProof(proofRanges, numSectors, proofHashes, leafHashes, newRoot); err != nil {
		err = fmt.Errorf("failed to verify the merkle proof with the new root: %s", err.Error())
		return types.StorageContractRevision{}, common.ErrCompose(storage.ErrHostNegotiate, err)
	}

	// update the uploadRevision's merkle root
	uploadRevision.NewFileMerkleRoot = newRoot
	return uploadRevision, nil
}

// waitAndParseUploadMerkleProofResp will wait the response from client getting from the storage host
// check for the error message code and decode the upload merkle proof
func waitAndParseUploadMerkleProofResp(sp storage.Peer) (storage.UploadMerkleProof, error) {
	var merkleProof storage.UploadMerkleProof
	msg, err := sp.ClientWaitContractResp()
	if err != nil {
		return storage.UploadMerkleProof{}, fmt.Errorf("failed to wait for client merkle proof response: %s", err.Error())
	}

	// check the error message code
	if msg.Code == storage.HostBusyHandleReqMsg {
		return storage.UploadMerkleProof{}, storage.ErrHostBusyHandleReq
	}

	if msg.Code == storage.HostNegotiateErrorMsg {
		return storage.UploadMerkleProof{}, storage.ErrHostNegotiate
	}

	if err := msg.Decode(&merkleProof); err != nil {
		return storage.UploadMerkleProof{}, common.ErrCompose(storage.ErrHostNegotiate, err)
	}

	return merkleProof, nil
}

// formUploadRequest will form the upload request which is used for sending upload request to storage client
func formUploadRequest(storageContractID common.Hash, uploadRevision types.StorageContractRevision, uploadActions []storage.UploadAction) storage.UploadRequest {
	// form the upload request
	uploadReq := storage.UploadRequest{
		StorageContractID: storageContractID,
		Actions:           uploadActions,
		NewRevisionNumber: uploadRevision.NewRevisionNumber,
	}

	// update the upload request with valid proof payback
	for _, payback := range uploadRevision.NewValidProofOutputs {
		uploadReq.NewValidProofValues = append(uploadReq.NewValidProofValues, payback.Value)
	}

	// update the upload request with missed proof payback
	for _, payback := range uploadRevision.NewMissedProofOutputs {
		uploadReq.NewMissedProofValues = append(uploadReq.NewMissedProofValues, payback.Value)
	}

	return uploadReq
}

// calculatePricesAndNewFileSize will calculate bandwidthPrice, deposit, and newFileSize for new upload contract revision
func calculatePricesAndNewFileSize(contractRevision types.StorageContractRevision, blockHeight uint64, hostInfo storage.HostInfo, uploadActions []storage.UploadAction) (bandwidthPrice, storagePrice, deposit common.BigInt, newFileSize uint64) {
	// calculate price per sector
	blockBytes := storage.SectorSize * uint64(contractRevision.NewWindowEnd-blockHeight)
	sectorBandwidthPrice := hostInfo.UploadBandwidthPrice.MultUint64(storage.SectorSize)
	sectorStoragePrice := hostInfo.StoragePrice.MultUint64(blockBytes)
	sectorDeposit := hostInfo.Deposit.MultUint64(blockBytes)

	// calculate the bandwidthPrice, storagePrice, and deposit based on the new file size
	newFileSize = contractRevision.NewFileSize
	for _, action := range uploadActions {
		switch action.Type {
		case storage.UploadActionAppend:
			bandwidthPrice = bandwidthPrice.Add(sectorBandwidthPrice)
		}
	}
	if newFileSize > contractRevision.NewFileSize {
		addedSectors := (newFileSize - contractRevision.NewFileSize) / storage.SectorSize
		storagePrice = sectorStoragePrice.MultUint64(addedSectors)
		deposit = sectorDeposit.MultUint64(addedSectors)
	}

	// calculate the proof size and update the bandwidthPrice
	proofSize := storage.HashSize * (128 + len(uploadActions))
	bandwidthPrice = bandwidthPrice.Add(hostInfo.DownloadBandwidthPrice.MultUint64(uint64(proofSize)))
	return
}

// contractRevisionPaybackValidation will validate the client's validProofPayback to check if the client
// has enough fund to pay the upload price if succeed. Moreover, it will also validate the host's missedProofPayback
// to check if the host has enough money to pay for the punishment
func contractRevisionPaybackValidation(contractRevision types.StorageContractRevision, cost common.BigInt, deposit common.BigInt) error {
	// check if the client has enough fund to pay upload price for valid proof
	if contractRevision.NewValidProofOutputs[0].Value.Cmp(cost.BigIntPtr()) < 0 {
		return errors.New("contract has insufficient fund")
	}

	// check if the host deposit has enough money to pay for the punishment
	if contractRevision.NewMissedProofOutputs[1].Value.Cmp(deposit.BigIntPtr()) < 0 {
		return errors.New("contract has insufficient deposit")
	}

	return nil
}

// contractReturn will return the contract back to the active contract
// list. the error will be ignored
func (cm *ContractManager) contractReturn(c *contractset.Contract) {
	_ = cm.activeContracts.Return(c)
}
