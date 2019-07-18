package eth

import (
	"errors"
	"github.com/DxChainNetwork/godx/p2p"
)

func (pm *ProtocolManager) hostConfigMsgHandler(p *peer, configMsg p2p.Msg) {
	config := pm.eth.storageHost.RetrieveExternalConfig()
	if err := p.SendStorageHostConfig(config); err != nil {
		p.TriggerError(err)
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
