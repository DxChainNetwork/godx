// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/DxChainNetwork/godx/storage/storagehost"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/accounts"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"

	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

func ContractCreateRenewHandler(np NegotiationProtocol, sp storage.Peer, contractCreateReqMsg p2p.Msg) {
	var negotiateErr error
	var negotiationData contractNegotiation
	defer handleNegotiationErr(&negotiateErr, sp, np)

	// check if the contract is accepted, if not, make the error
	// storage host negotiation error
	if !np.IsAcceptingContract() {
		err := fmt.Errorf("storage host does not accept new storage contract")
		negotiateErr = common.ErrCompose(err, storage.ErrHostNegotiate)
		return
	}

	// decode and validate storage contract create request
	req, err := decodeAndValidateReq(np, contractCreateReqMsg, &negotiationData)
	if err != nil {
		negotiateErr = err
		return
	}

	// storage host sign and update the storage contract
	sc, err := signAndValidateContract(&negotiationData, req, np)
	if err != nil {
		negotiateErr = err
		return
	}

	// start the contract create host sign negotiation and get the client signed contract revision
	clientRevisionSign, err := contractCreateNegotiation(sp, sc)
	if err != nil {
		negotiateErr = err
		return
	}

	if err := contractRevisionNegotiation(np, sp, sc, negotiationData, clientRevisionSign, req); err != nil {
		negotiateErr = err
		return
	}
}

// decodeAndValidateReq will decode and validate the contract create request
func decodeAndValidateReq(np NegotiationProtocol, contractCreateReqMsg p2p.Msg, negotiationData *contractNegotiation) (storage.ContractCreateRequest, error) {
	// decode the contract create request, if failed, the error
	// should be categorized as client's error
	var req storage.ContractCreateRequest
	if err := contractCreateReqMsg.Decode(&req); err != nil {
		err = fmt.Errorf("failed to decode the contract create request: %s", err.Error())
		return storage.ContractCreateRequest{}, common.ErrCompose(storage.ErrClientNegotiate, err)
	}

	// parse and update the client public key
	clientPubKey, err := crypto.SigToPub(req.StorageContract.RLPHash().Bytes(), req.Sign)
	if err != nil {
		err = fmt.Errorf("failed to recover the client's public key from the signature")
		return storage.ContractCreateRequest{}, common.ErrCompose(storage.ErrClientNegotiate, err)
	}
	negotiationData.clientPubKey = clientPubKey

	// balance verification
	hostAddress := req.StorageContract.ValidProofOutputs[1].Address
	hostDeposit := req.StorageContract.HostCollateral.Value
	if err := hostBalanceValidation(np, hostAddress, hostDeposit); err != nil {
		return storage.ContractCreateRequest{}, err
	}

	// validate the host address and update the negotiationData
	if err := hostAddressValidation(hostAddress, negotiationData, np); err != nil {
		return storage.ContractCreateRequest{}, err
	}

	return req, nil
}

// signAndValidateContract will have the contract signed by the storage contract, update the contract
// and then perform a bunch of validation on the updated contract
func signAndValidateContract(negotiationData *contractNegotiation, req storage.ContractCreateRequest, np NegotiationProtocol) (types.StorageContract, error) {
	// extract data
	sc := req.StorageContract
	clientSign := req.Sign

	// storage host sign the storage contract
	hostSign, err := hostSignedContract(negotiationData.account, negotiationData.wallet, sc.RLPHash().Bytes())
	if err != nil {
		err = fmt.Errorf("failed to sign and update the contract: %s", err.Error())
		return types.StorageContract{}, err
	}

	// get the public key of the storage host
	hostPubKey, err := hostPubKeyRecovery(sc.RLPHash().Bytes(), hostSign)
	if err != nil {
		err = fmt.Errorf("failed to sign and update the contract: %s", err.Error())
		return types.StorageContract{}, err
	}

	// update the negotiationData
	negotiationData.hostPubKey = hostPubKey
	sc.Signatures = [][]byte{clientSign, hostSign}

	// validate the storage contract
	if err := contractValidation(np, req, sc, negotiationData.hostPubKey, negotiationData.clientPubKey); err != nil {
		err = fmt.Errorf("contract validation failed: %s", err.Error())
		return types.StorageContract{}, err
	}

	// return the updated and validated storage contract
	return sc, nil
}

