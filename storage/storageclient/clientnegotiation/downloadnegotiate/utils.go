// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiate

import (
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/storage/storageclient/clientnegotiation"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

// Special Types Error:
// 1. ErrClientNegotiate   ->  send negotiation failed message, wait response
// 2. ErrClientCommit      ->  send commit failed message, wait response
// 3. ErrHostCommit		   ->  sendACK, wait response, punish host, check and update the connection
// 4. ErrHostNegotiate     ->  punish host, check and update the connection
func handleNegotiationErr(ne clientnegotiation.NegotiationError, err *error, hostID enode.ID, sp storage.Peer, it storagehostmanager.InteractionType) {
	// if no error, reward the host and return directly
	if err == nil {
		ne.IncrementSuccessfulInteractions(hostID, it)
		return
	}

	// otherwise, based on the error type, handle it differently
	switch {
	case common.ErrContains(*err, storage.ErrClientNegotiate):
		_ = sp.SendClientNegotiateErrorMsg()
	case common.ErrContains(*err, storage.ErrClientCommit):
		_ = sp.SendClientCommitFailedMsg()
	case common.ErrContains(*err, storage.ErrHostNegotiate):
		ne.IncrementFailedInteractions(hostID, it)
		ne.CheckAndUpdateConnection(sp.PeerNode())
		return
	case common.ErrContains(*err, storage.ErrHostCommit):
		_ = sp.SendClientAckMsg()
		ne.IncrementFailedInteractions(hostID, it)
		ne.CheckAndUpdateConnection(sp.PeerNode())
		_ = sp.SendClientAckMsg()
	default:
		return
	}

	// wait until host sent back ACK message
	if msg, respErr := sp.ClientWaitContractResp(); respErr != nil || msg.Code != storage.HostAckMsg {
		log.Error("handleNegotiateErr error", "type", err, "err", respErr, "msgCode", msg.Code)
	}
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

func newContractRevision(current types.StorageContractRevision, cost *big.Int) types.StorageContractRevision {
	rev := current

	rev.NewValidProofOutputs = make([]types.DxcoinCharge, 2)
	rev.NewMissedProofOutputs = make([]types.DxcoinCharge, 2)

	for i, v := range current.NewValidProofOutputs {
		rev.NewValidProofOutputs[i] = types.DxcoinCharge{
			Address: v.Address,
			Value:   big.NewInt(v.Value.Int64()),
		}
	}

	for i, v := range current.NewMissedProofOutputs {
		rev.NewMissedProofOutputs[i] = types.DxcoinCharge{
			Address: v.Address,
			Value:   big.NewInt(v.Value.Int64()),
		}
	}

	// move valid payout from client to host
	rev.NewValidProofOutputs[0].Value.Sub(current.NewValidProofOutputs[0].Value, cost)
	rev.NewValidProofOutputs[1].Value.Add(current.NewValidProofOutputs[1].Value, cost)

	// move missed payout from client to void, mean that will burn missed payout of client
	rev.NewMissedProofOutputs[0].Value.Sub(current.NewMissedProofOutputs[0].Value, cost)

	// increment revision number
	rev.NewRevisionNumber++

	return rev
}

// storageContractRevisionCommit will commit the storage upload contract revision
func storageContractRevisionCommit(sp storage.Peer, contractRevision types.StorageContractRevision, contract *contractset.Contract, prices ...common.BigInt) error {
	// commit the upload contract revision
	unCommitContractHeader := contract.Header()
	if err := contract.CommitRevision(contractRevision, prices...); err != nil {
		err = fmt.Errorf("client failed to commit the contract revision while uploading/downloading: %s", err.Error())
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
