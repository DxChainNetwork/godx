// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"github.com/DxChainNetwork/godx/storage/storageclient/memorymanager"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

var (
	zeroValue = new(big.Int).SetInt64(0)

	extraRatio = 0.02
)

// StorageClient contains fileds that are used to perform StorageHost
// selection operation, file uploading, downloading operations, and etc.
type StorageClient struct {
	fileSystem *filesystem.FileSystem

	// TODO (jacky): File Download Related

	// TODO (jacky): File Upload Related

	// Todo (jacky): File Recovery Related

	// Memory Management
	memoryManager *memorymanager.MemoryManager

	storageHostManager *storagehostmanager.StorageHostManager

	// Download management. The heap has a separate mutex because it is always
	// accessed in isolation.
	downloadHeapMu sync.Mutex           // Used to protect the downloadHeap.
	downloadHeap   *downloadSegmentHeap // A heap of priority-sorted segments to download.
	newDownloads   chan struct{}        // Used to notify download loop that new downloads are available.

	// List of workers that can be used for uploading and/or downloading.
	workerPool map[storage.ContractID]*worker

	// Cache the hosts from the last price estimation result
	lastEstimationStorageHost []StorageHostEntry

	// Directories and File related
	persist        persistence
	persistDir     string
	staticFilesDir string

	// Utilities
	log  log.Logger
	lock sync.Mutex
	tm   threadmanager.ThreadManager

	// information on network, block chain, and etc.
	info       ParsedAPI
	ethBackend storage.EthBackend
	apiBackend ethapi.Backend

	// get the P2P server for adding peer
	p2pServer *p2p.Server

	// file management.
	staticFileSet *dxfile.FileSet
}

// New initializes StorageClient object
func New(persistDir string) (*StorageClient, error) {
	sc := &StorageClient{
		// TODO(mzhang): replace the implemented contractor here
		fileSystem:     filesystem.New(persistDir, &filesystem.AlwaysSuccessContractManager{}),
		persistDir:     persistDir,
		staticFilesDir: filepath.Join(persistDir, DxPathRoot),
		log:            log.New(),
		newDownloads:   make(chan struct{}, 1),
		downloadHeap:   new(downloadSegmentHeap),
		workerPool:     make(map[storage.ContractID]*worker),
	}

	sc.memoryManager = memorymanager.New(DefaultMaxMemory, sc.tm.StopChan())
	sc.storageHostManager = storagehostmanager.New(sc.persistDir)

	return sc, nil
}

// Start controls go routine checking and updating process
func (sc *StorageClient) Start(b storage.EthBackend, server *p2p.Server, apiBackend ethapi.Backend) error {
	// get the eth backend
	sc.ethBackend = b
	sc.apiBackend = apiBackend

	// validation
	if server == nil {
		return errors.New("failed to get the P2P server")
	}

	// get the p2p server for the adding peers
	sc.p2pServer = server

	// getting all needed API functions
	err := sc.filterAPIs(b.APIs())
	if err != nil {
		return err
	}

	// TODO: (mzhang) Initialize ContractManager & HostManager -> assign to StorageClient
	err = sc.storageHostManager.Start(sc.p2pServer, sc)
	if err != nil {
		return err
	}

	// Load settings from persist file
	if err := sc.loadPersist(); err != nil {
		return err
	}

	// active the work pool to get a worker for a upload/download task.
	sc.activateWorkerPool()

	// loop to download
	go sc.downloadLoop()

	// kill workers on shutdown.
	sc.tm.OnStop(func() error {
		sc.lock.Lock()
		for _, worker := range sc.workerPool {
			close(worker.killChan)
		}
		sc.lock.Unlock()
		return nil
	})

	// TODO (mzhang): Subscribe consensus change

	if err = sc.fileSystem.Start(); err != nil {
		return err
	}

	// TODO (Jacky): Starting Worker, Checking file healthy, etc.

	// TODO (mzhang): Register On Stop Thread Control Function, waiting for WAL

	return nil
}

func (sc *StorageClient) Close() error {
	var fullErr error
	// Closing the host manager
	sc.log.Info("Closing the renter host manager")
	err := sc.storageHostManager.Close()
	fullErr = common.ErrCompose(fullErr, err)

	// Closing the file system
	sc.log.Info("Closing the renter file system")
	err = sc.fileSystem.Close()
	fullErr = common.ErrCompose(fullErr, err)

	// Closing the thread manager
	err = sc.tm.Stop()
	fullErr = common.ErrCompose(fullErr, err)
	return fullErr
}

