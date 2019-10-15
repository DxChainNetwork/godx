// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiate

import (
	"fmt"
	"io"
	"math/bits"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage/storageclient/clientnegotiation"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
)

func Handler(dp clientnegotiation.DownloadProtocol, sp storage.Peer, downloadData io.Writer, downloadReq storage.DownloadRequest, hostInfo storage.HostInfo) (negotiateErr error) {
	// validate the download request
	if err := downloadRequestValidation(downloadReq); err != nil {
		negotiateErr = err
		return
	}

	// get the contract based on the hostID
	contract, err := dp.GetContractBasedOnHostID(hostInfo.EnodeID)
	if err != nil {
		negotiateErr = fmt.Errorf("download negotiation failed, failed to get the contract: %s", err.Error())
		return
	}

	// return the contract at the end
	defer dp.ContractReturn(contract)

	// calculate and validate the download price
	downloadPrice, err := calculateAndValidateSectorDownloadPrice(downloadReq, hostInfo, contract.Header().LatestContractRevision)
	if err != nil {
		return err
	}

	// form new download contract revision
	downloadRevision, err := formDownloadContractRevision(contract, downloadPrice)
	if err != nil {
		negotiateErr = fmt.Errorf("download negotiation failed, failed to form the download contract reivision: %s", err.Error())
		return
	}

	// client sign the download contract revision
	clientRevisionSign, err := clientSignDownloadContractRevision(dp.GetAccountManager(), downloadRevision)
	if err != nil {
		negotiateErr = fmt.Errorf("download negotiation failed, failed to create client download contract reivision sign: %s", err.Error())
		return
	}

	// update the download request
	downloadReq = updateDownloadRequest(downloadReq, clientRevisionSign, downloadRevision)
	defer handleContractDownloadErr(dp, &negotiateErr, hostInfo.EnodeID, sp)

	// start download request negotiation, get the storage host sign
	hostRevisionSign, err := downloadRequestNegotiation(sp, downloadReq, downloadData)
	if err != nil {
		negotiateErr = err
		return
	}

	// update the download revision
	downloadRevision.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}

	// commit the revision
	negotiateErr = storageContractRevisionCommit(sp, downloadRevision, contract, downloadPrice)
	return
}

// downloadRequestValidation will validate the download request
func downloadRequestValidation(downloadReq storage.DownloadRequest) error {
	// validate if the request sector size is greater than the limited sector size
	if uint64(downloadReq.Sector.Offset)+uint64(downloadReq.Sector.Length) > storage.SectorSize {
		return fmt.Errorf("the requested download sector size exceed the limit %v", storage.SectorSize)
	}

	// validate the request sector
	if downloadReq.MerkleProof {
		if downloadReq.Sector.Offset%merkle.LeafSize != 0 || downloadReq.Sector.Length%merkle.LeafSize != 0 {
			return fmt.Errorf("sector's offset and length must be multiple of leaf size when requesting a merkle proof")
		}
	}

	return nil
}

// formDownloadContractRevision will form the new download contract for data download
func formDownloadContractRevision(contract *contractset.Contract, downloadPrice common.BigInt) (types.StorageContractRevision, error) {
	// get the latest contract revision
	latestRevision := contract.Header().LatestContractRevision

	// create new download revision
	downloadRevision := newContractRevision(latestRevision, downloadPrice.BigIntPtr())
	return downloadRevision, nil
}

// clientSignDownloadContractRevision will sign the storage contract download revision by storage client
func clientSignDownloadContractRevision(am storage.ClientAccountManager, downloadRevision types.StorageContractRevision) ([]byte, error) {
	// get the storage client's account and wallet
	clientAccount := accounts.Account{Address: downloadRevision.NewValidProofOutputs[0].Address}
	clientWallet, err := am.Find(clientAccount)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to find the client wallet: %s", err.Error())
	}

	// sign the contract revision
	clientRevisionSign, err := clientWallet.SignHash(clientAccount, downloadRevision.RLPHash().Bytes())
	if err != nil {
		return []byte{}, fmt.Errorf("client failed to sign the download contract revision: %s", err.Error())
	}

	return clientRevisionSign, nil
}

// updateDownloadRequest will update the download request
func updateDownloadRequest(downloadReq storage.DownloadRequest, clientRevisionSign []byte, downloadRevision types.StorageContractRevision) storage.DownloadRequest {
	// update download request
	downloadReq.Signature = clientRevisionSign[:]
	downloadReq.StorageContractID = downloadRevision.ParentID
	downloadReq.NewRevisionNumber = downloadRevision.NewRevisionNumber

	// update request's valid proof payback
	for _, payback := range downloadRevision.NewValidProofOutputs {
		downloadReq.NewValidProofValues = append(downloadReq.NewValidProofValues, payback.Value)
	}

	// update request's missed proof payback
	for _, payback := range downloadRevision.NewMissedProofOutputs {
		downloadReq.NewMissedProofValues = append(downloadReq.NewMissedProofValues, payback.Value)
	}

	return downloadReq
}

