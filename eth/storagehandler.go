package eth

import (
	"errors"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func (pm *ProtocolManager) hostConfigMsgHandler(p *peer, configMsg p2p.Msg) error {
	// avoid multiple host config request calls attack
	// generate too many go routines and used all resources
	if err := p.HostConfigProcessing(); err != nil {
		return err
	}

	// start the go routine, handle the host config request
	// once done, release the channel
	go func() {
		pm.wg.Add(1)
		defer pm.wg.Done()
		defer p.HostConfigProcessingDone()
		config := pm.eth.storageHost.RetrieveExternalConfig()
		if err := p.SendStorageHostConfig(config); err != nil {
			p.TriggerError(err)
		}
	}()

	return nil
}

func (pm *ProtocolManager) contractMsgHandler(p *peer, msg p2p.Msg) error {
	// send the message to the hostContractMsg channel if the handler
	// does not exist
	select {
	case p.hostContractMsg <- msg:
	default:
		err := errors.New("hostMsgSchedule error: message received before finishing the previous message handling")
		log.Error("error handling hostContractMsg", "err", err.Error())
		return err
	}
	return nil
}

func (pm *ProtocolManager) contractReqHandler(handler func(h *storagehost.StorageHost, sp storage.Peer, msg p2p.Msg), p *peer, msg p2p.Msg) error {
	// avoid continuously contract related requests attack
	// generate too many go routines and used all resources
	if err := p.HostContractProcessing(); err != nil {
		// error is ignored intentionally. If error occurred,
		// the client must wait until time out
		_ = p.SendHostBusyHandleRequestErr()
		return err
	}

	// start the go routine, handle the host contract request
	// once done, release the channel
	go func() {
		pm.wg.Add(1)
		defer pm.wg.Done()
		defer p.HostContractProcessingDone()
		handler(pm.eth.storageHost, p, msg)
	}()

	return nil
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