func (sc *StorageClient) setBandwidthLimits(uploadSpeedLimit int64, downloadSpeedLimit int64) error {
	// validation
	if uploadSpeedLimit < 0 || downloadSpeedLimit < 0 {
		return errors.New("upload/download speed limit cannot be negative")
	}

	// Update the contract settings accordingly
	if uploadSpeedLimit == 0 && downloadSpeedLimit == 0 {
		// TODO (mzhang): update contract settings using contract manager
	} else {
		// TODO (mzhang): update contract settings to the loaded data
	}

	return nil
}

func (sc *StorageClient) ContractCreate(params ContractParams) error {
	// Extract vars from params, for convenience
	allowance, funding, clientPublicKey, startHeight, endHeight, host := params.Allowance, params.Funding, params.ClientPublicKey, params.StartHeight, params.EndHeight, params.Host

	// Calculate the payouts for the client, host, and whole contract
	period := endHeight - startHeight
	expectedStorage := allowance.ExpectedStorage / allowance.Hosts
	clientPayout, hostPayout, _, err := ClientPayoutsPreTax(host, funding, zeroValue, zeroValue, period, expectedStorage)
	if err != nil {
		return err
	}

	uc := types.UnlockConditions{
		PublicKeys: []ecdsa.PublicKey{
			clientPublicKey,
			host.PublicKey,
		},
		SignaturesRequired: 2,
	}

	clientAddr := crypto.PubkeyToAddress(clientPublicKey)
	hostAddr := crypto.PubkeyToAddress(host.PublicKey)

	// Create storage contract
	storageContract := types.StorageContract{
		FileSize:         0,
		FileMerkleRoot:   common.Hash{}, // no proof possible without data
		WindowStart:      endHeight,
		WindowEnd:        endHeight + host.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout}},
		UnlockHash:       uc.UnlockHash(),
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout, Address: clientAddr},
			// Deposit is returned to host
			{Value: hostPayout, Address: hostAddr},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout, Address: clientAddr},
			{Value: hostPayout, Address: hostAddr},
		},
	}

	// Increase Successful/Failed interactions accordingly
	defer func() {
		hostID := PubkeyToEnodeID(&host.PublicKey)
		if err != nil {
			sc.storageHostManager.IncrementFailedInteractions(hostID)
		} else {
			sc.storageHostManager.IncrementSuccessfulInteractions(hostID)
		}
	}()

	account := accounts.Account{Address: clientAddr}
	wallet, err := sc.ethBackend.AccountManager().Find(account)
	if err != nil {
		return storagehost.ExtendErr("find client account error", err)
	}

	// Setup connection with storage host
	session, err := sc.ethBackend.SetupConnection(host.NetAddress)
	if err != nil {
		return storagehost.ExtendErr("setup connection with host failed", err)
	}
	defer sc.ethBackend.Disconnect(host.NetAddress)

	clientContractSign, err := wallet.SignHash(account, storageContract.RLPHash().Bytes())
	if err != nil {
		return storagehost.ExtendErr("contract sign by client failed", err)
	}

	// Send the ContractCreate request
	req := storage.ContractCreateRequest{
		StorageContract: storageContract,
		Sign:            clientContractSign,
	}

	if err := session.SendStorageContractCreation(req); err != nil {
		return err
	}

	var hostSign []byte
	msg, err := session.ReadMsg()
	if err != nil {
		return err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return negotiationErr
	}

	if err := msg.Decode(&hostSign); err != nil {
		return err
	}

	storageContract.Signatures[0] = clientContractSign
	storageContract.Signatures[1] = hostSign

	// Assemble init revision and sign it
	storageContractRevision := types.StorageContractRevision{
		ParentID:              storageContract.RLPHash(),
		UnlockConditions:      uc,
		NewRevisionNumber:     1,
		NewFileSize:           storageContract.FileSize,
		NewFileMerkleRoot:     storageContract.FileMerkleRoot,
		NewWindowStart:        storageContract.WindowStart,
		NewWindowEnd:          storageContract.WindowEnd,
		NewValidProofOutputs:  storageContract.ValidProofOutputs,
		NewMissedProofOutputs: storageContract.MissedProofOutputs,
		NewUnlockHash:         storageContract.UnlockHash,
	}

	clientRevisionSign, err := wallet.SignHash(account, storageContractRevision.RLPHash().Bytes())
	if err != nil {
		return storagehost.ExtendErr("client sign revision error", err)
	}
	storageContractRevision.Signatures = [][]byte{clientRevisionSign}

	if err := session.SendStorageContractCreationClientRevisionSign(clientRevisionSign); err != nil {
		return storagehost.ExtendErr("send revision sign by client error", err)
	}

	var hostRevisionSign []byte
	msg, err = session.ReadMsg()
	if err != nil {
		return err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return negotiationErr
	}

	if err := msg.Decode(&hostRevisionSign); err != nil {
		return err
	}

	scBytes, err := rlp.EncodeToBytes(storageContract)
	if err != nil {
		return err
	}

	sendAPI := NewStorageContractTxAPI(sc.apiBackend)
	args := SendStorageContractTxArgs{
		From: clientAddr,
	}
	addr := common.Address{}
	addr.SetBytes([]byte{10})
	args.To = &addr
	args.Input = (*hexutil.Bytes)(&scBytes)
	ctx := context.Background()
	if _, err := sendAPI.SendFormContractTX(ctx, args); err != nil {
		return storagehost.ExtendErr("Send storage contract transaction error", err)
	}

	// wrap some information about this contract
	header := contractset.ContractHeader{
		ID:                     storage.ContractID(storageContract.ID()),
		EnodeID:                PubkeyToEnodeID(&host.PublicKey),
		StartHeight:            startHeight,
		EndHeight:              endHeight,
		TotalCost:              common.NewBigInt(funding.Int64()),
		ContractFee:            common.NewBigInt(host.ContractPrice.Int64()),
		LatestContractRevision: storageContractRevision,
		Status: storage.ContractStatus{
			UploadAbility: true,
			Canceled:      true,
			RenewAbility:  false,
		},
	}

	// store this contract info to client local
	_, err = sc.storageHostManager.GetStorageContractSet().InsertContract(header, nil)
	if err != nil {
		return err
	}
	return nil
}