func contractCreateNegotiation(sp storage.Peer, sc types.StorageContract) ([]byte, error) {
	// send the contract creation host sign
	hostContractSign := sc.Signatures[1]
	if err := sp.SendContractCreationHostSign(hostContractSign); err != nil {
		err = fmt.Errorf("contract create host sign neogtiation failed, failed to send contract creation host sign: %s", err.Error())
		return []byte{}, err
	}

	// wait and handle the storage client response
	return waitAndHandleClientRevSignResp(sp)
}

func contractRevisionNegotiation(np NegotiationProtocol, sp storage.Peer, sc types.StorageContract, negotiationData contractNegotiation, clientRevSign []byte, req storage.ContractCreateRequest) error {
	// construct and update the storage contract revision
	contractRev := constructContractRevision(sc, negotiationData.clientPubKey, negotiationData.hostPubKey)
	updatedContractRev, err := signAndUpdateContractRevision(contractRev, negotiationData, clientRevSign)
	if err != nil {
		return err
	}

	// start the revision negotiation
	if err := sp.SendContractCreationHostRevisionSign(updatedContractRev.Signatures[1]); err != nil {
		err = fmt.Errorf("contract reivision negotiation failed, failed to send contract creation host revision sign: %s", err.Error())
		return err
	}

	// wait and handle client contract commit response
	sr, err := waitAndHandleClientCommitResp(sp, np, req, sc, contractRev)
	if err != nil {
		return err
	}

	// update contract record
	if err := updateHostContractRecord(sp, np, sc.ID()); err != nil {
		return err
	}

	// send host host ACK message
	if err := sp.SendHostAckMsg(); err != nil {
		_ = np.RollBackStorageResponsibility(sr)
		np.RollBackConnectionType(sp)
	}

	return nil
}

func waitAndHandleClientRevSignResp(sp storage.Peer) ([]byte, error) {
	// wait for client response
	var clientRevisionSign []byte
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		err = fmt.Errorf("storage host failed to wait for client contract reivision sign: %s", err.Error())
		return []byte{}, err
	}

	// check for error message code
	if msg.Code == storage.ClientNegotiateErrorMsg {
		return []byte{}, storage.ErrClientNegotiate
	}

	// decode the message
	if err = msg.Decode(&clientRevisionSign); err != nil {
		err = fmt.Errorf("failed to decode client revision sign: %s", err.Error())
		return []byte{}, err
	}

	return clientRevisionSign, nil
}

func waitAndHandleClientCommitResp(sp storage.Peer, np NegotiationProtocol, req storage.ContractCreateRequest, sc types.StorageContract, contractRevision types.StorageContractRevision) (storagehost.StorageResponsibility, error) {
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		err = fmt.Errorf("waitAndHandleClientCommitResp failed, host failed to wait for client commit response: %s", err.Error())
		return storagehost.StorageResponsibility{}, err
	}

	if msg.Code == storage.ClientCommitFailedMsg {
		return storagehost.StorageResponsibility{}, fmt.Errorf("waitAndHandleClientCommitResp failed, client sent back commint failed message")
	}

	// client sent back commit success message
	if msg.Code == storage.ClientCommitSuccessMsg {
		return clientCommitSuccessHandle(np, req, sc, contractRevision)
	}

	// for other message code, return error directly
	return storagehost.StorageResponsibility{}, fmt.Errorf("waitAndHandleClientCommitResp failed, client sent back a negotiation error message")
}

func clientCommitSuccessHandle(np NegotiationProtocol, req storage.ContractCreateRequest, sc types.StorageContract, contractRevision types.StorageContractRevision) (storagehost.StorageResponsibility, error) {
	// get the block number
	blockNumber := np.GetBlockHeight()
	hostConfig := np.GetHostConfig()

	// construct the storage responsibility
	sr := constructStorageResponsibility(hostConfig.ContractPrice, blockNumber, sc, contractRevision)
	if req.Renew {
		sr = updateStorageResponsibility(np, req, sr, hostConfig)
	}

	// commit storage responsibility
	if err := np.FinalizeStorageResponsibility(sr); err != nil {
		err = fmt.Errorf("clientCommitSuccessHandle failed, failed to finalize the storage responsibility: %s", err.Error())
		return storagehost.StorageResponsibility{}, common.ErrCompose(err, storage.ErrHostCommit)
	}

	return sr, nil
}

