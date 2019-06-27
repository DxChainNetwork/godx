// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"bytes"
	"errors"
	"math/big"
	"reflect"
	"strconv"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
)

const (
	postponedExecution          = 3  //Total length of time to start a test task
	confirmedBufferHeight       = 40 //signing transaction not confirmed maximum time
	errGetStorageResponsibility = "failed to get data from DB as I wished "
	errPutStorageResponsibility = "failed to put data from DB as I wished "
	//PrefixStorageResponsibility db prefix for StorageResponsibility
	PrefixStorageResponsibility = "StorageResponsibility-"
	//PrefixHeight db prefix for task
	PrefixHeight = "height-"
)

//Storage contract should not be empty
var (
	emptyStorageContract = types.StorageContract{}

	//Total time to sign the contract
	postponedExecutionBuffer = storage.BlocksPerDay
)

const (
	unresolved storageResponsibilityStatus = iota //Storage responsibility is initialization, no meaning
	rejected                                      //Storage responsibility never begins
	succeeded                                     // Successful storage responsibility
	failed                                        //Failed storage responsibility
)

var (
	errEmptyOriginStorageContract = errors.New("storage contract has no storage responsibility")
	errEmptyRevisionSet           = errors.New("take the last revision ")
	errInsaneRevision             = errors.New("revision is not necessary")
	errNotAllowed                 = errors.New("time is not allowed")
	errTransactionNotConfirmed    = errors.New("transaction not confirmed")
)

type (
	//StorageResponsibility storage contract management and maintenance on the storage host side
	StorageResponsibility struct {
		//Store the root set of related metadata
		SectorRoots []common.Hash

		//Primary source of income and expenditure
		ContractCost             common.BigInt
		LockedStorageDeposit     common.BigInt
		PotentialDownloadRevenue common.BigInt
		PotentialStorageRevenue  common.BigInt
		PotentialUploadRevenue   common.BigInt
		RiskedStorageDeposit     common.BigInt
		TransactionFeeExpenses   common.BigInt

		//Chain height during negotiation
		NegotiationBlockNumber uint64

		//The initial storage contract
		OriginStorageContract types.StorageContract

		//Revision set
		StorageContractRevisions []types.StorageContractRevision
		ResponsibilityStatus     storageResponsibilityStatus

		CreateContractConfirmed    bool
		StorageProofConfirmed      bool
		StorageProofConstructed    bool
		StorageRevisionConfirmed   bool
		StorageRevisionConstructed bool
	}

	storageResponsibilityStatus uint64
)

func (i storageResponsibilityStatus) String() string {
	switch i {
	case 0:
		return "unresolved"
	case 1:
		return "rejected"
	case 2:
		return "succeeded"
	case 3:
		return "failed"
	default:
		return "storageResponsibilityStatus(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}

//Returns expired block number
func (so *StorageResponsibility) expiration() uint64 {
	//If there is revision, return NewWindowStart
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewWindowStart
	}
	return so.OriginStorageContract.WindowStart
}

func (so *StorageResponsibility) fileSize() uint64 {
	//If there is revision, return NewFileSize
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewFileSize
	}
	return so.OriginStorageContract.FileSize
}

func (so *StorageResponsibility) id() (scid common.Hash) {
	return so.OriginStorageContract.RLPHash()
}

//Check this storage responsibility
func (so *StorageResponsibility) isSane() error {
	if reflect.DeepEqual(so.OriginStorageContract, emptyStorageContract) {
		return errEmptyOriginStorageContract
	}

	if len(so.StorageContractRevisions) == 0 {
		return errEmptyRevisionSet
	}

	return nil
}

func (so *StorageResponsibility) merkleRoot() common.Hash {
	//If there is revision, return NewFileMerkleRoot
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewFileMerkleRoot
	}
	return so.OriginStorageContract.FileMerkleRoot
}

func (so *StorageResponsibility) payouts() ([]types.DxcoinCharge, []types.DxcoinCharge) {
	validProofOutputs := make([]types.DxcoinCharge, 2)
	missedProofOutputs := make([]types.DxcoinCharge, 2)
	//If there is revision, return NewValidProofOutputs and NewMissedProofOutputs
	if len(so.StorageContractRevisions) > 0 {
		copy(validProofOutputs, so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewValidProofOutputs)
		copy(missedProofOutputs, so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewMissedProofOutputs)
		return validProofOutputs, missedProofOutputs
	}
	validProofOutputs = so.OriginStorageContract.ValidProofOutputs
	missedProofOutputs = so.OriginStorageContract.MissedProofOutputs
	return validProofOutputs, missedProofOutputs
}

