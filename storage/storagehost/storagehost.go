// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	tm "github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	sm "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager"
)

var handlerMap = map[uint64]func(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error{
	storage.StorageContractCreationMsg:        handleContractCreate,
	storage.StorageContractUploadRequestMsg:   handleUpload,
	storage.StorageContractDownloadRequestMsg: handleDownload,
}

// TODO: Network, Transaction, protocol related implementations BELOW:
// TODO: check unlock hash
func (h *StorageHost) checkUnlockHash() error {
	return nil
}

// TODO: return the externalConfig for host
func (h *StorageHost) externalConfig() storage.HostExtConfig {
	return storage.HostExtConfig{}
}

// TODO: mock the database for storing storage obligation, currently use the
//  	 LDBDatabase, not sure which tables should be init here, modify the database
//  	 for developer's convenience
func (h *StorageHost) initDB() error {
	var err error
	h.db, err = ethdb.NewLDBDatabase(filepath.Join(h.persistDir, HostDB), 16, 16)
	if err != nil {
		return err
	}
	// add the close of database to the thread manager
	_ = h.tm.AfterStop(func() error {
		h.db.Close()
		return nil
	})
	// TODO: create the table if not exist
	return nil
}

// TODO: load the database, storage obligation, currently mock loads the config from the database,
//  	 if the config file load sucess
func (h *StorageHost) loadFromDB() error {
	return nil
}

// StorageHost provide functions for storagehost management
// It loads or use default config when it have been initialized
// It aims at communicate by protocal with client and lent its own storage to the client
type StorageHost struct {

	// Account manager for wallet/account related operation
	am *accounts.Manager

	// storageHost basic config
	broadcast          bool
	broadcastConfirmed bool
	blockHeight        uint64

	financialMetrics HostFinancialMetrics
	config           storage.HostIntConfig
	revisionNumber   uint64

	// storage host manager for manipulating the file storage system
	sm.StorageManager

	lockedStorageObligations map[common.Hash]*TryMutex

	// things for log and persistence
	// TODO: database to store the info of storage obligation, here just a mock
	db         *ethdb.LDBDatabase
	persistDir string
	log        log.Logger

	// things for thread safety
	lock sync.RWMutex
	tm   tm.ThreadManager

	ethBackend storage.EthBackend
}

// Start loads all APIs and make them mapping, also introduce the account
// manager as a member variable in side the StorageHost
func (h *StorageHost) Start(eth storage.EthBackend) {
	// TODO: Start Load all APIs and make them mapping
	// init the account manager
	h.am = eth.AccountManager()
	h.ethBackend = eth
}

// New Initialize the Host, including init the structure
// load or use the default config, init db and ext.
func New(persistDir string) (*StorageHost, error) {
	// do a host creation, but incomplete config

	host := StorageHost{
		log:        log.New(),
		persistDir: persistDir,
		// TODO: init the storageHostObligation
	}

	var err error   // error potentially affect the system
	var tmErr error // error for thread manager, could be handle, would be log only

	// use the thread manager to close the things open
	defer func() {
		if err != nil {
			if tmErr := host.tm.Stop(); tmErr != nil {
				err = errors.New(err.Error() + "; " + tmErr.Error())
			}
		}
	}()

	// try to make the dir for storing host files.
	// Because MkdirAll does nothing is the folder already exist, no worry to the existing folder
	if err = os.MkdirAll(persistDir, 0700); err != nil {
		host.log.Crit("Making directory hit unexpected error: " + err.Error())
		return nil, err
	}

	// initialize the storage manager
	host.StorageManager, err = sm.New(filepath.Join(persistDir, StorageManager))
	if err != nil {
		host.log.Crit("Error caused by Creating StorageManager: " + err.Error())
		return nil, err
	}

	// add the storage manager to the thread group
	// log if closing fail
	if tmErr = host.tm.AfterStop(func() error {
		err := host.StorageManager.Close()
		if err != nil {
			host.log.Warn("Fail to close storage manager: " + err.Error())
		}
		return err
	}); tmErr != nil {
		host.log.Warn(tmErr.Error())
	}

	// load the data from file or from default config
	err = host.load()
	if err != nil {
		return nil, err
	}

	// add the syncConfig to the thread group, make sure it would be store before system down
	if tmErr = host.tm.AfterStop(func() error {
		err := host.syncConfig()
		if err != nil {
			host.log.Warn("Fail to synchronize to config file: " + err.Error())
		}
		return err
	}); tmErr != nil {
		// just log the cannot syn problem, the does not sever enough to panic the system
		host.log.Warn(tmErr.Error())
	}

	// TODO: storageObligation handle

	// TODO: Init the networking

	return &host, nil
}

// Close the storage host and persist the data
func (h *StorageHost) Close() error {
	return h.tm.Stop()
}

// HostExtConfig return the host external config, which configure host through,
// user should not able to modify the config
func (h *StorageHost) HostExtConfig() storage.HostExtConfig {
	h.lock.Lock()
	defer h.lock.Unlock()
	if err := h.tm.Add(); err != nil {
		h.log.Crit("Call to HostExtConfig fail")
	}

	defer h.tm.Done()
	// mock the return of host external config
	return h.externalConfig()
}

// FinancialMetrics contains the information about the activities,
// commitments, rewards of host
func (h *StorageHost) FinancialMetrics() HostFinancialMetrics {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if err := h.tm.Add(); err != nil {
		h.log.Crit("Fail to add FinancialMetrics Getter to thread manager")
	}
	defer h.tm.Done()

	return h.financialMetrics
}

// SetIntConfig set the input hostconfig to the current host if check all things are good
func (h *StorageHost) SetIntConfig(config storage.HostIntConfig, debug ...bool) error {
	// TODO: not sure the exact procedure
	// TODO: For debugging purpose, currently use vargs and tag for directly ignore the
	//  checking parts, set the config and increase the revisionNumber, for future
	//  development, please follow the logic to make the test case success as expected,
	//  or delete and  do another test case for convenience

	h.lock.Lock()
	defer h.lock.Unlock()
	if err := h.tm.Add(); err != nil {
		h.log.Crit("Fail to add HostIntConfig Getter to thread manager")
		return err
	}
	defer h.tm.Done()

	// for debugging purpose, just jump to the last part, so it won't be affected
	// by the implementation of checking parts (check unlock hash and network address)
	if debug != nil && len(debug) >= 1 && debug[0] {
		goto LOADSETTING
	}

	// TODO: check the unlock hash, if does not need the hash, remove this part of code
	if config.AcceptingContracts {
		err := h.checkUnlockHash()
		if err != nil {
			return errors.New("no unlock hash, stop updating: " + err.Error())
		}
	}

	// TODO: Checking the NetAddress

LOADSETTING:
	h.config = config
	h.revisionNumber++

	// synchronize the config to file
	if err := h.syncConfig(); err != nil {
		return errors.New("internal config update fail: " + err.Error())
	}

	return nil
}

// InternalConfig Return the internal config of host
func (h *StorageHost) InternalConfig() storage.HostIntConfig {
	h.lock.RLock()
	defer h.lock.RUnlock()
	// if not able to add to the thread manager, simply return a empty config structure
	if err := h.tm.Add(); err != nil {
		return storage.HostIntConfig{}
	}
	defer h.tm.Done()
	return h.config
}

// load do the following things:
// 1. init the database
// 2. load the config from file
// 3. load from database
// 4. if the config file not found, create the config file, and use the default config
// 5. finally synchronize the data to config file
func (h *StorageHost) load() error {
	var err error

	// Initialize the database
	if err = h.initDB(); err != nil {
		h.log.Crit("Unable to initialize the database: " + err.Error())
		return err
	}

	// try to load from the config files,
	if err = h.loadFromFile(); err == nil {
		// TODO: mock the loading from database
		err = h.loadFromDB()
		return err
	} else if !os.IsNotExist(err) {
		// if the error is NOT caused by FILE NOT FOUND Exception
		return err
	}

	// At this step, the error is caused by FILE NOT FOUND Exception
	// Create the config file
	h.log.Info("Creat a new HostSetting file")

	// currently the error is caused by file not found exception
	// create the file
	file, err := os.Create(filepath.Join(h.persistDir, HostSettingFile))
	if err != nil {
		// if the error is throw when create the file
		// close the file and directly return the error
		_ = file.Close()
		return err
	}

	// assert the error is nil, close the file
	if err := file.Close(); err != nil {
		h.log.Info("Unable to close the config file")
	}

	// load the default config
	h.loadDefaults()

	// and get synchronization
	if syncErr := h.syncConfig(); syncErr != nil {
		h.log.Warn("Tempt to synchronize config to file failed: " + syncErr.Error())
	}

	return nil
}

// loadFromFile load host config from the file, guarantee that host would not be
// modified if error happen
func (h *StorageHost) loadFromFile() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// load and create a persist from JSON file
	persist := new(persistence)

	// NOTE:
	// if it is loaded the file causing the error, directly return the error info
	// and not do any modification to the host
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join(h.persistDir, HostSettingFile), persist); err != nil {
		return err
	}

	h.loadPersistence(persist)
	return nil
}

