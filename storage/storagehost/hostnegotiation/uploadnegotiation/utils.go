// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"

	"github.com/DxChainNetwork/godx/storage"
)

func waitAndHandleClientRevSignResp(sp storage.Peer) ([]byte, error) {
	// wait for client response
	var clientRevisionSign []byte
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		err = fmt.Errorf("storage host failed to wait for client contract reivision sign: %s", err.Error())
		return []byte{}, err
	}

	// check for error message code
	if msg.Code == storage.ClientNegotiateErrorMsg {
		return []byte{}, storage.ErrClientNegotiate
	}

	// decode the message
	if err = msg.Decode(&clientRevisionSign); err != nil {
		err = fmt.Errorf("failed to decode client revision sign: %s", err.Error())
		return []byte{}, err
	}

	return clientRevisionSign, nil
}

func waitAndHandleClientCommitRespUpload(sp storage.Peer, np hostnegotiation.Protocol, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility, hostConfig storage.HostIntConfig, newRev types.StorageContractRevision) error {
	// wait for storage host's response
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		return fmt.Errorf("waitAndHandleClientCommitRespUpload failed, host falied to wait for the client's response: %s", err.Error())
	}

	// based on the message code, handle the client's upload commit response
	if err := handleClientUploadCommitResp(msg, sp, np, session, sr, hostConfig, newRev); err != nil {
		return err
	}

	return nil
}

// handleClientUploadCommitResp will handle client's response based on the message code
func handleClientUploadCommitResp(msg p2p.Msg, sp storage.Peer, np hostnegotiation.Protocol, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility, hostConfig storage.HostIntConfig, newRev types.StorageContractRevision) error {
	switch msg.Code {
	case storage.ClientCommitSuccessMsg:
		return handleClientUploadSuccessCommit(sp, np, session, sr, hostConfig, newRev)
	case storage.ClientCommitFailedMsg:
		return storage.ErrClientCommit
	case storage.ClientNegotiateErrorMsg:
		return storage.ErrClientNegotiate
	default:
		return fmt.Errorf("failed to reconize the message code")
	}
}

func handleClientUploadSuccessCommit(sp storage.Peer, np hostnegotiation.Protocol, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility, hostConfig storage.HostIntConfig, newRev types.StorageContractRevision) error {
	// update and modify the storage responsibility
	sr = updateStorageResponsibilityUpload(session, sr, hostConfig, newRev)
	if err := np.ModifyStorageResponsibility(sr, nil, session.SectorRootsGained, session.SectorDataGained); err != nil {
		return storage.ErrHostCommit
	}

	// if the storage host successfully commit the storage responsibility, set the connection to be static
	np.CheckAndSetStaticConnection(sp)

	// at the end, send the storage host ack message
	if err := sp.SendHostAckMsg(); err != nil {
		_ = np.RollbackUploadStorageResponsibility(session.SrSnapshot, session.SectorRootsGained, nil, nil)
		return fmt.Errorf("failed to send the host ack message at the end during the upload, negotiation failed: %s", err.Error())
	}

	return nil
}

// updateStorageResponsibilityUpload updates the storage responsibility
func updateStorageResponsibilityUpload(session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility, hostConfig storage.HostIntConfig, newRev types.StorageContractRevision) storagehost.StorageResponsibility {
	// calculate the bandwidthRevenue after added merkle proof
	bandwidthRevenue := calcBandwidthRevenueWithProof(session, len(session.MerkleProof.OldSubtreeHashes), len(session.MerkleProof.OldLeafHashes), hostConfig.DownloadBandwidthPrice)

	// update the storage responsibility
	sr.SectorRoots = session.SectorRoots
	sr.PotentialStorageRevenue = sr.PotentialStorageRevenue.Add(session.StorageRevenue)
	sr.RiskedStorageDeposit = sr.RiskedStorageDeposit.Add(session.NewDeposit)
	sr.PotentialUploadRevenue = sr.PotentialUploadRevenue.Add(bandwidthRevenue)
	sr.StorageContractRevisions = append(sr.StorageContractRevisions, newRev)

	// return the updated storage responsibility
	return sr
}