//The block number that the proof must submit
func (so *StorageResponsibility) proofDeadline() uint64 {
	//If there is revision, return NewWindowEnd
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewWindowEnd
	}
	return so.OriginStorageContract.WindowEnd

}

//Amount that can be obtained after fulfilling the responsibility
func (so StorageResponsibility) value() common.BigInt {
	return so.ContractCost.Add(so.PotentialDownloadRevenue).Add(so.PotentialStorageRevenue).Add(so.PotentialUploadRevenue).Add(so.RiskedStorageDeposit)
}

func (h *StorageHost) deleteStorageResponsibilities(soids []common.Hash) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	for _, soid := range soids {
		err := deleteStorageResponsibility(h.db, soid)
		if err != nil {
			return err
		}
	}

	return nil
}

//Schedule a task to execute at the specified block number
func (h *StorageHost) queueTaskItem(height uint64, id common.Hash) error {

	if height < h.blockHeight {
		h.log.Info("It is not appropriate to arrange such a task")
	}

	return StoreHeight(h.db, id, height)
}

//InsertStorageResponsibility insert a storage Responsibility to the storage host.
func (h *StorageHost) InsertStorageResponsibility(so StorageResponsibility) error {
	err := func() error {
		h.lock.Lock()
		defer h.lock.Unlock()

		//Submit revision time exceeds storage responsibility expiration time
		if h.blockHeight+postponedExecutionBuffer >= so.expiration() {
			h.log.Warn("failed to submit revision in storage responsibility due date")
			return errNotAllowed
		}

		//Not enough time to submit proof of storage, no need to put in the task force
		if so.expiration()+postponedExecution >= so.proofDeadline() {
			h.log.Warn("Not enough time to submit proof of storage")
			return errNotAllowed
		}

		errDB := func() error {
			//If it is contract renew, put the sectorRoots of the storage contract into the new contract.
			if len(so.SectorRoots) != 0 {
				err := h.AddSectorBatch(so.SectorRoots)
				if err != nil {
					return err
				}
			}
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				return errPut
			}
			return nil
		}()

		if errDB != nil {
			return errDB
		}

		// Update the host financial metrics with regards to this storage responsibility.
		h.financialMetrics.ContractCount++
		h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Add(so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Add(so.LockedStorageDeposit)
		h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Add(so.RiskedStorageDeposit)
		h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Add(so.TransactionFeeExpenses)

		return nil
	}()

	//The above operation has an error and will not be inserted into the task queue.
	if err != nil {
		return err
	}

	h.lock.Lock()
	defer h.lock.Unlock()
	//insert the check  contract create task in the task queue.
	errContractCreate := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
	errContractCreateDoubleTime := h.queueTaskItem(h.blockHeight+postponedExecution*2, so.id())

	//insert the check revision task in the task queue.
	errRevision := h.queueTaskItem(so.expiration()-postponedExecutionBuffer, so.id())
	errRevisionDoubleTime := h.queueTaskItem(so.expiration()-postponedExecutionBuffer+postponedExecution, so.id())

	//insert the check proof task in the task queue.
	errProof := h.queueTaskItem(so.expiration()+postponedExecution, so.id())
	errProofDoubleTime := h.queueTaskItem(so.expiration()+postponedExecution*2, so.id())
	err = common.ErrCompose(errContractCreate, errContractCreateDoubleTime, errRevision, errRevisionDoubleTime, errProof, errProofDoubleTime)
	if err != nil {
		h.log.Warn("Error with task item, redacting responsibility", "id", so.id())
		return common.ErrCompose(err, h.removeStorageResponsibility(so, rejected))
	}

	return nil
}