// loadDefaults loads the default config for the host
func (h *StorageHost) loadDefaults() {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.config = loadDefaultConfig()
}

func (h *StorageHost) HandleSession(s *storage.Session) error {
	msg, err := s.ReadMsg()
	if err != nil {
		return err
	}
	if handler, ok := handlerMap[msg.Code]; ok {
		return handler(h, s, msg)
	}
	return nil
}

func handleContractCreate(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {

	// this RPC call contains two request/response exchanges.
	s.SetDeadLine(storage.FormContractTime)

	if !h.externalConfig().AcceptingContracts {
		err := errors.New("host is not accepting new contracts")
		s.SendErrorMsg(err)
		return err
	}

	// 1. Read ContractCreateRequest msg
	var req storage.ContractCreateRequest
	if err := beginMsg.Decode(&req); err != nil {
		s.SendErrorMsg(err)
		return err
	}

	sc := req.StorageContract
	clientPK, err := crypto.SigToPub(sc.RLPHash().Bytes(), req.Sign)
	if err != nil {
		return ExtendErr("recover publicKey from signature failed", err)
	}
	sc.Signatures[0] = req.Sign

	// Check host balance >= storage contract cost
	hostAddress := sc.ValidProofOutputs[1].Address
	stateDB, err := h.ethBackend.GetBlockChain().State()
	if err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("get state db error", err)
	}
	if stateDB.GetBalance(hostAddress).Cmp(sc.HostCollateral.Value) < 0 {
		s.SendErrorMsg(err)
		return ExtendErr("host balance insufficient", err)
	}

	account := accounts.Account{Address: hostAddress}
	wallet, err := h.ethBackend.AccountManager().Find(account)

	if err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("find host account error", err)
	}
	hostContractSign, err := wallet.SignHash(account, sc.RLPHash().Bytes())
	if err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("host account sign storage contract error", err)
	}

	// Ecrecover host pk for setup unlock conditions
	hostPK, err := crypto.SigToPub(sc.RLPHash().Bytes(), hostContractSign)
	if err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("Ecrecover pk from sign error", err)
	}

	// Check an incoming storage contract matches the host's expectations for a valid contract
	if err := VerifyStorageContract(h, &sc, clientPK, hostPK); err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("host verify storage contract failed", err)
	}

	// 2. After check, send host contract sign to client
	sc.Signatures[1] = hostContractSign
	if err := s.SendStorageContractCreationHostSign(hostContractSign); err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("send storage contract create sign by host", err)
	}

	// 3. Wait for the client revision sign
	var clientRevisionSign []byte
	msg, err := s.ReadMsg()
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	if err = msg.Decode(&clientRevisionSign); err != nil {
		s.SendErrorMsg(err)
		return err
	}

	// Reconstruct revision locally by host
	storageContractRevision := types.StorageContractRevision{
		ParentID: sc.RLPHash(),
		UnlockConditions: types.UnlockConditions{
			PublicKeys: []ecdsa.PublicKey{
				*clientPK,
				*hostPK,
			},
			SignaturesRequired: 2,
		},
		NewRevisionNumber:     1,
		NewFileSize:           sc.FileSize,
		NewFileMerkleRoot:     sc.FileMerkleRoot,
		NewWindowStart:        sc.WindowStart,
		NewWindowEnd:          sc.WindowEnd,
		NewValidProofOutputs:  sc.ValidProofOutputs,
		NewMissedProofOutputs: sc.MissedProofOutputs,
		NewUnlockHash:         sc.UnlockHash,
	}

	// Sign revision by storage host
	hostRevisionSign, err := wallet.SignHash(account, storageContractRevision.RLPHash().Bytes())
	if err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("host sign revison error", err)
	}

	storageContractRevision.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}

	so := StorageObligation{
		SectorRoots:              nil,
		ContractCost:             h.externalConfig().ContractPrice.BigIntPtr(),
		LockedCollateral:         new(big.Int).Sub(sc.ValidProofOutputs[1].Value, h.externalConfig().ContractPrice.BigIntPtr()),
		PotentialStorageRevenue:  big.NewInt(0),
		RiskedCollateral:         big.NewInt(0),
		NegotiationHeight:        h.blockHeight,
		OriginStorageContract:    sc,
		StorageContractRevisions: []types.StorageContractRevision{storageContractRevision},
	}

	if err := FinalizeStorageObligation(h, so); err != nil {
		s.SendErrorMsg(err)
		return ExtendErr("finalize storage obligation error", err)
	}

	return s.SendStorageContractCreationHostRevisionSign(hostRevisionSign)
}