func (sc *StorageClient) Append(session *storage.Session, data []byte) error {
	return sc.Write(session, []storage.UploadAction{{Type: storage.UploadActionAppend, Data: data}})
}

func (sc *StorageClient) Write(session *storage.Session, actions []storage.UploadAction) (err error) {

	// Retrieve the last contract revision
	contractRevision := types.StorageContractRevision{}
	scs := sc.storageHostManager.GetStorageContractSet()

	// find the contractID formed by this host
	hostInfo := session.HostInfo()
	contractID := scs.GetContractIDByHostID(hostInfo.EnodeID)
	contract, exist := scs.Acquire(contractID)
	if !exist {
		return errors.New(fmt.Sprintf("not exist this contract: %s", contractID.String()))
	}

	defer scs.Return(contract)
	contractHeader := contract.Header()
	contractRevision = contractHeader.LatestContractRevision

	// Calculate price per sector
	blockBytes := storage.SectorSize * uint64(contractRevision.NewWindowEnd-sc.ethBackend.GetCurrentBlockHeight())
	sectorBandwidthPrice := hostInfo.UploadBandwidthPrice.MultUint64(storage.SectorSize)
	sectorStoragePrice := hostInfo.StoragePrice.MultUint64(blockBytes)
	sectorDeposit := hostInfo.Deposit.MultUint64(blockBytes)

	// Calculate the new Merkle root set and total cost/collateral
	var bandwidthPrice, storagePrice, deposit *big.Int
	newFileSize := contractRevision.NewFileSize
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			bandwidthPrice = bandwidthPrice.Add(bandwidthPrice, sectorBandwidthPrice.BigIntPtr())
			newFileSize += storage.SectorSize
		}
	}

	if newFileSize > contractRevision.NewFileSize {
		addedSectors := (newFileSize - contractRevision.NewFileSize) / storage.SectorSize
		storagePrice = sectorStoragePrice.MultUint64(addedSectors).BigIntPtr()
		deposit = sectorDeposit.MultUint64(addedSectors).BigIntPtr()
	}

	// Estimate cost of Merkle proof
	proofSize := storage.HashSize * (128 + len(actions))
	bandwidthPrice = bandwidthPrice.Add(bandwidthPrice, hostInfo.DownloadBandwidthPrice.MultUint64(uint64(proofSize)).BigIntPtr())

	cost := new(big.Int).Add(bandwidthPrice.Add(bandwidthPrice, storagePrice), hostInfo.BaseRPCPrice.BigIntPtr())

	// Check that enough funds are available
	if contractRevision.NewValidProofOutputs[0].Value.Cmp(cost) < 0 {
		return errors.New("contract has insufficient funds to support upload")
	}
	if contractRevision.NewMissedProofOutputs[1].Value.Cmp(deposit) < 0 {
		return errors.New("contract has insufficient collateral to support upload")
	}

	// Create the revision; we will update the Merkle root later
	rev := NewRevision(contractRevision, cost)
	rev.NewMissedProofOutputs[1].Value = rev.NewMissedProofOutputs[1].Value.Sub(rev.NewMissedProofOutputs[1].Value, deposit)
	rev.NewFileSize = newFileSize

	// Create the request
	req := storage.UploadRequest{
		StorageContractID: contractRevision.ParentID,
		Actions:           actions,
		NewRevisionNumber: rev.NewRevisionNumber,
	}
	req.NewValidProofValues = make([]*big.Int, len(rev.NewValidProofOutputs))
	for i, o := range rev.NewValidProofOutputs {
		req.NewValidProofValues[i] = o.Value
	}
	req.NewMissedProofValues = make([]*big.Int, len(rev.NewMissedProofOutputs))
	for i, o := range rev.NewMissedProofOutputs {
		req.NewMissedProofValues[i] = o.Value
	}

	// record the change to this contract, that can allow us to continue this incomplete upload at last time
	walTxn, err := contract.RecordUploadPreRev(rev, common.Hash{}, common.NewBigInt(storagePrice.Int64()), common.NewBigInt(bandwidthPrice.Int64()))
	if err != nil {
		return err
	}

	defer func() {

		// Increase Successful/Failed interactions accordingly
		if err != nil {
			sc.storageHostManager.IncrementFailedInteractions(hostInfo.EnodeID)
		} else {
			sc.storageHostManager.IncrementSuccessfulInteractions(hostInfo.EnodeID)
		}

		// reset deadline
		session.SetDeadLine(time.Hour)
	}()

	// 1. Send storage upload request
	session.SetDeadLine(storage.ContractRevisionTime)
	if err := session.SendStorageContractUploadRequest(req); err != nil {
		return err
	}

	// 2. Read merkle proof response from host
	var merkleResp storage.UploadMerkleProof
	msg, err := session.ReadMsg()
	if err != nil {
		return err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return negotiationErr
	}

	if err := msg.Decode(&merkleResp); err != nil {
		return err
	}

	// Verify merkle proof
	numSectors := contractRevision.NewFileSize / storage.SectorSize
	proofRanges := CalculateProofRanges(actions, numSectors)
	proofHashes := merkleResp.OldSubtreeHashes
	leafHashes := merkleResp.OldLeafHashes
	oldRoot, newRoot := contractRevision.NewFileMerkleRoot, merkleResp.NewMerkleRoot
	verified, err := merkle.VerifyDiffProof(proofRanges, numSectors, proofHashes, leafHashes, oldRoot)
	if err != nil {
		sc.log.Error("something wrong for verifying diff proof", "error", err)
	}
	if !verified {
		return errors.New("invalid Merkle proof for old root")
	}

	// and then modify the leaves and verify the new Merkle root
	leafHashes = ModifyLeaves(leafHashes, actions, numSectors)
	proofRanges = ModifyProofRanges(proofRanges, actions, numSectors)
	verified, err = merkle.VerifyDiffProof(proofRanges, numSectors, proofHashes, leafHashes, newRoot)
	if err != nil {
		sc.log.Error("something wrong for verifying diff proof", "error", err)
	}
	if !verified {
		return errors.New("invalid Merkle proof for new root")
	}

	// Update the revision, sign it, and send it
	rev.NewFileMerkleRoot = newRoot

	// get client wallet
	am := sc.ethBackend.AccountManager()
	clientAddr := rev.NewValidProofOutputs[0].Address
	clientAccount := accounts.Account{Address: clientAddr}
	clientWallet, err := am.Find(clientAccount)
	if err != nil {
		return err
	}

	// client sign the new revision
	clientRevisionSign, err := clientWallet.SignHash(clientAccount, rev.RLPHash().Bytes())
	if err != nil {
		return err
	}
	rev.Signatures[0] = clientRevisionSign

	// send client sig to host
	if err := session.SendStorageContractUploadClientRevisionSign(clientRevisionSign); err != nil {
		return err
	}

	// Read the host's signature
	var hostRevisionSig []byte
	msg, err = session.ReadMsg()
	if err != nil {
		return err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return negotiationErr
	}

	if err := msg.Decode(&hostRevisionSig); err != nil {
		return err
	}

	rev.Signatures[1] = hostRevisionSig

	// commit upload revision
	err = contract.CommitUpload(walTxn, rev, common.Hash{}, common.NewBigInt(storagePrice.Int64()), common.NewBigInt(bandwidthPrice.Int64()))
	if err != nil {
		return err
	}

	return nil
}

