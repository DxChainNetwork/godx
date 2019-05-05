package core

import (
	"errors"
	"strconv"
	"sync"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	// canonicalChainEvChanSize is the size of channel listening to CanonicalChainHeadEvent.
	canonicalChainEvChanHeadSize = 10
)

var (
	emptyStorageContractID = types.StorageContractID{}

	errStorageProofTiming     = errors.New("missed proof triggered for file contract that is not expiring")
	errMissingStorageContract = errors.New("storage proof submitted for non existing file contract")
)

// Backend wraps all methods required for maintenance.
type Backend interface {
	SubscribeCanonicalChainEvent(ch chan<- CanonicalChainHeadEvent) event.Subscription
	ChainDb() ethdb.Database
}

type MaintenanceSystem struct {
	backend Backend

	// Subscription for new canonical chain event
	canonicalChainSub event.Subscription

	// Channel to receive new canonical chain event
	canonicalChainCh chan CanonicalChainHeadEvent
	quitCh           chan struct{}
	wg               sync.WaitGroup
}

func NewMaintenanceSystem(backend Backend) *MaintenanceSystem {
	m := &MaintenanceSystem{
		backend:          backend,
		canonicalChainCh: make(chan CanonicalChainHeadEvent, canonicalChainEvChanHeadSize),
		quitCh:           make(chan struct{}),
	}

	// subscribe canonical chain head event for file contract maintenance
	m.canonicalChainSub = m.backend.SubscribeCanonicalChainEvent(m.canonicalChainCh)
	m.wg.Add(1)
	go m.maintenanceLoop()
	return m
}

func (m *MaintenanceSystem) maintenanceLoop() {
	defer m.wg.Done()

	for {
		select {
		case ev := <-m.canonicalChainCh:
			db := m.backend.ChainDb()
			err := applyMaintenance(db, ev.Block)
			if err != nil {
				log.Error("failed to apply maintenace", "error", err)
				return
			}
		case err := <-m.canonicalChainSub.Err():
			log.Error("failed to subscribe canonical chain head event for file contract maintenance", "error", err)
			return
		case <-m.quitCh:
			log.Info("maintenance stopped")
			return
		}
	}
}

func (m *MaintenanceSystem) Stop() {
	log.Info("maintenance stopping ...")
	m.canonicalChainSub.Unsubscribe()
	close(m.quitCh)
	m.wg.Wait()
}

func applyMissedStorageProof(db ethdb.Database, height types.BlockHeight, fcid types.StorageContractID) error {

	// check if fileContract of this fcid exists
	fc, err := vm.GetStorageContract(db, fcid)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return errMissingStorageContract
		}
		return err
	}

	// Check that whether the file contract is in expire bucket at block height.
	if fc.WindowEnd != height {
		return errStorageProofTiming
	}

	// Remove the file contract from db
	vm.DeleteStorageContract(db, fcid)
	vm.DeleteExpireStorageContract(db, fcid, height)

	return nil
}

// applyStorageContractMaintenance looks for all of the file contracts that have
// expired without an appropriate storage proof, and calls 'applyMissedProof'
// for the file contract.
func applyStorageContractMaintenance(db ethdb.Database, block *types.Block) error {
	ldb, ok := db.(*ethdb.LDBDatabase)
	if !ok {
		return errors.New("not persistent db")
	}
	blockHeight := block.NumberU64()
	heightStr := strconv.FormatUint(blockHeight, 10)
	iterator := ldb.NewIteratorWithPrefix([]byte(vm.PrefixExpireStorageContract + heightStr + "-"))

	for iterator.Next() {
		keyBytes := iterator.Key()
		height, fcID := vm.SplitStorageContractID(keyBytes)
		if height == 0 && fcID == emptyStorageContractID {
			log.Warn("split empty file contract ID")
			continue
		}
		err := applyMissedStorageProof(db, types.BlockHeight(height), fcID)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyMaintenance(db ethdb.Database, block *types.Block) error {
	err := applyStorageContractMaintenance(db, block)
	if err != nil {
		return err
	}

	return nil
}
