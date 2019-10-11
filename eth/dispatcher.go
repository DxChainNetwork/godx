// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"errors"

	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

var hostHandlers = map[uint64]func(np hostnegotiation.Protocol, sp storage.Peer, msg p2p.Msg){
	storage.ContractCreateReqMsg: hostnegotiation.ContractCreateRenewHandler,
	storage.ContractUploadReqMsg: hostnegotiation.ContractUploadHandler,
	//storage.ContractDownloadReqMsg: storagehost.DownloadHandler,
}

func (pm *ProtocolManager) msgDispatch(msg p2p.Msg, p *peer) error {
	switch {
	case msg.Code < 0x20:
		// ethMsgSchedule
		return pm.ethMsgSchedule(msg, p)

	case msg.Code < 0x30:
		// clientMsgSchedule
		return pm.clientMsgSchedule(msg, p)

	case msg.Code < 0x40:
		// hostMsgSchedule
		return pm.hostMsgSchedule(msg, p)

	default:
		// message code exceed the range
		return errors.New("invalid message code")
	}
}

func (pm *ProtocolManager) ethMsgSchedule(msg p2p.Msg, p *peer) error {
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

func (pm *ProtocolManager) clientMsgSchedule(msg p2p.Msg, p *peer) error {
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
		err := errors.New("clientMsgSchedule error: message received before finishing the previous message handling")
		log.Error("error handling clientContractMsg", "err", err.Error())
		return err
	}
}

func (pm *ProtocolManager) hostMsgSchedule(msg p2p.Msg, p *peer) error {
	// check if the message code is HostConfigReqMsg, which needs to be handled
	// explicitly
	if msg.Code == storage.HostConfigReqMsg {
		return pm.hostConfigMsgHandler(p, msg)
	}

	// gets the handler based on the message code,
	// if the handler does not exists, meaning it is not request message
	// handle it as a dialogue message
	handler, exists := hostHandlers[msg.Code]
	if !exists {
		return pm.contractMsgHandler(p, msg)
	}

	// if handler exists, handle it as the request
	return pm.contractReqHandler(handler, p, msg)
}