// download

// calls the Read RPC, writing the requested data to w.
//
// NOTE: The RPC can be cancelled (with a granularity of one section) via the cancel channel.
func (client *StorageClient) Read(s *storage.Session, w io.Writer, req storage.DownloadRequest, cancel <-chan struct{}) (err error) {

	// reset deadline when finished.
	// if client has download the data, but not sent stopping or completing signal to host, the conn should be disconnected after 1 hour.
	defer s.SetDeadLine(time.Hour)

	// sanity check the request.
	for _, sec := range req.Sections {
		if uint64(sec.Offset)+uint64(sec.Length) > storage.SectorSize {
			return errors.New("illegal offset and/or length")
		}
		if req.MerkleProof {
			if sec.Offset%storage.SegmentSize != 0 || sec.Length%storage.SegmentSize != 0 {
				return errors.New("offset and length must be multiples of SegmentSize when requesting a Merkle proof")
			}
		}
	}

	// calculate estimated bandwidth
	var totalLength uint64
	for _, sec := range req.Sections {
		totalLength += uint64(sec.Length)
	}
	var estProofHashes uint64
	if req.MerkleProof {

		// use the worst-case proof size of 2*tree depth (this occurs when
		// proving across the two leaves in the center of the tree)
		estHashesPerProof := 2 * bits.Len64(storage.SectorSize/storage.SegmentSize)
		estProofHashes = uint64(len(req.Sections) * estHashesPerProof)
	}
	estBandwidth := totalLength + estProofHashes*uint64(storage.HashSize)
	if estBandwidth < storage.RPCMinLen {
		estBandwidth = storage.RPCMinLen
	}

	// calculate sector accesses
	sectorAccesses := make(map[common.Hash]struct{})
	for _, sec := range req.Sections {
		sectorAccesses[sec.MerkleRoot] = struct{}{}
	}

	// Retrieve the last contract revision
	lastRevision := types.StorageContractRevision{}
	scs := client.storageHostManager.GetStorageContractSet()

	// find the contractID formed by this host
	hostInfo := s.HostInfo()
	contractID := scs.GetContractIDByHostID(hostInfo.EnodeID)
	contract, exist := scs.Acquire(contractID)
	if !exist {
		return errors.New(fmt.Sprintf("not exist this contract: %s", contractID.String()))
	}

	defer scs.Return(contract)
	contractHeader := contract.Header()
	lastRevision = contractHeader.LatestContractRevision

	// calculate price
	bandwidthPrice := hostInfo.DownloadBandwidthPrice.MultUint64(estBandwidth)
	sectorAccessPrice := hostInfo.SectorAccessPrice.MultUint64(uint64(len(sectorAccesses)))

	price := hostInfo.BaseRPCPrice.Add(bandwidthPrice).Add(sectorAccessPrice)
	if lastRevision.NewValidProofOutputs[0].Value.Cmp(price.BigIntPtr()) < 0 {
		return errors.New("contract has insufficient funds to support download")
	}

	// To mitigate small errors (e.g. differing block heights), fudge the
	// price and collateral by 0.2%.
	price = price.MultFloat64(1 + extraRatio)

	// create the download revision and sign it
	newRevision := NewRevision(lastRevision, price.BigIntPtr())

	// client sign the revision
	am := client.ethBackend.AccountManager()
	account := accounts.Account{Address: newRevision.NewValidProofOutputs[0].Address}
	wallet, err := am.Find(account)
	if err != nil {
		return err
	}

	clientSig, err := wallet.SignHash(account, newRevision.RLPHash().Bytes())
	if err != nil {
		return err
	}
	newRevision.Signatures[0] = clientSig

	req.NewRevisionNumber = newRevision.NewRevisionNumber
	req.NewValidProofValues = make([]*big.Int, len(newRevision.NewValidProofOutputs))
	for i, nvpo := range newRevision.NewValidProofOutputs {
		req.NewValidProofValues[i] = nvpo.Value
	}
	req.NewMissedProofValues = make([]*big.Int, len(newRevision.NewMissedProofOutputs))
	for i, nmpo := range newRevision.NewMissedProofOutputs {
		req.NewMissedProofValues[i] = nmpo.Value
	}
	req.Signature = clientSig[:]

	// record the change to this contract
	walTxn, err := contract.RecordDownloadPreRev(newRevision, price)
	if err != nil {
		return err
	}

	// Increase Successful/Failed interactions accordingly
	defer func() {
		if err != nil {
			client.storageHostManager.IncrementFailedInteractions(hostInfo.EnodeID)
		} else {
			client.storageHostManager.IncrementSuccessfulInteractions(hostInfo.EnodeID)
		}
	}()

	// send download request
	s.SetDeadLine(storage.DownloadTime)
	err = s.SendStorageContractDownloadRequest(req)
	if err != nil {
		return err
	}

	// spawn a goroutine to handle cancellation
	doneChan := make(chan struct{})
	go func() {
		select {
		case <-cancel:
		case <-doneChan:
		}

		// if negotiation is canceled or done, client should send stop msg to host
		s.SendStopMsg()
	}()

	// ensure we send DownloadStop before returning
	defer close(doneChan)

	// read responses
	var hostSig []byte
	for _, sec := range req.Sections {
		var resp storage.DownloadResponse
		msg, err := s.ReadMsg()
		if err != nil {
			return err
		}

		// if host send some negotiation error, client should handler it
		if msg.Code == storage.NegotiationErrorMsg {
			var negotiationErr error
			msg.Decode(&negotiationErr)
			return negotiationErr
		}

		err = msg.Decode(&resp)
		if err != nil {
			return err
		}

		// The host may have sent data, a signature, or both. If they sent data, validate it.
		if len(resp.Data) > 0 {
			if len(resp.Data) != int(sec.Length) {
				return errors.New("host did not send enough sector data")
			}
			if req.MerkleProof {
				proofStart := int(sec.Offset) / storage.SegmentSize
				proofEnd := int(sec.Offset+sec.Length) / storage.SegmentSize
				verified, err := merkle.VerifyRangeProof(resp.Data, resp.MerkleProof, proofStart, proofEnd, sec.MerkleRoot)
				if err != nil {
					client.log.Error("something wrong for verifying range proof", "error", err)
				}
				if !verified {
					return errors.New("host provided incorrect sector data or Merkle proof")
				}
			}

			// write sector data
			if _, err := w.Write(resp.Data); err != nil {
				return err
			}
		}

		// If the host sent a signature, exit the loop; they won't be sending any more data
		if len(resp.Signature) > 0 {
			hostSig = resp.Signature
			break
		}
	}
	if hostSig == nil {

		// the host is required to send a signature; if they haven't sent one
		// yet, they should send an empty response containing just the signature.
		var resp storage.DownloadResponse
		msg, err := s.ReadMsg()
		if err != nil {
			return err
		}

		// if host send some negotiation error, client should handler it
		if msg.Code == storage.NegotiationErrorMsg {
			var negotiationErr error
			msg.Decode(&negotiationErr)
			return negotiationErr
		}

		err = msg.Decode(&resp)
		if err != nil {
			return err
		}

		hostSig = resp.Signature
	}
	newRevision.Signatures[1] = hostSig

	// commit this revision
	err = contract.CommitDownload(walTxn, newRevision, price)
	if err != nil {
		return err
	}

	return nil
}

