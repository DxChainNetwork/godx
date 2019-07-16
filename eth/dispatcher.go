// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"errors"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

func (pm *ProtocolManager) msgDispatcher(msg p2p.Msg, p *peer) error {
	switch {
	case msg.Code < 0x20:
		// ethMsgScheduler
		return pm.ethMsgScheduler(msg, p)

	case msg.Code < 0x30:
		// clientMsgScheduler
		return pm.clientMsgScheduler(msg, p)

	case msg.Code < 0x40:
		// hostMsgScheduler
		return pm.hostMsgScheduler(msg, p)

	default:
		// message code exceed the range
		return errors.New("invalid message code")
	}

}

func (pm *ProtocolManager) ethMsgScheduler(msg p2p.Msg, p *peer) error {
	// insert the message into eth buffer
	p.InsertEthMsgBuffer(msg)
	select {
	// send the start signal, indicating there
	// is a new message added into the buffer
	case p.ethStartIndicator <- struct{}{}:
		return nil
	default:
		// if blocked, indicating that the ethMsgHandler is started already
		// the messages just inserted will be handled eventually
		return nil
	}
}

func (pm *ProtocolManager) clientMsgScheduler(msg p2p.Msg, p *peer) error {
	// if the message is hostConfigRespMsg, try to push it to the channel
	// if failed, discard the message right away, meaning the last config
	// message handling is not finished yet
	if msg.Code == storage.HostConfigRespMsg {
		select {
		case p.clientConfigMsg <- msg:
			return nil
		default:
			return msg.Discard()
		}
	}

	// otherwise, push the message into clientContractMsg channel
	// similarly, if the channel is full, meaning the previous message
	// handling was not complete, trigger the error directly because the
	// client should not receive the request before the handling finished
	select {
	case p.clientContractMsg <- msg:
		return nil
	default:
		err := errors.New("clientMsgScheduler error: message received before finishing the previous message handling")
		log.Error("error handling clientContractMsg", "err", err.Error())
		return err
	}
}

func (pm *ProtocolManager) hostMsgScheduler(msg p2p.Msg, p *peer) error {
	// if the message is hostConfigReqMsg, try to push it to the channel
	// if failed, discard the message right away, meaning the last config
	// message handling is not finished yet
	if msg.Code == storage.HostConfigReqMsg {
		select {
		case p.hostConfigMsg <- msg:
			return nil
		default:
			return msg.Discard()
		}
	}

	// otherwise, push the message into hostContractMsg channel
	// similarly, if the channel is full, meaning the previous message handling
	// was not completed yet, trigger the error directly
	select {
	case p.hostContractMsg <- msg:
		return nil
	default:
		err := errors.New("hostMsgScheduler error: message received before finishing the previous message handling")
		log.Error("error handling hostContractMsg", "err", err.Error())
		return err
	}
}
