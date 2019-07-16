package eth

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

func (pm *ProtocolManager) clientHostConfigMsgHandler(p *peer, configMsg chan p2p.Msg) {
	var msg p2p.Msg
	for {
		// waiting for config response message
		select {
		case msg = <-configMsg:
		case <-pm.quitSync:
			return
		}

		// double check the message code
		if msg.Code != storage.HostConfigRespMsg {
			err := fmt.Errorf("error analyzing the host config message, expected message code 0x20, got %x", msg.Code)
			p.TriggerError(err)
		}

		// handle the config response message
		op, err := pm.eth.storageClient.RetrieveOperation(p.ID(), storage.ConfigOP)
		if err != nil {
			p.TriggerError(err)
			return
		}

		if err := op.Done(msg); err != nil {
			err = fmt.Errorf("handle host setting message failed: %s", err.Error())
			p.TriggerError(err)
			return
		}
	}
}

func (pm *ProtocolManager) hostConfigMsgHandler(p *peer, configMsg chan p2p.Msg) {
	var msg p2p.Msg
	for {
		// waiting for the config request message
		select {
		case msg = <-configMsg:
		case <-pm.quitSync:
			return
		}

		// double check the message code
		if msg.Code != storage.HostConfigReqMsg {
			err := fmt.Errorf("error analyzing the host config message, expected message code 0x30, got %x", msg.Code)
			p.TriggerError(err)
		}

		// get the storage host configuration and send it back to storage client
		config := pm.eth.storageHost.RetrieveExternalConfig()
		if err := p.SendStorageHostConfig(config); err != nil {
			p.TriggerError(err)
		}
	}

}

func (pm *ProtocolManager) clientContractMsgHandler(p *peer, contractMsg chan p2p.Msg) {
	var msg p2p.Msg
	for {
		select {
		case msg = <-contractMsg:
			op, err := pm.eth.storageClient.RetrieveOperation(p.ID(), storage.ContractOP)
			if err != nil {
				log.Error("failed to retrieve operation from the storage client while handling the client contract message", "err", err.Error())
				p.TriggerError(err)
				return
			}

			if err := op.Done(msg); err != nil {
				err = fmt.Errorf("error operation done: %s", err.Error())
				log.Error("error handling client contract message", "err", err.Error())
				p.TriggerError(err)
				return
			}

		case <-pm.quitSync:
			return
		}
	}
}

func (pm *ProtocolManager) hostContractMsgHandler(p *peer, contractMsg chan p2p.Msg) {
	var msg p2p.Msg
	for {
		select {
		case msg = <-contractMsg:
		case <-pm.quitSync:
			return
		}

		switch {
		case msg.Code == storage.ContractCreateReqMsg:
			go func() { pm.eth.storageHost.ContractCreateHandler(p, msg) }()
		default:
			if err := pm.eth.storageHost.InsertMsg(msg); err != nil {
				err = fmt.Errorf("error insert message: %s", err.Error())
			}
		}
	}
}

func (pm *ProtocolManager) ethMsgHandler(p *peer) {
	// get the initial number of eth messages in the ethMsgBuffer
	messages := p.GetEthMsgBuffer()

	for {
		// loop through the messages, handle each of them, and then
		// update the eth message buffer
		for _, msg := range messages {
			if err := pm.handleEthMsg(p, msg); err != nil {
				p.Log().Error("Ethereum handle message failed", "err", err.Error())
				p.TriggerError(err)
			}

			// remove the message from the eth message buffer
			p.UpdateEthMsgBuffer()
		}

		// waiting fro the start signal was sent, then update the
		// eth message buffer
		if err := pm.waitEthStartIndicator(p); err != nil {
			return
		}
		messages = p.GetEthMsgBuffer()
	}
}

func (pm *ProtocolManager) waitEthStartIndicator(p *peer) error {
	select {
	case <-p.ethStartIndicator:
		return nil
	case <-pm.quitSync:
		return errors.New("protocol manager sync quit")
	}
}