// calls the Read RPC with a single section and returns the requested data. A Merkle proof is always requested.
func (client *StorageClient) Download(s *storage.Session, root common.Hash, offset, length uint32) ([]byte, error) {
	client.lock.Lock()
	defer client.lock.Unlock()

	req := storage.DownloadRequest{
		Sections: []storage.DownloadRequestSection{{
			MerkleRoot: root,
			Offset:     offset,
			Length:     length,
		}},
		MerkleProof: true,
	}
	var buf bytes.Buffer
	err := client.Read(s, &buf, req, nil)
	return buf.Bytes(), err
}

// newDownload creates and initializes a download task based on the provided parameters from outer request
func (client *StorageClient) newDownload(params downloadParams) (*download, error) {

	// params validation.
	if params.file == nil {
		return nil, errors.New("no file provided when requesting download")
	}
	if params.length < 0 {
		return nil, errors.New("download length must be zero or a positive whole number")
	}
	if params.offset < 0 {
		return nil, errors.New("download offset cannot be a negative number")
	}
	if params.offset+params.length > params.file.FileSize() {
		return nil, errors.New("download is requesting data past the boundary of the file")
	}

	// instantiate the download object.
	d := &download{
		completeChan:      make(chan struct{}),
		startTime:         time.Now(),
		destination:       params.destination,
		destinationString: params.destinationString,
		destinationType:   params.destinationType,
		latencyTarget:     params.latencyTarget,
		length:            params.length,
		offset:            params.offset,
		overdrive:         params.overdrive,
		dxFilePath:        params.file.DxPath(),
		priority:          params.priority,
		log:               client.log,
		memoryManager:     client.memoryManager,
	}

	// set the end time of the download when it's done.
	d.onComplete(func(_ error) error {
		d.endTime = time.Now()
		return nil
	})

	// nothing more to do for 0-byte files or 0-length downloads.
	if d.length == 0 {
		d.markComplete()
		return d, nil
	}

	// determine which segments to download.
	minSegment, minSegmentOffset := params.file.SegmentIndexByOffset(params.offset)
	maxSegment, maxSegmentOffset := params.file.SegmentIndexByOffset(params.offset + params.length)

	if maxSegment > 0 && maxSegmentOffset == 0 {
		maxSegment--
	}

	// check the requested segment number
	if minSegment == params.file.NumSegments() || maxSegment == params.file.NumSegments() {
		return nil, errors.New("download is requesting a segment that is past the boundary of the file")
	}

	// map from the host id to the index of the sector within the segment.
	segmentMaps := make([]map[string]downloadSectorInfo, maxSegment-minSegment+1)
	for segmentIndex := minSegment; segmentIndex <= maxSegment; segmentIndex++ {
		segmentMaps[segmentIndex-minSegment] = make(map[string]downloadSectorInfo)
		sectors, err := params.file.Sectors(uint64(segmentIndex))
		if err != nil {
			return nil, err
		}
		for sectorIndex, sectorSet := range sectors {
			for _, sector := range sectorSet {

				// sanity check - a worker should not have two sectors for the same segment.
				_, exists := segmentMaps[segmentIndex-minSegment][sector.HostID.String()]
				if exists {
					client.log.Error("ERROR: Worker has multiple sectors uploaded for the same segment.")
				}
				segmentMaps[segmentIndex-minSegment][sector.HostID.String()] = downloadSectorInfo{
					index: uint64(sectorIndex),
					root:  sector.MerkleRoot,
				}
			}
		}
	}

	// where to write a segment within the download destination
	writeOffset := int64(0)
	d.segmentsRemaining += maxSegment - minSegment + 1

	// queue the downloads for each segment.
	for i := minSegment; i <= maxSegment; i++ {
		uds := &unfinishedDownloadSegment{
			destination:  params.destination,
			erasureCode:  params.file.ErasureCode(),
			masterKey:    params.file.CipherKey(),
			segmentIndex: i,
			segmentMap:   segmentMaps[i-minSegment],
			segmentSize:  params.file.SegmentSize(),
			sectorSize:   params.file.SectorSize(),

			// increase target by 25ms per segment
			latencyTarget:       params.latencyTarget + (25 * time.Duration(i-minSegment)),
			needsMemory:         params.needsMemory,
			priority:            params.priority,
			completedSectors:    make([]bool, params.file.ErasureCode().NumSectors()),
			physicalSegmentData: make([][]byte, params.file.ErasureCode().NumSectors()),
			sectorUsage:         make([]bool, params.file.ErasureCode().NumSectors()),
			download:            d,
			clientFile:          params.file,
		}

		// set the offset within the segment that we start downloading from
		if i == minSegment {
			uds.fetchOffset = minSegmentOffset
		} else {
			uds.fetchOffset = 0
		}

		// set the number of bytes to fetch within the segment that we start downloading from
		if i == maxSegment && maxSegmentOffset != 0 {
			uds.fetchLength = maxSegmentOffset - uds.fetchOffset
		} else {
			uds.fetchLength = params.file.SegmentSize() - uds.fetchOffset
		}

		// set the writeOffset within the destination for where the data be written.
		uds.writeOffset = writeOffset
		writeOffset += int64(uds.fetchLength)

		uds.overdrive = uint32(params.overdrive)

		// add this segment to the segment heap, and notify the download loop a new task
		client.addSegmentToDownloadHeap(uds)
		select {
		case client.newDownloads <- struct{}{}:
		default:
		}
	}
	return d, nil
}

