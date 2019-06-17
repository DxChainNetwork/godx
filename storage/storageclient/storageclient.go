// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"github.com/DxChainNetwork/godx/storage/storageclient/memorymanager"
	"github.com/DxChainNetwork/godx/storage/storageclient/proto"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

var (
	zeroValue  = new(big.Int).SetInt64(0)
	extraRatio = 0.02
)

// StorageClient contains fields that are used to perform StorageHost
// selection operation, file uploading, downloading operations, and etc.
type StorageClient struct {
	fileSystem *filesystem.FileSystem

	// TODO (jacky): File Download Related

	// TODO (jacky): File Upload Related

	// Todo (jacky): File Recovery Related

	// Memory Management
	memoryManager *memorymanager.MemoryManager

	storageHostManager *storagehostmanager.StorageHostManager
	contractManager    *contractmanager.ContractManager

	// Download management
	downloadHeapMu sync.Mutex
	downloadHeap   *downloadSegmentHeap
	newDownloads   chan struct{}

	// List of workers that can be used for uploading and/or downloading.
	workerPool map[storage.ContractID]*worker

	// Cache the hosts from the last price estimation result
	lastEstimationStorageHost []proto.StorageHostEntry

	// Directories and File related
	persist        persistence
	persistDir     string
	staticFilesDir string

	//storage client is used as the address to sign the storage contract and pays for the money
	PaymentAddress common.Address

	// Utilities
	log  log.Logger
	lock sync.Mutex
	tm   threadmanager.ThreadManager

	// information on network, block chain, and etc.
	info       storage.ParsedAPI
	ethBackend storage.EthBackend
	apiBackend ethapi.Backend

	// get the P2P server for adding peer
	p2pServer *p2p.Server

	// file management.
	staticFileSet *dxfile.FileSet
}

// New initializes StorageClient object
func New(persistDir string) (*StorageClient, error) {
	var err error

	// TODO: data initialization(file management system, file upload, file download)

	sc := &StorageClient{
		fileSystem:     filesystem.New(persistDir, &filesystem.AlwaysSuccessContractManager{}),
		persistDir:     persistDir,
		staticFilesDir: filepath.Join(persistDir, DxPathRoot),
		log:            log.New(),
		newDownloads:   make(chan struct{}, 1),
		downloadHeap:   new(downloadSegmentHeap),
		workerPool:     make(map[storage.ContractID]*worker),
	}

	sc.memoryManager = memorymanager.New(DefaultMaxMemory, sc.tm.StopChan())

	// initialize storageHostManager
	sc.storageHostManager = storagehostmanager.New(sc.persistDir)

	// initialize storage contract manager
	if sc.contractManager, err = contractmanager.New(sc.persistDir, sc.storageHostManager); err != nil {
		err = fmt.Errorf("error initializing contract manager: %s", err.Error())
		return nil, err
	}

	return sc, nil
}

