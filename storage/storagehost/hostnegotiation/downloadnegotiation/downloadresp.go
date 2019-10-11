// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiation

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// signDownloadRevision will retrieve the host's wallet information, which will be used
// to sign the new constructed download contract revision
func signAndUpdateDownloadRevision(np hostnegotiation.Protocol, ds *hostnegotiation.DownloadSession, newRev types.StorageContractRevision, clientSign []byte) error {
	// retrieve the wallet information
	account := accounts.Account{Address: newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Address}
	wallet, err := np.FindWallet(account)
	if err != nil {
		return errFindWallet
	}

	// sign the download revision and update the contract revision
	hostSign, err := wallet.SignHash(account, newRev.RLPHash().Bytes())
	if err != nil {
		return errHostRevSign
	}

	// update and return the new revision
	newRev.Signatures = [][]byte{clientSign, hostSign}
	ds.NewRev = newRev
	return nil
}

// constructDownloadResp will fetch the data requested by the storage client, construct the merkle proof
// and construct the download response for the storage host to send back to the client
func constructDownloadResp(np hostnegotiation.Protocol, req storage.DownloadRequest, hostSign []byte) (storage.DownloadResponse, error) {
	// fetch the requested data from the local storage
	dataRequested, err := fetchDownloadData(np, req.Sector)
	if err != nil {
		return storage.DownloadResponse{}, err
	}

	// construct the merkle proof is requested
	merkleProof, err := constructMerkleProof(req, req.Sector, dataRequested)
	if err != nil {
		return storage.DownloadResponse{}, err
	}

	// construct and return the download response
	return storage.DownloadResponse{
		Signature:   hostSign,
		Data:        dataRequested,
		MerkleProof: merkleProof,
	}, nil
}

// fetchDownloadData will fetch the data requested by the storage client from the
// local storage
func fetchDownloadData(np hostnegotiation.Protocol, dataSector storage.DownloadRequestSector) ([]byte, error) {
	sectorData, err := np.ReadSector(dataSector.MerkleRoot)
	if err != nil {
		return nil, errFetchData
	}

	return sectorData[dataSector.Offset : dataSector.Offset+dataSector.Length], nil
}

func constructMerkleProof(req storage.DownloadRequest, dataSector storage.DownloadRequestSector, dataRequested []byte) ([]common.Hash, error) {
	// construct the merkle proof is requested
	if req.MerkleProof {
		proofStart := int(dataSector.Offset) / merkle.LeafSize
		proofEnd := int(dataSector.Offset) / merkle.LeafSize
		merkleProof, err := merkle.Sha256RangeProof(dataRequested, proofStart, proofEnd)
		if err != nil {
			return nil, errFetchMerkleProof
		}

		return merkleProof, nil
	}

	// return empty hash directly if the merkle proof is not requested
	return []common.Hash{}, nil
}