func handleUpload(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {
	s.SetDeadLine(storage.ContractRevisionTime)

	// Read upload request
	var uploadRequest storage.UploadRequest
	if err := beginMsg.Decode(&uploadRequest); err != nil {
		s.SendErrorMsg(err)
		return fmt.Errorf("[Error Decode UploadRequest] Msg: %v | Error: %v", beginMsg, err)
	}

	// Get revision from storage obligation
	so, err := GetStorageObligation(h.db, uploadRequest.StorageContractID)
	if err != nil {
		s.SendErrorMsg(err)
		return fmt.Errorf("[Error Get Storage Obligation] Error: %v", err)
	}

	settings := h.externalConfig()
	currentBlockHeight := h.blockHeight
	currentRevision := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]

	// Process each action
	newRoots := append([]common.Hash(nil), so.SectorRoots...)
	sectorsChanged := make(map[uint64]struct{})
	var bandwidthRevenue *big.Int
	var sectorsRemoved []common.Hash
	var sectorsGained []common.Hash
	var gainedSectorData [][]byte
	for _, action := range uploadRequest.Actions {
		switch action.Type {
		case storage.UploadActionAppend:
			// Update sector roots.
			newRoot := storage.MerkleRoot(action.Data)
			newRoots = append(newRoots, newRoot)
			sectorsGained = append(sectorsGained, newRoot)
			gainedSectorData = append(gainedSectorData, action.Data)

			sectorsChanged[uint64(len(newRoots))-1] = struct{}{}

			// Update finances
			bandwidthRevenue = bandwidthRevenue.Add(bandwidthRevenue, settings.UploadBandwidthPrice.MultUint64(storage.SectorSize).BigIntPtr())

		default:
			s.SendErrorMsg(err)
			return errors.New("unknown action type " + action.Type)
		}
	}

	var storageRevenue, newDeposit *big.Int
	if len(newRoots) > len(so.SectorRoots) {
		bytesAdded := storage.SectorSize * uint64(len(newRoots)-len(so.SectorRoots))
		blocksRemaining := so.ProofDeadline() - currentBlockHeight
		blockBytesCurrency := new(big.Int).Mul(big.NewInt(int64(blocksRemaining)), big.NewInt(int64(bytesAdded)))
		storageRevenue = new(big.Int).Mul(blockBytesCurrency, settings.StoragePrice.BigIntPtr())
		newDeposit = newDeposit.Add(newDeposit, new(big.Int).Mul(blockBytesCurrency, settings.Deposit.BigIntPtr()))
	}

	// If a Merkle proof was requested, construct it
	newMerkleRoot := storage.CachedMerkleRoot(newRoots)

	// Construct the new revision
	newRevision := currentRevision
	newRevision.NewRevisionNumber = uploadRequest.NewRevisionNumber
	for _, action := range uploadRequest.Actions {
		if action.Type == storage.UploadActionAppend {
			newRevision.NewFileSize += storage.SectorSize
		}
	}
	newRevision.NewFileMerkleRoot = newMerkleRoot
	newRevision.NewValidProofOutputs = make([]types.DxcoinCharge, len(currentRevision.NewValidProofOutputs))
	for i := range newRevision.NewValidProofOutputs {
		newRevision.NewValidProofOutputs[i] = types.DxcoinCharge{
			Value:   uploadRequest.NewValidProofValues[i],
			Address: currentRevision.NewValidProofOutputs[i].Address,
		}
	}
	newRevision.NewMissedProofOutputs = make([]types.DxcoinCharge, len(currentRevision.NewMissedProofOutputs))
	for i := range newRevision.NewMissedProofOutputs {
		newRevision.NewMissedProofOutputs[i] = types.DxcoinCharge{
			Value:   uploadRequest.NewMissedProofValues[i],
			Address: currentRevision.NewMissedProofOutputs[i].Address,
		}
	}

	// Verify the new revision
	newRevenue := new(big.Int).Add(storageRevenue.Add(storageRevenue, bandwidthRevenue), settings.BaseRPCPrice.BigIntPtr())
	so.SectorRoots, newRoots = newRoots, so.SectorRoots
	if err := VerifyRevision(&so, &newRevision, currentBlockHeight, newRevenue, newDeposit); err != nil {
		s.SendErrorMsg(err)
		return err
	}
	so.SectorRoots, newRoots = newRoots, so.SectorRoots

	var merkleResp storage.UploadMerkleProof
	// Calculate which sectors changed
	oldNumSectors := uint64(len(so.SectorRoots))
	proofRanges := make([]storage.ProofRange, 0, len(sectorsChanged))
	for index := range sectorsChanged {
		if index < oldNumSectors {
			proofRanges = append(proofRanges, storage.ProofRange{
				Start: index,
				End:   index + 1,
			})
		}
	}
	sort.Slice(proofRanges, func(i, j int) bool {
		return proofRanges[i].Start < proofRanges[j].Start
	})
	// Record old leaf hashes for all changed sectors
	leafHashes := make([]common.Hash, len(proofRanges))
	for i, r := range proofRanges {
		leafHashes[i] = so.SectorRoots[r.Start]
	}

	// Construct the merkle proof
	merkleResp = storage.UploadMerkleProof{
		OldSubtreeHashes: storage.MerkleDiffProof(proofRanges, oldNumSectors, nil, so.SectorRoots),
		OldLeafHashes:    leafHashes,
		NewMerkleRoot:    newMerkleRoot,
	}

	// Calculate bandwidth cost of proof
	proofSize := storage.HashSize * (len(merkleResp.OldSubtreeHashes) + len(leafHashes) + 1)

	bandwidthRevenue = bandwidthRevenue.Add(bandwidthRevenue, settings.DownloadBandwidthPrice.MultUint64(uint64(proofSize)).BigIntPtr())

	if err := s.SendStorageContractUploadMerkleProof(merkleResp); err != nil {
		s.SendErrorMsg(err)
		return fmt.Errorf("[Error Send Storage Proof] Error: %v", err)
	}

	var clientRevisionSign []byte
	msg, err := s.ReadMsg()
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	if err = msg.Decode(&clientRevisionSign); err != nil {
		s.SendErrorMsg(err)
		return err
	}
	newRevision.Signatures[0] = clientRevisionSign

	// Update the storage obligation
	so.SectorRoots = newRoots
	so.PotentialStorageRevenue = so.PotentialStorageRevenue.Add(so.PotentialStorageRevenue, storageRevenue)
	so.RiskedCollateral = so.RiskedCollateral.Add(so.RiskedCollateral, newDeposit)
	so.PotentialUploadRevenue = so.PotentialUploadRevenue.Add(so.PotentialUploadRevenue, bandwidthRevenue)
	so.StorageContractRevisions = append(so.StorageContractRevisions, newRevision)
	err = h.modifyStorageObligation(so, sectorsRemoved, sectorsGained, gainedSectorData)
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	// Sign host's revision and send it to client
	account := accounts.Account{Address: newRevision.NewValidProofOutputs[1].Address}
	wallet, err := h.am.Find(account)
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	sign, err := wallet.SignHash(account, newRevision.RLPHash().Bytes())
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	if err := s.SendStorageContractUploadHostRevisionSign(sign); err != nil {
		s.SendErrorMsg(err)
		return err
	}

	return nil
}