func constructContractRevision(sc types.StorageContract, clientPubKey, hostPubKey *ecdsa.PublicKey) types.StorageContractRevision {
	return types.StorageContractRevision{
		ParentID: sc.RLPHash(),
		UnlockConditions: types.UnlockConditions{
			PaymentAddresses: []common.Address{
				crypto.PubkeyToAddress(*clientPubKey),
				crypto.PubkeyToAddress(*hostPubKey),
			},
			SignaturesRequired: contractRequiredSignatures,
		},
		NewRevisionNumber:     1,
		NewFileSize:           sc.FileSize,
		NewFileMerkleRoot:     sc.FileMerkleRoot,
		NewWindowStart:        sc.WindowStart,
		NewWindowEnd:          sc.WindowEnd,
		NewValidProofOutputs:  sc.ValidProofOutputs,
		NewMissedProofOutputs: sc.MissedProofOutputs,
		NewUnlockHash:         sc.UnlockHash,
	}
}

func constructStorageResponsibility(contractPrice common.BigInt, blockNumber uint64, sc types.StorageContract, contractRevision types.StorageContractRevision) storagehost.StorageResponsibility {
	// prepare the needed data
	hostValidPayout := common.PtrBigInt(sc.ValidProofOutputs[validProofPaybackHostAddressIndex].Value)
	lockedStorageDeposit := hostValidPayout.Sub(contractPrice)

	// construct and return the storage responsibility
	return storagehost.StorageResponsibility{
		SectorRoots:              nil,
		ContractCost:             contractPrice,
		LockedStorageDeposit:     lockedStorageDeposit,
		PotentialStorageRevenue:  common.BigInt0,
		RiskedStorageDeposit:     common.BigInt0,
		NegotiationBlockNumber:   blockNumber,
		OriginStorageContract:    sc,
		StorageContractRevisions: []types.StorageContractRevision{contractRevision},
	}
}

func updateStorageResponsibility(np NegotiationProtocol, req storage.ContractCreateRequest, sr storagehost.StorageResponsibility, hostConfig storage.HostIntConfig) storagehost.StorageResponsibility {
	// try to get storage responsibility first
	oldSr, err := np.GetStorageResponsibility(req.OldContractID)
	if err == nil {
		sr.SectorRoots = oldSr.SectorRoots
	}

	// calculate the renew revenue and update the storage responsibility
	renewRevenue := renewBasePrice(sr, hostConfig.StoragePrice, req.StorageContract.WindowEnd, req.StorageContract.FileSize)
	sr.ContractCost = common.PtrBigInt(req.StorageContract.ValidProofOutputs[validProofPaybackHostAddressIndex].Value).Sub(hostConfig.ContractPrice).Sub(renewRevenue)
	sr.PotentialStorageRevenue = renewRevenue
	sr.RiskedStorageDeposit = renewBaseDeposit(sr, req.StorageContract.WindowEnd, req.StorageContract.FileSize, hostConfig.Deposit)
	return sr
}

func updateHostContractRecord(sp storage.Peer, np NegotiationProtocol, contractID common.Hash) error {
	// get the peer node
	node := sp.PeerNode()
	if node == nil {
		return fmt.Errorf("failed to update the host contract record: peer node cannot be nil")
	}

	// set the connection to static connection
	np.SetStatic(node)

	// insert the contract
	np.InsertContract(node.String(), contractID)

	return nil
}

func signAndUpdateContractRevision(contractRevision types.StorageContractRevision, negotiationData contractNegotiation, clientRevisionSign []byte) (types.StorageContractRevision, error) {
	// get the host revision sign
	hostRevisionSign, err := hostSignedContract(negotiationData.account, negotiationData.wallet, contractRevision.RLPHash().Bytes())
	if err != nil {
		return types.StorageContractRevision{}, err
	}

	// update the storage contract revision signatures and return
	contractRevision.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}
	return contractRevision, nil
}

// hostSignedContract will return the contract signed by the storage host
func hostSignedContract(account accounts.Account, wallet accounts.Wallet, storageContract []byte) ([]byte, error) {
	hostContractSign, err := wallet.SignHash(account, storageContract)
	if err != nil {
		err = fmt.Errorf("storage host failed to sign the storage contract: %s", err.Error())
		return []byte{}, common.ErrCompose(storage.ErrHostNegotiate, err)
	}

	return hostContractSign, nil
}

// hostPubKeyRecovery will recover the storage host public key based on the host signed storage
// contract
func hostPubKeyRecovery(storageContract []byte, hostSign []byte) (*ecdsa.PublicKey, error) {
	hostPubKey, err := crypto.SigToPub(storageContract, hostSign)
	if err != nil {
		return nil, fmt.Errorf("storage host failed to recover its public key from its signature: %s", err.Error())
	}
	return hostPubKey, nil
}