//the virtual sector will need to appear in 'sectorsRemoved' multiple times. Same with 'sectorsGained'。
func (h *StorageHost) modifyStorageResponsibility(so StorageResponsibility, sectorsRemoved []common.Hash, sectorsGained []common.Hash, gainedSectorData [][]byte) error {
	if _, ok := h.lockedStorageResponsibility[so.id()]; ok {
		h.log.Warn("modifyStorageResponsibility called with an responsibility that is not locked")
	}

	//Need enough time to submit revision
	if so.expiration()-postponedExecutionBuffer <= h.blockHeight {
		return errNotAllowed
	}

	//sectorsGained and gainedSectorData must have the same length
	if len(sectorsGained) != len(gainedSectorData) {
		h.log.Warn("sectorsGained and gainedSectorData must have the same length", "sectorsGained length", len(sectorsGained), "gainedSectorDataLength", len(gainedSectorData))
		return errInsaneRevision
	}

	for _, data := range gainedSectorData {
		//No 4MB sector has no meaning
		if uint64(len(data)) != storage.SectorSize {
			h.log.Warn("No 4MB sector has no meaning,sector size", "length", len(data))
			return errInsaneRevision
		}
	}

	var i int
	var err error
	for i = range sectorsGained {
		err = h.AddSector(sectorsGained[i], gainedSectorData[i])
		//If the adding or update fails,the added sectors should be
		//removed and the StorageResponsibility should be considered invalid.
		if err != nil {
			h.log.Warn("Error writing data to the sector", "err", err)
			break
		}
	}
	//This operation is wrong, you need to restore the sector
	if err != nil {
		for j := 0; j < i; j++ {
			//The error of restoring a sector doesn't make any sense to us.
			h.DeleteSector(sectorsGained[j])
		}
		return err
	}

	var oldso StorageResponsibility
	var errOld error
	errDBso := func() error {
		//Get old storage responsibility, return error if not found
		oldso, errOld = getStorageResponsibility(h.db, so.id())
		if errOld != nil {
			return errOld
		}

		return putStorageResponsibility(h.db, so.id(), so)
	}()

	if errDBso != nil {
		//This operation is wrong, you need to restore the sector
		for i := range sectorsGained {
			//The error of restoring a sector doesn't make any sense to us.
			h.DeleteSector(sectorsGained[i])
		}
		return errDBso
	}
	//Delete the deleted sector
	for k := range sectorsRemoved {
		//The error of restoring a sector doesn't make any sense to us.
		h.DeleteSector(sectorsRemoved[k])
	}

	// Update the financial information for the storage responsibility - apply the
	h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Add(so.ContractCost)
	h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Add(so.LockedStorageDeposit)
	h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
	h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
	h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
	h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Add(so.RiskedStorageDeposit)
	h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Add(so.TransactionFeeExpenses)

	// Update the financial information for the storage responsibility - remove the
	h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(oldso.ContractCost)
	h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(oldso.LockedStorageDeposit)
	h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(oldso.PotentialStorageRevenue)
	h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(oldso.PotentialDownloadRevenue)
	h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(oldso.PotentialUploadRevenue)
	h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(oldso.RiskedStorageDeposit)
	h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Sub(oldso.TransactionFeeExpenses)

	return nil
}

//PruneStaleStorageResponsibilities remove stale storage responsibilities because these storage responsibilities will affect the financial metrics of the host
func (h *StorageHost) PruneStaleStorageResponsibilities() error {
	h.lock.RLock()
	sos := h.StorageResponsibilities()
	h.lock.RUnlock()
	var scids []common.Hash
	for _, so := range sos {
		if h.blockHeight > so.NegotiationBlockNumber+confirmedBufferHeight {
			scids = append(scids, so.id())
		}
		if !so.CreateContractConfirmed {
			return errTransactionNotConfirmed
		}
	}

	// Delete storage responsibility from the database.
	err := h.deleteStorageResponsibilities(scids)
	if err != nil {
		h.log.Warn("unable to delete responsibility", "err", err)
		return err
	}

	// Update the financial metrics of the host.
	return h.resetFinancialMetrics()
}