// managedDownload performs a file download and returns the download object
func (client *StorageClient) managedDownload(p storage.ClientDownloadParameters) (*download, error) {
	entry, err := client.staticFileSet.Open(p.DxFilePath)
	if err != nil {
		return nil, err
	}

	defer entry.Close()
	defer entry.SetTimeAccess(time.Now())

	// validate download parameters.
	isHTTPResp := p.HttpWriter != nil
	if p.Async && isHTTPResp {
		return nil, errors.New("cannot async download to http response")
	}
	if isHTTPResp && p.Destination != "" {
		return nil, errors.New("destination cannot be specified when downloading to http response")
	}
	if !isHTTPResp && p.Destination == "" {
		return nil, errors.New("destination not supplied")
	}
	if p.Destination != "" && !filepath.IsAbs(p.Destination) {
		return nil, errors.New("destination must be an absolute path")
	}

	if p.Offset == entry.FileSize() && entry.FileSize() != 0 {
		return nil, errors.New("offset equals filesize")
	}

	// if length == 0, download the rest file.
	if p.Length == 0 {
		if p.Offset > entry.FileSize() {
			return nil, errors.New("offset cannot be greater than file size")
		}
		p.Length = entry.FileSize() - p.Offset
	}

	// check whether offset and length is valid
	if p.Offset < 0 || p.Offset+p.Length > entry.FileSize() {
		return nil, fmt.Errorf("offset and length combination invalid, max byte is at index %d", entry.FileSize()-1)
	}

	// instantiate the correct downloadWriter implementation
	var dw downloadDestination
	var destinationType string
	if isHTTPResp {
		dw = newDownloadWriter(p.HttpWriter)
		destinationType = "http stream"
	} else {
		osFile, err := os.OpenFile(p.Destination, os.O_CREATE|os.O_WRONLY, entry.FileMode())
		if err != nil {
			return nil, err
		}
		dw = osFile
		destinationType = "file"
	}

	if isHTTPResp {
		w, ok := p.HttpWriter.(http.ResponseWriter)
		if ok {
			w.Header().Set("Content-Length", fmt.Sprint(p.Length))
		}
	}

	// create the download object.
	d, err := client.newDownload(downloadParams{
		destination:       dw,
		destinationType:   destinationType,
		destinationString: p.Destination,
		file:              entry.DxFile.Snapshot(),
		latencyTarget:     25e3 * time.Millisecond,
		length:            p.Length,
		needsMemory:       true,
		offset:            p.Offset,
		overdrive:         3,
		priority:          5,
	})
	if closer, ok := dw.(io.Closer); err != nil && ok {
		closeErr := closer.Close()
		if closeErr != nil {
			return nil, errors.New(fmt.Sprintf("get something wrong when create download object: %v, destination close error: %v", err, closeErr))
		}
		return nil, errors.New(fmt.Sprintf("get something wrong when create download object: %v, destination close successfully", err))
	} else if err != nil {
		return nil, err
	}

	// register some cleanup for when the download is done.
	d.onComplete(func(_ error) error {
		if closer, ok := dw.(io.Closer); ok {
			return closer.Close()
		}
		return nil
	})

	return d, nil
}

// NOTE: DownloadSync and DownloadAsync can directly be accessed to outer request via RPC or IPC ...

// performs a file download and blocks until the download is finished.
func (client *StorageClient) DownloadSync(p storage.ClientDownloadParameters) error {
	if err := client.tm.Add(); err != nil {
		return err
	}
	defer client.tm.Done()

	d, err := client.managedDownload(p)
	if err != nil {
		return err
	}

	// block until the download has completed
	select {
	case <-d.completeChan:
		return d.Err()
	case <-client.tm.StopChan():
		return errors.New("download interrupted by shutdown")
	}
}

// performs a file download without blocking until the download is finished
func (client *StorageClient) DownloadAsync(p storage.ClientDownloadParameters) error {
	if err := client.tm.Add(); err != nil {
		return err
	}
	defer client.tm.Done()

	_, err := client.managedDownload(p)
	return err
}
