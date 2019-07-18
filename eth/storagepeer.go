// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"time"
)

func (p *peer) TriggerError(err error) {
	select {
	case p.errMsg <- err:
	default:
	}
}

func (p *peer) SendStorageHostConfig(config storage.HostExtConfig) error {
	return p2p.Send(p.rw, storage.HostConfigRespMsg, config)
}

func (p *peer) RequestStorageHostConfig() error {
	return p2p.Send(p.rw, storage.HostConfigReqMsg, struct{}{})
}

func (p *peer) RequestContractCreation(req storage.ContractCreateRequest) error {
	return p2p.Send(p.rw, storage.ContractCreateReqMsg, req)
}

func (p *peer) SendContractCreateClientRevisionSign(revisionSign []byte) error {
	return p2p.Send(p.rw, storage.ContractCreateClientRevisionSign, revisionSign)
}

func (p *peer) SendContractCreationHostSign(contractSign []byte) error {
	return p2p.Send(p.rw, storage.ContractCreateHostSign, contractSign)
}

func (p *peer) SendContractCreationHostRevisionSign(revisionSign []byte) error {
	return p2p.Send(p.rw, storage.ContractCreateRevisionSign, revisionSign)
}

func (p *peer) RequestContractUpload(req storage.UploadRequest) error {
	return p2p.Send(p.rw, storage.ContractUploadReqMsg, req)
}

func (p *peer) SendContractUploadClientRevisionSign(revisionSign []byte) error {
	return p2p.Send(p.rw, storage.ContractUploadClientRevisionSign, revisionSign)
}

func (p *peer) SendUploadMerkleProof(merkleProof storage.UploadMerkleProof) error {
	return p2p.Send(p.rw, storage.ContractUploadMerkleProofMsg, merkleProof)
}

func (p *peer) SendUploadHostRevisionSign(revisionSign []byte) error {
	return p2p.Send(p.rw, storage.ContractUploadRevisionSign, revisionSign)
}

func (p *peer) RequestContractDownload(req storage.DownloadRequest) error {
	return p2p.Send(p.rw, storage.ContractDownloadReqMsg, req)
}

func (p *peer) SendRevisionStop() error {
	return p2p.Send(p.rw, storage.NegotiationStopMsg, "revision stop")
}

func (p *peer) SendContractDownloadData(resp storage.DownloadResponse) error {
	return p2p.Send(p.rw, storage.ContractDownloadDataMsg, resp)
}

func (p *peer) WaitConfigResp() (msg p2p.Msg, err error) {
	timeout := time.After(1 * time.Minute)
	select {
	case msg = <-p.clientConfigMsg:
		return
	case <-timeout:
		err = errors.New("timeout -> client waits too long for config response from the host")
		return
	}
}

func (p *peer) ClientWaitContractResp() (msg p2p.Msg, err error) {
	timeout := time.After(1 * time.Minute)
	select {
	case msg = <-p.clientContractMsg:
		return
	case <-timeout:
		err = errors.New("timeout -> client waits too long for contract response from the host")
		return
	}
}

func (p *peer) HostWaitContractResp() (msg p2p.Msg, err error) {
	timeout := time.After(1 * time.Minute)
	select {
	case msg = <-p.hostContractMsg:
		return
	case <-timeout:
		err = errors.New("timeout -> host waits too long for contract response from the host")
		return
	}
}

func (p *peer) HostConfigProcessing() error {
	select {
	case p.hostConfigProcessing <- struct{}{}:
		return nil
	default:
		return errors.New("host config request is currently processing, please wait until it finished first")
	}
}

func (p *peer) HostConfigProcessingDone() {
	select {
	case <-p.hostConfigProcessing:
		return
	default:
		p.Log().Warn("host config processing finished before it is actually done")
	}
}

func (p *peer) HostContractProcessing() error {
	select {
	case p.hostContractProcessing <- struct{}{}:
		return nil
	default:
		return errors.New("host contract related operation is currently processing, please wait until it finished first")
	}
}

func (p *peer) HostContractProcessingDone() {
	select {
	case <-p.hostContractProcessing:
		return
	default:
		p.Log().Warn("host contract processing finished before it is actually done")
	}
}

func (p *peer) RevisionStart() error {
	select {
	case p.contractRevising <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("another operation is revising the contract, or renewing started")
	}
}

func (p *peer) IsRevising() bool {
	select {
	case p.contractRevising <- struct{}{}:
		<-p.contractRevising
		return false
	default:
		return true
	}
}

func (p *peer) RevisionDone() {
	select {
	case <-p.contractRevising:
	default:
	}
}