//No matter what state the storage responsibility will be deleted
func (h *StorageHost) removeStorageResponsibility(so StorageResponsibility, sos storageResponsibilityStatus) error {

	//Unchecked error, even if there is an error, we want to delete
	h.DeleteSectorBatch(so.SectorRoots)

	switch sos {
	case unresolved:
		h.log.Info("storage responsibility 'unresolved' during call to removeStorageResponsibility", "id", so.id())
	case rejected:
		if h.financialMetrics.TransactionFeeExpenses.Cmp(so.TransactionFeeExpenses) >= 0 {
			// Remove the responsibility statistics as potential risk and income.
			h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
			h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedStorageDeposit)
			h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
			h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
			h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
			h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedStorageDeposit)
			h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Sub(so.TransactionFeeExpenses)
		}
	case succeeded:
		revenue := so.ContractCost.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
		//No storage responsibility for file upload or download does not require proof of storage
		if len(so.SectorRoots) == 0 {
			h.log.Info("No need to submit a storage proof for empty storage contract.", "Revenue", revenue)
		} else {
			h.log.Info("Successfully submitted a storage proof.", " Revenue", revenue)
		}

		// Remove the responsibility statistics as potential risk and income.
		h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedStorageDeposit)
		h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedStorageDeposit)

		// Add the responsibility statistics as actual income.
		h.financialMetrics.ContractCompensation = h.financialMetrics.ContractCompensation.Add(so.ContractCost)
		h.financialMetrics.StorageRevenue = h.financialMetrics.StorageRevenue.Add(so.PotentialStorageRevenue)
		h.financialMetrics.DownloadBandwidthRevenue = h.financialMetrics.DownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
		h.financialMetrics.UploadBandwidthRevenue = h.financialMetrics.UploadBandwidthRevenue.Add(so.PotentialUploadRevenue)

	case failed:
		// Remove the responsibility statistics as potential risk and income.
		h.log.Info("Missed storage proof.", "Revenue", so.ContractCost.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue))

		h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedStorageDeposit)
		h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedStorageDeposit)

		// Add the responsibility statistics as loss.
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Add(so.RiskedStorageDeposit)
		h.financialMetrics.LostRevenue = h.financialMetrics.LostRevenue.Add(so.ContractCost).Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)

	}

	h.financialMetrics.ContractCount--
	so.ResponsibilityStatus = sos
	so.SectorRoots = []common.Hash{}
	return putStorageResponsibility(h.db, so.id(), so)
}

func (h *StorageHost) resetFinancialMetrics() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	fm := HostFinancialMetrics{}
	sos := h.StorageResponsibilities()
	for _, so := range sos {
		// Submit transaction fee first
		fm.TransactionFeeExpenses = fm.TransactionFeeExpenses.Add(so.TransactionFeeExpenses)
		// Update the other financial values based on the responsibility status.
		switch so.ResponsibilityStatus {
		case unresolved:
			fm.ContractCount++
			fm.PotentialContractCompensation = fm.PotentialContractCompensation.Add(so.ContractCost)
			fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.LockedStorageDeposit)
			fm.PotentialStorageRevenue = fm.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
			fm.RiskedStorageDeposit = fm.RiskedStorageDeposit.Add(so.RiskedStorageDeposit)
			fm.PotentialDownloadBandwidthRevenue = fm.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.PotentialUploadBandwidthRevenue = fm.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		case succeeded:
			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			fm.StorageRevenue = fm.StorageRevenue.Add(so.PotentialStorageRevenue)
			fm.DownloadBandwidthRevenue = fm.DownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.UploadBandwidthRevenue = fm.UploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		case failed:
			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			if !so.RiskedStorageDeposit.IsNeg() {
				// Storage responsibility failed with risked collateral.
				fm.LostRevenue = fm.LostRevenue.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
				fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.RiskedStorageDeposit)
			}
		}
	}

	h.financialMetrics = fm
	return nil
}