func handleDownload(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {
	s.SetDeadLine(storage.DownloadTime)

	// read the download request.
	var req storage.DownloadRequest
	err := beginMsg.Decode(req)
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	// as soon as client complete downloading, will send stop msg.
	stopSignal := make(chan error, 1)
	go func() {
		msg, err := s.ReadMsg()
		if err != nil {
			stopSignal <- err
		} else if msg.Code != storage.NegotiationStopMsg {
			stopSignal <- errors.New("expected 'stop' from client, got " + string(msg.Code))
		} else {
			stopSignal <- nil
		}
	}()

	// get storage obligation
	so, err := GetStorageObligation(h.db, req.StorageContractID)
	if err != nil {
		return fmt.Errorf("[Error Get Storage Obligation] Error: %v", err)
	}

	// Check the contract is empty
	if reflect.DeepEqual(so.OriginStorageContract, types.StorageContract{}) {
		err := errors.New("no contract locked")
		s.SendErrorMsg(err)
		<-stopSignal
		return err
	}

	h.lock.RLock()
	settings := h.externalConfig()
	h.lock.RUnlock()

	currentRevision := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]

	// Validate the request.
	for _, sec := range req.Sections {
		var err error
		switch {
		case uint64(sec.Offset)+uint64(sec.Length) > storage.SectorSize:
			err = errors.New("download request has invalid sector bounds")
		case sec.Length == 0:
			err = errors.New("length cannot be zero")
		case req.MerkleProof && (sec.Offset%storage.SegmentSize != 0 || sec.Length%storage.SegmentSize != 0):
			err = errors.New("offset and length must be multiples of SegmentSize when requesting a Merkle proof")
		case len(req.NewValidProofValues) != len(currentRevision.NewValidProofOutputs):
			err = errors.New("wrong number of valid proof values")
		case len(req.NewMissedProofValues) != len(currentRevision.NewMissedProofOutputs):
			err = errors.New("wrong number of missed proof values")
		}
		if err != nil {
			s.SendErrorMsg(err)
			return err
		}
	}

	// construct the new revision
	newRevision := currentRevision
	newRevision.NewRevisionNumber = req.NewRevisionNumber
	newRevision.NewValidProofOutputs = make([]types.DxcoinCharge, len(currentRevision.NewValidProofOutputs))
	for i := range newRevision.NewValidProofOutputs {
		newRevision.NewValidProofOutputs[i] = types.DxcoinCharge{
			Value:   req.NewValidProofValues[i],
			Address: currentRevision.NewValidProofOutputs[i].Address,
		}
	}
	newRevision.NewMissedProofOutputs = make([]types.DxcoinCharge, len(currentRevision.NewMissedProofOutputs))
	for i := range newRevision.NewMissedProofOutputs {
		newRevision.NewMissedProofOutputs[i] = types.DxcoinCharge{
			Value:   req.NewMissedProofValues[i],
			Address: currentRevision.NewMissedProofOutputs[i].Address,
		}
	}

	// calculate expected cost and verify against client's revision
	var estBandwidth uint64
	sectorAccesses := make(map[common.Hash]struct{})
	for _, sec := range req.Sections {

		// use the worst-case proof size of 2*tree depth.
		// this occurs when proving across the two leaves in the center of the tree.
		estHashesPerProof := 2 * bits.Len64(storage.SectorSize/storage.SegmentSize)
		estBandwidth += uint64(sec.Length) + uint64(estHashesPerProof*storage.HashSize)
		sectorAccesses[sec.MerkleRoot] = struct{}{}
	}
	if estBandwidth < storage.RPCMinLen {
		estBandwidth = storage.RPCMinLen
	}

	// calculate total cost
	bandwidthCost := settings.DownloadBandwidthPrice.MultUint64(estBandwidth)
	sectorAccessCost := settings.SectorAccessPrice.MultUint64(uint64(len(sectorAccesses)))
	totalCost := settings.BaseRPCPrice.Add(bandwidthCost).Add(sectorAccessCost)
	err = VerifyPaymentRevision(currentRevision, newRevision, h.blockHeight, totalCost.BigIntPtr())
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	// Sign the new revision.
	account := accounts.Account{Address: newRevision.NewValidProofOutputs[1].Address}
	wallet, err := h.am.Find(account)
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	hostSig, err := wallet.SignHash(account, newRevision.RLPHash().Bytes())
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	// update the storage obligation.
	paymentTransfer := currentRevision.NewValidProofOutputs[0].Value.Sub(currentRevision.NewValidProofOutputs[0].Value, newRevision.NewValidProofOutputs[0].Value)
	so.PotentialDownloadRevenue = so.PotentialDownloadRevenue.Add(so.PotentialDownloadRevenue, paymentTransfer)
	so.StorageContractRevisions = append(so.StorageContractRevisions, newRevision)
	h.lock.Lock()
	err = h.modifyStorageObligation(so, nil, nil, nil)
	h.lock.Unlock()
	if err != nil {
		s.SendErrorMsg(err)
		return err
	}

	// enter response loop
	for i, sec := range req.Sections {

		// TODO: Fetch the requested data.
		//sectorData, err := h.ReadSector(sec.MerkleRoot)
		//if err != nil {
		//	s.SendErrorMsg(err)
		//	return err
		//}
		sectorData := []byte{}
		data := sectorData[sec.Offset : sec.Offset+sec.Length]

		// construct the Merkle proof, if requested.
		var proof []common.Hash
		if req.MerkleProof {
			proofStart := int(sec.Offset) / storage.SegmentSize
			proofEnd := int(sec.Offset+sec.Length) / storage.SegmentSize
			proof = storage.MerkleRangeProof(sectorData, proofStart, proofEnd)
			proof = []common.Hash{}
		}

		// send the response. If the client sent a stop signal, or this is the final response, include host's signature in the response.
		resp := storage.DownloadResponse{
			Signature:   nil,
			Data:        data,
			MerkleProof: proof,
		}
		select {
		case err := <-stopSignal:
			if err != nil {
				return err
			}
			resp.Signature = hostSig

			// send data to client
			return s.SendStorageContractDownloadData(resp)
		default:
		}
		if i == len(req.Sections)-1 {
			resp.Signature = hostSig
		}

		// send data to client
		if err := s.SendStorageContractDownloadData(resp); err != nil {
			s.SendErrorMsg(err)
			return err
		}
	}

	// the stop signal must arrive before RPC is complete.
	return <-stopSignal
}
