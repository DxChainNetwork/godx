// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
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

func (p *peer) SendUploadMerkleProof(merkleProof storage.UploadMerkleProof) error {
	return p2p.Send(p.rw, storage.UploadMerkleProofMsg, merkleProof)
}