//Handling storage responsibilities in the task queue
func (h *StorageHost) threadedHandleTaskItem(soid common.Hash) {
	if err := h.tm.Add(); err != nil {
		return
	}
	defer h.tm.Done()

	// Lock the storage responsibility
	h.checkAndLockStorageResponsibility(soid)
	defer func() {
		h.checkAndUnlockStorageResponsibility(soid)
	}()

	// Fetch the storage Responsibility associated with the storage responsibility id.
	h.lock.RLock()
	so, err := getStorageResponsibility(h.db, soid)
	h.lock.RUnlock()
	if err != nil {
		h.log.Warn("Could not get storage Responsibility", "err", err)
		return
	}

	//Skip if the storage responsibility has been completed
	if so.ResponsibilityStatus != unresolved {
		return
	}

	if !so.CreateContractConfirmed {
		if h.blockHeight > so.expiration() {
			h.log.Info("If the storage contract has expired and the contract transaction has not been confirmed, delete the storage responsibility", "id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, rejected)
			if err != nil {
				h.log.Warn("failed to delete storage responsibility", "err", err)
			}
			h.lock.Unlock()
			return
		}

		//It is possible that the signing of the storage contract transaction has not yet been executed, and it is waiting in the task queue.
		h.lock.Lock()
		err := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Warn("Error queuing task item", "err", err)
		}
		return
	}

	//If revision meets the condition, a revision transaction will be submitted.
	if !so.StorageRevisionConfirmed && len(so.StorageContractRevisions) > 0 && h.blockHeight >= so.expiration()-postponedExecutionBuffer {
		if h.blockHeight > so.expiration() {
			h.log.Info("If the storage contract has expired and the revision transaction has not been confirmed, delete the storage responsibility", "id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, rejected)
			if err != nil {
				h.log.Warn("failed to delete storage responsibility", "err", err)
			}
			h.lock.Unlock()
			return
		}

		//It is possible that the signing of the storage contract transaction has not yet been executed, and it is waiting in the task queue.
		h.lock.Lock()
		err := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Warn("Error queuing action item", "err", err)
		}

		scrv := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]
		scBytes, err := rlp.EncodeToBytes(scrv)
		if err != nil {
			h.log.Warn("Error when serializing revision", "err", err)
			return
		}

		//The host sends a revision transaction to the transaction pool.
		if _, err := h.SendStorageContractRevisionTx(scrv.NewValidProofOutputs[1].Address, scBytes); err != nil {
			h.log.Warn("Error sending a revision transaction", "err", err)
			return
		}
	}

	//If revision meets the condition, a proof transaction will be submitted.
	if !so.StorageProofConfirmed && h.blockHeight >= so.expiration()+postponedExecution {
		h.log.Warn("The host is ready to submit a proof of transaction", "id", so.id())

		if len(so.SectorRoots) == 0 {
			h.log.Warn("The sector is empty and no storage operation appears", "id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, succeeded)
			h.lock.Unlock()
			if err != nil {
				h.log.Warn("Error removing storage Responsibility", "err", err)
			}
			return
		}

		if so.proofDeadline() < h.blockHeight {
			h.log.Info("If the storage contract has expired and the proof transaction has not been confirmed, delete the storage responsibility", "id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, failed)
			h.lock.Unlock()
			if err != nil {
				h.log.Warn("Error removing storage Responsibility", "err", err)
			}
			return
		}

		//The storage host side gets the index of the data containing the segment
		segmentIndex, err := h.storageProofSegment(so.OriginStorageContract)
		if err != nil {
			h.log.Warn("An error occurred while getting the storage certificate from the storage host", "err", err)
			return
		}

		sectorIndex := segmentIndex / (storage.SectorSize / merkle.LeafSize)
		sectorRoot := so.SectorRoots[sectorIndex]
		sectorBytes, err := h.ReadSector(sectorRoot)
		//No content can be read from the memory, indicating that the storage host is not storing.
		if err != nil {
			h.log.Warn("the storage host is not storing", "err", err)
			return
		}

		//Build a storage certificate for this storage contract
		sectorSegment := segmentIndex % (storage.SectorSize / merkle.LeafSize)
		base, cachedHashSet := MerkleProof(sectorBytes, sectorSegment)
		// Using the sector, build a cached root.
		log2SectorSize := uint64(0)
		for 1<<log2SectorSize < (storage.SectorSize / merkle.LeafSize) {
			log2SectorSize++
		}
		ct := merkle.NewCachedTree(log2SectorSize)
		err = ct.SetIndex(segmentIndex)
		if err != nil {
			h.log.Warn("cannot call SetIndex on Tree ", "err", err)
		}
		for _, root := range so.SectorRoots {
			ct.Push(root)
		}
		hashSet := ct.Prove(base, cachedHashSet)
		sp := types.StorageProof{
			ParentID: so.id(),
			HashSet:  hashSet,
		}
		copy(sp.Segment[:], base)

		//Here take the address of the storage host in the storage contract book
		fromAddress := so.OriginStorageContract.ValidProofOutputs[1].Address
		account := accounts.Account{Address: fromAddress}
		wallet, err := h.ethBackend.AccountManager().Find(account)
		if err != nil {
			h.log.Warn("There was an error opening the wallet", "err", err)
			return
		}
		spSign, err := wallet.SignHash(account, sp.RLPHash().Bytes())
		if err != nil {
			h.log.Warn("Error when sign data", "err", err)
			return
		}
		sp.Signature = spSign

		spBytes, err := rlp.EncodeToBytes(sp)
		if err != nil {
			h.log.Warn("Error when serializing proof", "err", err)
			return
		}

		//The host sends a storage proof transaction to the transaction pool.
		if _, err := h.SendStorageProofTx(fromAddress, spBytes); err != nil {
			h.log.Warn("Error sending a storage proof transaction", "err", err)
			return
		}

		h.lock.Lock()
		//Insert the check proof task in the task queue.
		err = h.queueTaskItem(so.proofDeadline(), so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Warn("Error queuing task item", err)
		}
	}

	// Save the storage Responsibility.
	errDB := putStorageResponsibility(h.db, soid, so)
	if errDB != nil {
		h.log.Warn("Error updating the storage Responsibility", errDB)
	}

	//If the submission of the storage certificate is successful during the non-expiration period, this deletes the storage responsibility
	if so.StorageProofConfirmed && h.blockHeight >= so.proofDeadline() {
		h.log.Info("This storage responsibility is responsible for the completion of the storage contract", "id", so.id())
		h.lock.Lock()
		err := h.removeStorageResponsibility(so, succeeded)
		if err != nil {
			h.log.Warn("failed to delete storage responsibility", "err", err)
		}
		h.lock.Unlock()
	}

}

//MerkleProof get the storage proof
func MerkleProof(b []byte, proofIndex uint64) (base []byte, hashSet []common.Hash) {
	t := merkle.NewTree()
	//This error doesn't mean anything to us.
	t.SetIndex(proofIndex)

	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		t.Push(buf.Next(merkle.LeafSize))
	}

	// Get the storage proof
	_, proof, _, _ := t.Prove()
	if len(proof) == 0 {
		//If there is no data, it will return a blank value
		return nil, nil
	}

	base = proof[0]
	hashSet = make([]common.Hash, len(proof)-1)
	for i, p := range proof[1:] {
		copy(hashSet[i][:], p)
	}

	return base, hashSet
}

