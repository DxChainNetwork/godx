// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

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
