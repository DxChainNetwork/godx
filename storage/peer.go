// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

import "github.com/DxChainNetwork/godx/p2p"

type Peer interface {
	TriggerError(error)
	SendStorageHostConfig(config HostExtConfig) error
	RequestStorageHostConfig() error
	SendUploadMerkleProof(merkleProof UploadMerkleProof) error
	RequestContractCreation(req ContractCreateRequest) error
	SendContractCreateClientRevisionSign(revisionSign []byte) error
	SendContractCreationHostSign(contractSign []byte) error
	SendContractCreationHostRevisionSign(revisionSign []byte) error
	WaitConfigResp() (p2p.Msg, error)
	ClientWaitContractResp() (msg p2p.Msg, err error)
	HostWaitContractResp() (msg p2p.Msg, err error)
}