//If it exists, return the index of the segment in the storage contract that needs to be proved
func (h *StorageHost) storageProofSegment(fc types.StorageContract) (uint64, error) {
	fcid := fc.RLPHash()
	triggerHeight := fc.WindowStart - 1

	block, errGetHeight := h.ethBackend.GetBlockByNumber(triggerHeight)
	if errGetHeight != nil {
		return 0, errGetHeight
	}

	triggerID := block.Hash()
	seed := crypto.Keccak256Hash(triggerID[:], fcid[:])
	numSegments := int64(calculateLeaves(fc.FileSize))
	seedInt := new(big.Int).SetBytes(seed[:])
	index := seedInt.Mod(seedInt, big.NewInt(numSegments)).Uint64()

	return index, nil
}

func calculateLeaves(dataSize uint64) uint64 {
	numSegments := dataSize / merkle.LeafSize
	if dataSize == 0 || dataSize%merkle.LeafSize != 0 {
		numSegments++
	}
	return numSegments
}

//HostBlockHeightChange handle when a new block is generated or a block is rolled back
func (h *StorageHost) HostBlockHeightChange(cce core.ChainChangeEvent) {

	h.lock.Lock()
	defer h.lock.Unlock()

	//Handling rolled back blocks
	h.RevertedBlockHashesStorageResponsibility(cce.RevertedBlockHashes)

	//Block executing the main chain
	taskItems := h.ApplyBlockHashesStorageResponsibility(cce.AppliedBlockHashes)
	for i := range taskItems {
		go h.threadedHandleTaskItem(taskItems[i])
	}

	err := h.syncConfig()
	if err != nil {
		h.log.Warn("could not save during ProcessConsensusChange", "err", err)
	}

}

// subscribeChainChangeEvent will receive changes on the block chain (blocks added / reverted)
// once received, a function will be triggered to analyze those blocks
func (h *StorageHost) subscribeChainChangEvent() {
	if err := h.tm.Add(); err != nil {
		return
	}
	defer h.tm.Done()

	chainChanges := make(chan core.ChainChangeEvent, 100)
	h.ethBackend.SubscribeChainChangeEvent(chainChanges)

	for {
		select {
		case change := <-chainChanges:
			h.HostBlockHeightChange(change)
		case <-h.tm.StopChan():
			return
		}
	}
}

