// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

import (
	"errors"
	"github.com/DxChainNetwork/godx/p2p"
)

var ErrRequestingHostConfig = errors.New("host configuration should only be requested one at a time")

type Peer interface {
	TriggerError(error)
	SendStorageHostConfig(config HostExtConfig) error
	RequestStorageHostConfig() error
	SendUploadMerkleProof(merkleProof UploadMerkleProof) error
	RequestContractCreation(req ContractCreateRequest) error
	SendContractCreateClientRevisionSign(revisionSign []byte) error
	SendContractCreationHostSign(contractSign []byte) error
	SendContractCreationHostRevisionSign(revisionSign []byte) error
	RequestContractUpload(req UploadRequest) error
	SendContractUploadClientRevisionSign(revisionSign []byte) error
	SendUploadHostRevisionSign(revisionSign []byte) error
	RequestContractDownload(req DownloadRequest) error
	SendRevisionStop() error
	SendContractDownloadData(resp DownloadResponse) error
	WaitConfigResp() (p2p.Msg, error)
	ClientWaitContractResp() (msg p2p.Msg, err error)
	HostWaitContractResp() (msg p2p.Msg, err error)
	RevisionStart() error
	IsRevising() bool
	RevisionDone()
	IsRequestingConfig() error
	DoneRequestingConfig()
}
