// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

import (
	"errors"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
)

// ErrRequestingHostConfig is the error code used when the client asks host for its configuration multiple
// times before the host finished handling the previous configuration request. Therefore, the host's evaluation
// should not be deducted.
var ErrRequestingHostConfig = errors.New("host configuration should only be requested one at a time")

// Peer is the interface returned by the SetupConnection. The use of it is to allow eth.peer object
// to be used in the storage model. All the methods provided in the Peer interface is used for negotiation
// during the contract create, contract revision, contract renew, and configuration request
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
	TryToRenewOrRevise() bool
	RevisionOrRenewingDone()
	TryRequestHostConfig() error
	RequestHostConfigDone()
	PeerNode() *enode.Node
}
