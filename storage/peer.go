// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

type Peer interface {
	TriggerError(error)
	SendStorageHostConfig(config HostExtConfig) error
	RequestStorageHostConfig() error
	SendUploadMerkleProof(merkleProof UploadMerkleProof) error
	RequestContractCreation(req ContractCreateRequest) error
	SendContractCreateClientRevisionSign(revisionSign []byte) error
	SendContractCreationHostSign(contractSign []byte) error
	SendContractCreationHostRevisionSign(revisionSign []byte) error
}