// downloadRequestNegotiation will negotiate the download request sent by storage client. If both
// sides reached to an agreement, then the host signed storage revision will be returned
func downloadRequestNegotiation(sp storage.Peer, downloadReq storage.DownloadRequest, downloadData io.Writer) ([]byte, error) {
	// send contract download request
	if err := sp.RequestContractDownload(downloadReq); err != nil {
		return []byte{}, err
	}

	// wait and handle the storage host download response
	downloadResp, err := waitAndHandleHostDownloadResp(sp, downloadReq.Sector, downloadReq.MerkleProof)
	if err != nil {
		return []byte{}, err
	}

	// write the sector data into the writer
	if _, err := downloadData.Write(downloadResp.Data); err != nil {
		err = fmt.Errorf("failed to write data into the writer: %s", err.Error())
		return []byte{}, common.ErrCompose(storage.ErrClientNegotiate, err)
	}

	return downloadResp.Signature, nil
}

// waitAndHandleHostDownloadResp will wait download response from the storage client, validate
// the response, and return host signed contract revision
func waitAndHandleHostDownloadResp(sp storage.Peer, sector storage.DownloadRequestSector, requestMerkleProof bool) (storage.DownloadResponse, error) {
	// wait for the host's response
	msg, err := sp.ClientWaitContractResp()
	if err != nil {
		return storage.DownloadResponse{}, fmt.Errorf("failed to wait for the host download response: %s", err.Error())
	}

	// check error message code
	if err := hostRespMsgCodeValidation(msg); err != nil {
		return storage.DownloadResponse{}, err
	}

	// decode the host's response
	var downloadResp storage.DownloadResponse
	if err := msg.Decode(&downloadResp); err != nil {
		err = fmt.Errorf("storage client failed to parse the host download response: %s", err.Error())
		return storage.DownloadResponse{}, common.ErrCompose(err, storage.ErrHostNegotiate)
	}

	// validate host's download response
	if err := hostDownloadRespValidation(downloadResp, sector, requestMerkleProof); err != nil {
		return storage.DownloadResponse{}, err
	}

	return downloadResp, nil
}

// hostDownloadRespValidation will validate the storage host's download response
func hostDownloadRespValidation(downloadResp storage.DownloadResponse, sector storage.DownloadRequestSector, requestMerkleProof bool) error {
	// check if the download response contains any data
	if len(downloadResp.Data) != int(sector.Length) {
		err := fmt.Errorf("download response from the host does not contain enough data sectors")
		return common.ErrCompose(storage.ErrHostNegotiate, err)
	}

	// validate the data
	if requestMerkleProof {
		proofStart := int(sector.Offset) / merkle.LeafSize
		proofEnd := int(sector.Offset+sector.Length) / merkle.LeafSize
		verified, err := merkle.Sha256VerifyRangeProof(downloadResp.Data, downloadResp.MerkleProof, proofStart, proofEnd, sector.MerkleRoot)
		if !verified || err != nil {
			err := fmt.Errorf("storage host provided incorrect data, failed to pass the merkle proof: %s", err.Error())
			return common.ErrCompose(storage.ErrHostNegotiate, err)
		}
	}

	// validate if the response contains any signatures
	if len(downloadResp.Signature) <= 0 {
		err := fmt.Errorf("storage host does not provide any signature in the response")
		return common.ErrCompose(storage.ErrHostNegotiate, err)
	}

	return nil
}

// calculateAndValidateSectorDownloadPrice will calculate the cost for downloading one sector of data
// and it will also check if the contract has enough fund to pay the download price
func calculateAndValidateSectorDownloadPrice(downloadReq storage.DownloadRequest, hostInfo storage.HostInfo, latestRevision types.StorageContractRevision) (common.BigInt, error) {
	// estimate the bandwidth needed for sector download
	var estHashesPerProof uint64
	if downloadReq.MerkleProof {
		estHashesPerProof = uint64(2 * bits.Len64(storage.SectorSize/storage.SegmentSize))
	}
	estBandwidth := uint64(downloadReq.Sector.Length) + uint64(estHashesPerProof)*uint64(storage.HashSize)

	// estimate the sector download price
	bandwidthPrice := hostInfo.DownloadBandwidthPrice.MultUint64(estBandwidth)
	downloadPrice := hostInfo.BaseRPCPrice.Add(bandwidthPrice).Add(hostInfo.SectorAccessPrice)

	// validate the download price
	if latestRevision.NewValidProofOutputs[0].Value.Cmp(downloadPrice.BigIntPtr()) < 0 {
		return common.BigInt0, fmt.Errorf("client does not have enough fund to download sector from the storage host")
	}

	// increase the price by 0.2% to mitigate small errors
	downloadPrice = downloadPrice.MultFloat64(1 + extraRatio)
	return downloadPrice, nil
}

func handleContractDownloadErr(dp clientnegotiation.DownloadProtocol, err *error, hostID enode.ID, sp storage.Peer) {
	handleNegotiationErr(dp, err, hostID, sp, storagehostmanager.InteractionDownload)
}