//ApplyBlockHashesStorageResponsibility block executing the main chain
func (h *StorageHost) ApplyBlockHashesStorageResponsibility(blocks []common.Hash) []common.Hash {
	var taskItems []common.Hash
	for _, blockApply := range blocks {
		//apply contract transaction
		ContractCreateIDsApply, revisionIDsApply, storageProofIDsApply, number, errGetBlock := h.GetAllStorageContractIDsWithBlockHash(blockApply)
		if errGetBlock != nil {
			continue
		}

		//Traverse all contract transactions and modify storage responsibility status
		for _, id := range ContractCreateIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.CreateContractConfirmed = true
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Warn(errPutStorageResponsibility, "err", errPut)
				continue
			}
		}

		//Traverse all revision transactions and modify storage responsibility status
		for key, value := range revisionIDsApply {
			so, errGet := getStorageResponsibility(h.db, key)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			if len(so.StorageContractRevisions) < 1 {
				h.log.Warn("Storage contract cannot get revisions", "id", so.id())
				continue
			}
			//To prevent vicious attacks, determine the consistency of the revision number.
			if value == so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewRevisionNumber {
				so.StorageRevisionConfirmed = true
			}
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Warn(errPutStorageResponsibility, "err", errPut)
				continue
			}
		}

		//Traverse all storageProof transactions and modify storage responsibility status
		for _, id := range storageProofIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.StorageProofConfirmed = true
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Warn(errPutStorageResponsibility, "err", errPut)
				continue
			}
		}

		if number != 0 {
			h.blockHeight++
		}
		existingItems, err := GetHeight(h.db, h.blockHeight)
		if err != nil {
			continue
		}

		// From the existing items, pull out a storage responsibility.
		knownActionItems := make(map[common.Hash]struct{})
		responsibilityIDs := make([]common.Hash, len(existingItems)/common.HashLength)
		for i := 0; i < len(existingItems); i += common.HashLength {
			copy(responsibilityIDs[i/common.HashLength][:], existingItems[i:i+common.HashLength])
		}
		for _, soid := range responsibilityIDs {
			_, exists := knownActionItems[soid]
			if !exists {
				taskItems = append(taskItems, soid)
				knownActionItems[soid] = struct{}{}
			}
		}

	}
	return taskItems
}

//RevertedBlockHashesStorageResponsibility handling rolled back blocks
func (h *StorageHost) RevertedBlockHashesStorageResponsibility(blocks []common.Hash) {
	for _, blockReverted := range blocks {
		//Rollback contract transaction
		ContractCreateIDs, revisionIDs, storageProofIDs, number, errGetBlock := h.GetAllStorageContractIDsWithBlockHash(blockReverted)
		if errGetBlock != nil {
			h.log.Warn("Failed to get the data from the block as expected ", "err", errGetBlock)
			continue
		}

		//Traverse all ContractCreate transactions and modify storage responsibility status
		for _, id := range ContractCreateIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.CreateContractConfirmed = false
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Warn(errPutStorageResponsibility, "err", errPut)
				continue
			}
		}

		//Traverse all revision transactions and modify storage responsibility status
		for key := range revisionIDs {
			so, errGet := getStorageResponsibility(h.db, key)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.StorageRevisionConfirmed = false
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Warn(errPutStorageResponsibility, "err", errPut)
				continue
			}
		}

		//Traverse all storageProof transactions and modify storage responsibility status
		for _, id := range storageProofIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.StorageProofConfirmed = false
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Warn(errPutStorageResponsibility, "err", errPut)
				continue
			}
		}

		if number != 0 && h.blockHeight > 1 {
			h.blockHeight--
		}

	}

}