// Start controls go routine checking and updating process
func (sc *StorageClient) Start(b storage.EthBackend, apiBackend ethapi.Backend) (err error) {
	// get the eth backend
	sc.ethBackend = b

	// getting all needed API functions
	if err = storage.FilterAPIs(b.APIs(), &sc.info); err != nil {
		return
	}

	// start storageHostManager
	if err = sc.storageHostManager.Start(sc); err != nil {
		return
	}

	// start contractManager
	if err = sc.contractManager.Start(sc); err != nil {
		err = fmt.Errorf("error starting contract manager: %s", err.Error())
		return
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

	sc.log.Info("storage client started")

	return nil
}

func (sc *StorageClient) Close() error {
	sc.log.Info("Closing The Contract Manager")
	sc.contractManager.Stop()

	var fullErr error

	// Closing the host manager
	sc.log.Info("Closing The Renter Host Manager")
	err := sc.storageHostManager.Close()
	fullErr = common.ErrCompose(fullErr, err)

	// Closing the file system
	sc.log.Info("Closing the renter file system")
	err = sc.fileSystem.Close()
	fullErr = common.ErrCompose(fullErr, err)

	// Closing the thread manager
	sc.log.Info("Closing The Storage Client Manager")
	err = sc.tm.Stop()
	fullErr = common.ErrCompose(fullErr, err)
	return fullErr
}

// ContractDetail will return the detailed contract information
func (sc *StorageClient) ContractDetail(contractID storage.ContractID) (detail storage.ContractMetaData, exists bool) {
	return sc.contractManager.RetrieveActiveContract(contractID)
}

// ActiveContracts will retrieve all active contracts, reformat them, and return them back
func (sc *StorageClient) ActiveContracts() (activeContracts []ActiveContractsAPIDisplay) {
	allActiveContracts := sc.contractManager.RetrieveActiveContracts()

	for _, contract := range allActiveContracts {
		activeContract := ActiveContractsAPIDisplay{
			ContractID:   contract.ID.String(),
			HostID:       contract.EnodeID.String(),
			AbleToUpload: contract.Status.UploadAbility,
			AbleToRenew:  contract.Status.RenewAbility,
			Canceled:     contract.Status.Canceled,
		}
		activeContracts = append(activeContracts, activeContract)
	}

	return
}

func (sc *StorageClient) CancelContracts() (err error) {
	return sc.contractManager.CancelStorageContract()
}

// SetClientSetting will config the client setting based on the value provided
// it will set the bandwidth limit, rentPayment, and ipViolation check
// By setting the rentPayment, the contract maintenance
func (sc *StorageClient) SetClientSetting(setting storage.ClientSetting) (err error) {
	// making sure the entire program will only be terminated after finish the SetClientSetting
	// operation

	if err = sc.tm.Add(); err != nil {
		return
	}
	defer sc.tm.Done()

	// input validation
	if setting.MaxUploadSpeed < 0 || setting.MaxDownloadSpeed < 0 {
		err = fmt.Errorf("both upload speed %v and download speed %v cannot be smaller than 0",
			setting.MaxUploadSpeed, setting.MaxDownloadSpeed)
	}

	// set the rent payment
	if err = sc.contractManager.SetRentPayment(setting.RentPayment); err != nil {
		return
	}

	// set upload/download (write/read) bandwidth limits
	if err = sc.setBandwidthLimits(setting.MaxDownloadSpeed, setting.MaxUploadSpeed); err != nil {
		return
	}

	// set the ip violation check
	sc.storageHostManager.SetIPViolationCheck(setting.EnableIPViolation)

	// update and save the persist
	sc.persist.MaxDownloadSpeed = setting.MaxDownloadSpeed
	sc.persist.MaxUploadSpeed = setting.MaxUploadSpeed
	if err = sc.saveSettings(); err != nil {
		err = fmt.Errorf("failed to save the storage client settigns: %s", err.Error())
	}

	// active the worker pool
	sc.activateWorkerPool()

	return
}

// RetrieveClientSetting will return the current storage client setting
func (sc *StorageClient) RetrieveClientSetting() (setting storage.ClientSetting) {
	maxDownloadSpeed, maxUploadSpeed, _ := sc.contractManager.RetrieveRateLimit()
	setting = storage.ClientSetting{
		RentPayment:       sc.contractManager.AcquireRentPayment(),
		EnableIPViolation: sc.storageHostManager.RetrieveIPViolationCheckSetting(),
		MaxUploadSpeed:    maxUploadSpeed,
		MaxDownloadSpeed:  maxDownloadSpeed,
	}
	return
}

// setBandwidthLimits specifies the data upload and downloading speed limit
func (sc *StorageClient) setBandwidthLimits(downloadSpeedLimit, uploadSpeedLimit int64) (err error) {
	// validation
	if uploadSpeedLimit < 0 || downloadSpeedLimit < 0 {
		return errors.New("upload/download speed limit cannot be negative")
	}

	// Update the contract settings accordingly
	if uploadSpeedLimit == 0 && downloadSpeedLimit == 0 {
		sc.contractManager.SetRateLimits(0, 0, 0)
	} else {
		sc.contractManager.SetRateLimits(downloadSpeedLimit, uploadSpeedLimit, DefaultPacketSize)
	}

	return nil
}

func (sc *StorageClient) Append(session *storage.Session, data []byte) error {
	return sc.Write(session, []storage.UploadAction{{Type: storage.UploadActionAppend, Data: data}})
}

func (sc *StorageClient) Write(session *storage.Session, actions []storage.UploadAction) (err error) {

	// retrieve the last contract revision
	contractRevision := types.StorageContractRevision{}
	scs := sc.contractManager.GetStorageContractSet()

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

	// calculate price per sector
	blockBytes := storage.SectorSize * uint64(contractRevision.NewWindowEnd-sc.ethBackend.GetCurrentBlockHeight())
	sectorBandwidthPrice := hostInfo.UploadBandwidthPrice.MultUint64(storage.SectorSize)
	sectorStoragePrice := hostInfo.StoragePrice.MultUint64(blockBytes)
	sectorDeposit := hostInfo.Deposit.MultUint64(blockBytes)

	// calculate the new Merkle root set and total cost/collateral
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

	// estimate cost of Merkle proof
	proofSize := storage.HashSize * (128 + len(actions))
	bandwidthPrice = bandwidthPrice.Add(bandwidthPrice, hostInfo.DownloadBandwidthPrice.MultUint64(uint64(proofSize)).BigIntPtr())

	cost := new(big.Int).Add(bandwidthPrice.Add(bandwidthPrice, storagePrice), hostInfo.BaseRPCPrice.BigIntPtr())

	// check that enough funds are available
	if contractRevision.NewValidProofOutputs[0].Value.Cmp(cost) < 0 {
		return errors.New("contract has insufficient funds to support upload")
	}
	if contractRevision.NewMissedProofOutputs[1].Value.Cmp(deposit) < 0 {
		return errors.New("contract has insufficient collateral to support upload")
	}

	// create the revision; we will update the Merkle root later
	rev := NewRevision(contractRevision, cost)
	rev.NewMissedProofOutputs[1].Value = rev.NewMissedProofOutputs[1].Value.Sub(rev.NewMissedProofOutputs[1].Value, deposit)
	rev.NewFileSize = newFileSize

	// create the request
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

		// record the successful or failed interactions
		if err != nil {
			sc.storageHostManager.IncrementFailedInteractions(hostInfo.EnodeID)
		} else {
			sc.storageHostManager.IncrementSuccessfulInteractions(hostInfo.EnodeID)
		}

		// reset deadline
		session.SetDeadLine(time.Hour)
	}()

	// 1. send storage upload request
	session.SetDeadLine(storage.ContractRevisionTime)
	if err := session.SendStorageContractUploadRequest(req); err != nil {
		return err
	}

	// 2. read merkle proof response from host
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

	// verify merkle proof
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

	// update the revision, sign it, and send it
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

	// read the host's signature
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
	// NOTE: if client has download the data, but not sent stopping or completing signal to host,
	// the conn should be disconnected after 1 hour.
	defer s.SetDeadLine(time.Hour)

	// sanity check the request.
	for _, sec := range req.Sections {
		if uint64(sec.Offset)+uint64(sec.Length) > storage.SectorSize {
			return errors.New("download out boundary of sector")
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

		// use the worst-case proof size of 2*tree depth,
		// which occurs when proving across the two leaves in the center of the tree
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

	// retrieve the last contract revision
	lastRevision := types.StorageContractRevision{}
	scs := client.contractManager.GetStorageContractSet()

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
		return errors.New("client funds not enough to support download")
	}

	// increase the price fluctuation by 0.2% to mitigate small errors, like different block height
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
	req.Signature = clientSig[:]
	req.StorageContractID = newRevision.ParentID
	req.NewRevisionNumber = newRevision.NewRevisionNumber

	req.NewValidProofValues = make([]*big.Int, len(newRevision.NewValidProofOutputs))
	for i, nvpo := range newRevision.NewValidProofOutputs {
		req.NewValidProofValues[i] = nvpo.Value
	}

	req.NewMissedProofValues = make([]*big.Int, len(newRevision.NewMissedProofOutputs))
	for i, nmpo := range newRevision.NewMissedProofOutputs {
		req.NewMissedProofValues[i] = nmpo.Value
	}

	// record the change to this contract
	walTxn, err := contract.RecordDownloadPreRev(newRevision, price)
	if err != nil {
		return err
	}

	// record the successful or failed interactions
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

		// if host sent data, should validate it
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

		// if host sent signature, indicate the download complete, should exit the loop
		if len(resp.Signature) > 0 {
			hostSig = resp.Signature
			break
		}
	}
	if hostSig == nil {

		// if we haven't received host signature, just read again
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
		return nil, errors.New("not exist the remote file")
	}
	if params.length < 0 {
		return nil, errors.New("download length cannot be negative")
	}
	if params.offset < 0 {
		return nil, errors.New("download offset cannot be negative")
	}
	if params.offset+params.length > params.file.FileSize() {
		return nil, errors.New("download data out the boundary of the remote file")
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

	// record the end time when it's done.
	d.onComplete(func(_ error) error {
		d.endTime = time.Now()
		return nil
	})

	// nothing to do
	if d.length == 0 {
		d.markComplete()
		return d, nil
	}

	// calculate which segments to download
	minSegment, minSegmentOffset := params.file.SegmentIndexByOffset(params.offset)
	maxSegment, maxSegmentOffset := params.file.SegmentIndexByOffset(params.offset + params.length)

	if maxSegment > 0 && maxSegmentOffset == 0 {
		maxSegment--
	}

	// check the requested segment number
	if minSegment == params.file.NumSegments() || maxSegment == params.file.NumSegments() {
		return nil, errors.New("download segment out the boundary of the remote file")
	}

	// map from the host id to the index of the sector within the segment
	segmentMaps := make([]map[string]downloadSectorInfo, maxSegment-minSegment+1)
	for segmentIndex := minSegment; segmentIndex <= maxSegment; segmentIndex++ {
		segmentMaps[segmentIndex-minSegment] = make(map[string]downloadSectorInfo)
		sectors, err := params.file.Sectors(uint64(segmentIndex))
		if err != nil {
			return nil, err
		}
		for sectorIndex, sectorSet := range sectors {
			for _, sector := range sectorSet {

				// check that a worker should not have two sectors for the same segment
				_, exists := segmentMaps[segmentIndex-minSegment][sector.HostID.String()]
				if exists {
					client.log.Error("a worker has multiple sectors for the same segment")
				}
				segmentMaps[segmentIndex-minSegment][sector.HostID.String()] = downloadSectorInfo{
					index: uint64(sectorIndex),
					root:  sector.MerkleRoot,
				}
			}
		}
	}

	// record where to write every segment
	writeOffset := int64(0)

	// record how many segments remained after every downloading
	d.segmentsRemaining += maxSegment - minSegment + 1

	// queue the downloads for each segment
	for i := minSegment; i <= maxSegment; i++ {
		uds := &unfinishedDownloadSegment{
			destination:  params.destination,
			erasureCode:  params.file.ErasureCode(),
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

		// set the offset of the segment to begin downloading
		if i == minSegment {
			uds.fetchOffset = minSegmentOffset
		} else {
			uds.fetchOffset = 0
		}

		// set the number of bytes to download the segment
		if i == maxSegment && maxSegmentOffset != 0 {
			uds.fetchLength = maxSegmentOffset - uds.fetchOffset
		} else {
			uds.fetchLength = params.file.SegmentSize() - uds.fetchOffset
		}

		// set the writeOffset where the data be written
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

// createDownload performs a file download and returns the download object
func (client *StorageClient) createDownload(p storage.DownloadParameters) (*download, error) {
	dxPath := storage.DxPath{p.RemoteFilePath}
	entry, err := client.fileSystem.OpenFile(dxPath)
	if err != nil {
		return nil, err
	}

	defer entry.Close()
	defer entry.SetTimeAccess(time.Now())

	// validate download parameters.
	if p.WriteToLocalPath == "" {
		return nil, errors.New("not specified local path")
	}
	if p.WriteToLocalPath != "" && !filepath.IsAbs(p.WriteToLocalPath) {
		return nil, errors.New("local path must be absolute")
	}

	// instantiate the file to write the downloaded data
	var dw writeDestination
	var destinationType string
	osFile, err := os.OpenFile(p.WriteToLocalPath, os.O_CREATE|os.O_WRONLY, entry.FileMode())
	if err != nil {
		return nil, err
	}
	dw = osFile
	destinationType = "file"

	// create the download object.
	d, err := client.newDownload(downloadParams{
		destination:       dw,
		destinationType:   destinationType,
		destinationString: p.WriteToLocalPath,
		file:              entry.DxFile.Snapshot(),
		latencyTarget:     25e3 * time.Millisecond,

		// always download the whole file
		length:      entry.FileSize(),
		needsMemory: true,

		// always download from 0
		offset:    0,
		overdrive: 3,
		priority:  5,
	})
	if closer, ok := dw.(io.Closer); err != nil && ok {
		closeErr := closer.Close()
		if closeErr != nil {
			return nil, errors.New(fmt.Sprintf("something wrong with creating download object: %v, destination close error: %v", err, closeErr))
		}
		return nil, errors.New(fmt.Sprintf("get something wrong with creating download object: %v, destination close successfully", err))
	} else if err != nil {
		return nil, err
	}

	// register the func, and run it when download is done.
	d.onComplete(func(_ error) error {
		if closer, ok := dw.(io.Closer); ok {
			return closer.Close()
		}
		return nil
	})

	return d, nil
}

// NOTE: DownloadSync can directly be accessed to outer request via RPC or IPC ...
// but can not async download to http response, so DownloadAsync should not open to out.

// performs a file download and blocks until the download is finished.
func (client *StorageClient) DownloadSync(p storage.DownloadParameters) error {
	if err := client.tm.Add(); err != nil {
		return err
	}
	defer client.tm.Done()

	d, err := client.createDownload(p)
	if err != nil {
		return err
	}

	// block until the download has completed
	select {
	case <-d.completeChan:
		return d.Err()
	case <-client.tm.StopChan():
		return errors.New("download is shutdown")
	}
}

// performs a file download without blocking
func (client *StorageClient) DownloadAsync(p storage.DownloadParameters) error {
	if err := client.tm.Add(); err != nil {
		return err
	}
	defer client.tm.Done()

	_, err := client.createDownload(p)
	return err
}
