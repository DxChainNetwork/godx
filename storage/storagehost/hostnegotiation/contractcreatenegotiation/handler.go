// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

func Handler(np hostnegotiation.Protocol, sp storage.Peer, contractCreateReqMsg p2p.Msg) {
	var negotiateErr error
	var session hostnegotiation.ContractCreateSession
	defer handleNegotiationErr(np, sp, &negotiateErr)

	// 1. check if the storage host is accepting contract, if not, return error directly
	if !np.IsAcceptingContract() {
		negotiateErr = errNoNewContract
		return
	}

	// 2. decode and validate storage contract create request
	req, err := decodeAndValidateReq(np, &session, contractCreateReqMsg)
	if err != nil {
		negotiateErr = err
		return
	}

	// 3. storage host sign and update the storage contract
	sc, err := signAndUpdateContract(np, &session, req)
	if err != nil {
		negotiateErr = err
		return
	}

	// 4. storage contract validation
	if err := contractValidation(np, &session, req, sc); err != nil {
		negotiateErr = err
		return
	}

	// 5. start the contract create host sign negotiation and get the client signed contract revision
	clientRevisionSign, err := contractCreateNegotiation(sp, sc)
	if err != nil {
		negotiateErr = err
		return
	}

	// 6. negotiate contract revision with the storage client
	if err := contractRevisionNegotiation(np, sp, session, sc, clientRevisionSign, req); err != nil {
		negotiateErr = err
		return
	}
}

// decodeAndValidateReq will decode and validate the contract create request
func decodeAndValidateReq(np hostnegotiation.Protocol, session *hostnegotiation.ContractCreateSession, contractCreateReqMsg p2p.Msg) (storage.ContractCreateRequest, error) {
	// decode the contract create request
	var req storage.ContractCreateRequest
	if err := contractCreateReqMsg.Decode(&req); err != nil {
		return storage.ContractCreateRequest{}, negotiationError(err.Error())
	}

	// return the decoded contract create request, and send back
	// the validation result
	return req, contractCreateReqValidation(np, req)
}

// signAndUpdateContract will sign the storage contract using the host's wallet and account information
// In addition, the contract's signatures will be updated
func signAndUpdateContract(np hostnegotiation.Protocol, session *hostnegotiation.ContractCreateSession, req storage.ContractCreateRequest) (types.StorageContract, error) {
	// extracts data from the contract create request
	clientSign := req.Sign
	sc := req.StorageContract

	// retrieve the host's account and the wallet information
	wallet, account, err := retrieveAccountAndWallet(np, req)
	if err != nil {
		return types.StorageContract{}, err
	}
	session.Wallet = wallet
	session.Account = account

	// sign the storage contract using host's account
	hostSign, err := wallet.SignHash(account, sc.RLPHash().Bytes())
	if err != nil {
		return types.StorageContract{}, negotiationError(err.Error())
	}

	// update the storage contract signatures field
	sc.Signatures = [][]byte{clientSign, hostSign}
	return sc, nil
}

// contractValidation will validate the storage contract sent by the storage client
func contractValidation(np hostnegotiation.Protocol, session *hostnegotiation.ContractCreateSession,
	req storage.ContractCreateRequest, sc types.StorageContract) error {
	// retrieve client and host public key for evaluation usage
	clientPubKey, hostPubKey, err := retrieveClientAndHostPubKey(session, req, sc)
	if err != nil {
		return err
	}

	// get needed data
	blockHeight := np.GetBlockHeight()
	hostConfig := np.GetHostConfig()
	lockedStorageDeposit := np.GetFinancialMetrics().LockedStorageDeposit

	// a part of validation that is needed no matter the validation is for contract create
	// or contract renew
	if err := contractCreateOrRenewValidation(clientPubKey, hostPubKey, sc, blockHeight, hostConfig); err != nil {
		return err
	}

	// check if the contract is renewing
	if req.Renew {
		return contractRenewValidation(np, req.OldContractID, sc, hostConfig, lockedStorageDeposit)
	}

	// contract create validation
	return contractCreateValidation(sc, hostConfig, lockedStorageDeposit)
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

func contractRevisionNegotiation(np hostnegotiation.Protocol, sp storage.Peer, session hostnegotiation.ContractCreateSession,
	sc types.StorageContract, clientRevSign []byte, req storage.ContractCreateRequest) error {

	// construct and update the storage contract revision
	cr, err := constructContractRevision(sc, session, clientRevSign)
	if err != nil {
		return err
	}

	// start the revision negotiation
	if err := sp.SendContractCreationHostRevisionSign(cr.Signatures[hostSignIndex]); err != nil {
		err = fmt.Errorf("contract reivision negotiation failed, failed to send contract creation host revision sign: %s", err.Error())
		return err
	}

	// wait and handle client contract commit response
	sr, err := waitAndHandleClientCommitRespContractCrate(sp, np, req, sc, cr)
	if err != nil {
		return err
	}

	// update contract record
	if err := updateHostContractRecord(sp, np, sc.ID()); err != nil {
		return err
	}

	// send host host ACK message
	if err := sp.SendHostAckMsg(); err != nil {
		_ = np.RollBackCreateStorageResponsibility(sr)
		np.RollBackConnectionType(sp)
	}

	return nil
}

// handleNegotiationErr will handle the following error cases
//  1. no error          -> return directly
//  2. ErrHostCommit     -> send host commit, after getting response from client, send host ack message
//  3. other error types -> send host negotiation error
func handleNegotiationErr(np hostnegotiation.Protocol, sp storage.Peer, negotiateErr *error) {
	// return directly if there are no errors
	if negotiateErr == nil {
		return
	}

	// check and handle other negotiation errors
	if common.ErrContains(*negotiateErr, storage.ErrHostCommit) {
		_ = sp.SendHostCommitFailedMsg()
		// wait for client ack message
		if _, err := sp.HostWaitContractResp(); err != nil {
			return
		}
		// send back host ack message
		_ = sp.SendHostAckMsg()
	} else {
		// for other negotiation error message, send back the host negotiation error message
		// directly
		_ = sp.SendHostNegotiateErrorMsg()
		np.CheckAndUpdateConnection(sp.PeerNode())
	}
}