//GetAllStorageContractIDsWithBlockHash analyze the block structure and get three kinds of transaction collections: contractCreate, revision, and proof、block height.
func (h *StorageHost) GetAllStorageContractIDsWithBlockHash(blockHash common.Hash) (ContractCreateIDs []common.Hash, revisionIDs map[common.Hash]uint64, storageProofIDs []common.Hash, number uint64, errGet error) {
	revisionIDs = make(map[common.Hash]uint64)
	precompiled := vm.PrecompiledEVMFileContracts
	block, err := h.ethBackend.GetBlockByHash(blockHash)
	if err != nil {
		errGet = err
		return
	}
	number = block.NumberU64()
	txs := block.Transactions()
	for _, tx := range txs {
		p, ok := precompiled[*tx.To()]
		if !ok {
			continue
		}
		switch p {
		case vm.ContractCreateTransaction:
			var sc types.StorageContract
			err := rlp.DecodeBytes(tx.Data(), &sc)
			if err != nil {
				h.log.Warn("Error when serializing storage contract:", "err", err)
				continue
			}
			ContractCreateIDs = append(ContractCreateIDs, sc.RLPHash())
		case vm.CommitRevisionTransaction:
			var scr types.StorageContractRevision
			err := rlp.DecodeBytes(tx.Data(), &scr)
			if err != nil {
				h.log.Warn("Error when serializing revision:", "err", err)
				continue
			}
			revisionIDs[scr.ParentID] = scr.NewRevisionNumber
		case vm.StorageProofTransaction:
			var sp types.StorageProof
			err := rlp.DecodeBytes(tx.Data(), &sp)
			if err != nil {
				h.log.Warn("Error when serializing proof:", "err", err)
				continue
			}
			storageProofIDs = append(storageProofIDs, sp.ParentID)
		default:
			continue
		}
	}
	return
}

// StorageResponsibilities fetches the set of storage Responsibility in the host and
// returns metadata on them.
func (h *StorageHost) StorageResponsibilities() (sos []StorageResponsibility) {
	if len(h.lockedStorageResponsibility) < 1 {
		return nil
	}

	for i := range h.lockedStorageResponsibility {
		so, err := getStorageResponsibility(h.db, i)
		if err != nil {
			h.log.Warn(errGetStorageResponsibility, "err", err)
			continue
		}

		sos = append(sos, so)
	}

	return sos
}

// SendStorageContractRevisionTx send revision contract tx
func (h *StorageHost) SendStorageContractRevisionTx(from common.Address, input []byte) (common.Hash, error) {
	return h.parseAPI.StorageTx.SendContractRevisionTX(from, input)
}

// SendStorageProofTx send storage proof tx
func (h *StorageHost) SendStorageProofTx(from common.Address, input []byte) (common.Hash, error) {
	return h.parseAPI.StorageTx.SendStorageProofTX(from, input)
}

//putStorageResponsibility storage storageResponsibility from DB
func putStorageResponsibility(db ethdb.Database, storageContractID common.Hash, so StorageResponsibility) error {
	scdb := ethdb.StorageContractDB{db}
	data, err := rlp.EncodeToBytes(so)
	if err != nil {
		return err
	}
	return scdb.StoreWithPrefix(storageContractID, data, PrefixStorageResponsibility)
}

//deleteStorageResponsibility delete storageResponsibility from DB
func deleteStorageResponsibility(db ethdb.Database, storageContractID common.Hash) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(storageContractID, PrefixStorageResponsibility)
}

//getStorageResponsibility get storageResponsibility from DB
func getStorageResponsibility(db ethdb.Database, storageContractID common.Hash) (StorageResponsibility, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(storageContractID, PrefixStorageResponsibility)
	if err != nil {
		return StorageResponsibility{}, err
	}
	var so StorageResponsibility
	err = rlp.DecodeBytes(valueBytes, &so)
	if err != nil {
		return StorageResponsibility{}, err
	}
	return so, nil
}

//StoreHeight storage task by block height
func StoreHeight(db ethdb.Database, storageContractID common.Hash, height uint64) error {
	scdb := ethdb.StorageContractDB{db}

	existingItems, err := GetHeight(db, height)
	if err != nil {
		existingItems = make([]byte, 0)
	}

	existingItems = append(existingItems, storageContractID[:]...)

	return scdb.StoreWithPrefix(height, existingItems, PrefixHeight)
}

//DeleteHeight delete task by block height
func DeleteHeight(db ethdb.Database, height uint64) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(height, PrefixHeight)
}

//GetHeight get the task by block height
func GetHeight(db ethdb.Database, height uint64) ([]byte, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(height, PrefixHeight)
	if err != nil {
		return nil, err
	}

	return valueBytes, nil
}
