// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiation

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func retrieveClientAndHostPubKey(session *hostnegotiation.ContractCreateSession, req storage.ContractCreateRequest, sc types.StorageContract) (*ecdsa.PublicKey, *ecdsa.PublicKey, error) {
	// get the client public key
	clientPubKey, err := crypto.SigToPub(req.StorageContract.RLPHash().Bytes(), req.Sign)
	if err != nil {
		return nil, nil, negotiationError(err.Error())
	}

	// get the host public key
	hostPubKey, err := crypto.SigToPub(req.StorageContract.RLPHash().Bytes(), sc.Signatures[hostSignIndex])
	if err != nil {
		return nil, nil, negotiationError(err.Error())
	}

	// save information into session for later use
	session.ClientPubKey = clientPubKey
	session.HostPubKey = hostPubKey

	return clientPubKey, hostPubKey, nil
}

// retrieveAccountAndWallet will retrieve the host's wallet and account based on the host address extracted
// from the client's contract create request. The wallet is used to sign the contract
func retrieveAccountAndWallet(np hostnegotiation.Protocol, req storage.ContractCreateRequest) (accounts.Wallet, accounts.Account, error) {
	// get the host address from the contract create request, and construct the account
	hostAddress := req.StorageContract.ValidProofOutputs[validProofPaybackHostAddressIndex].Address
	account := accounts.Account{Address: hostAddress}

	// get the wallet and return it back
	wallet, err := np.FindWallet(account)
	if err != nil {
		return nil, accounts.Account{}, negotiationError(err.Error())
	}
	return wallet, account, nil
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

func waitAndHandleClientCommitRespContractCrate(sp storage.Peer, np hostnegotiation.Protocol, req storage.ContractCreateRequest, sc types.StorageContract, contractRevision types.StorageContractRevision) (storagehost.StorageResponsibility, error) {
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

func clientCommitSuccessHandle(np hostnegotiation.Protocol, req storage.ContractCreateRequest, sc types.StorageContract, contractRevision types.StorageContractRevision) (storagehost.StorageResponsibility, error) {
	// get the block number
	blockNumber := np.GetBlockHeight()
	hostConfig := np.GetHostConfig()

	// construct the storage responsibility
	sr := constructStorageResponsibility(hostConfig.ContractPrice, blockNumber, sc, contractRevision)
	if req.Renew {
		sr = updateStorageResponsibilityContractCreate(np, req, sr, hostConfig)
	}

	// commit storage responsibility
	if err := np.FinalizeStorageResponsibility(sr); err != nil {
		err = fmt.Errorf("clientCommitSuccessHandle failed, failed to finalize the storage responsibility: %s", err.Error())
		return storagehost.StorageResponsibility{}, common.ErrCompose(err, storage.ErrHostCommit)
	}

	return sr, nil
}

func constructContractRevision(sc types.StorageContract, session hostnegotiation.ContractCreateSession, clientRevSign []byte) (types.StorageContractRevision, error) {
	cr := types.StorageContractRevision{
		ParentID: sc.RLPHash(),
		UnlockConditions: types.UnlockConditions{
			PaymentAddresses: []common.Address{
				crypto.PubkeyToAddress(*session.ClientPubKey),
				crypto.PubkeyToAddress(*session.HostPubKey),
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

	// get the host revision sign
	hostRevSign, err := session.Wallet.SignHash(session.Account, cr.RLPHash().Bytes())
	if err != nil {
		return types.StorageContractRevision{}, negotiationError(err.Error())
	}

	// update the storage contract revision's signature and return
	cr.Signatures = [][]byte{clientRevSign, hostRevSign}
	return cr, nil
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

func updateStorageResponsibilityContractCreate(np hostnegotiation.Protocol, req storage.ContractCreateRequest, sr storagehost.StorageResponsibility, hostConfig storage.HostIntConfig) storagehost.StorageResponsibility {
	// try to get storage responsibility first
	oldSr, err := np.GetStorageResponsibility(req.OldContractID)
	if err == nil {
		sr.SectorRoots = oldSr.SectorRoots
	}

	// calculate the renew revenue and update the storage responsibility
	renewRevenue := renewBaseCost(sr, hostConfig.StoragePrice, req.StorageContract.WindowEnd, req.StorageContract.FileSize)
	sr.ContractCost = common.PtrBigInt(req.StorageContract.ValidProofOutputs[validProofPaybackHostAddressIndex].Value).Sub(hostConfig.ContractPrice).Sub(renewRevenue)
	sr.PotentialStorageRevenue = renewRevenue
	sr.RiskedStorageDeposit = renewBaseDeposit(sr, req.StorageContract.WindowEnd, req.StorageContract.FileSize, hostConfig.Deposit)
	return sr
}

func updateHostContractRecord(sp storage.Peer, np hostnegotiation.Protocol, contractID common.Hash) error {
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
