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
	postponedExecutionBuffer    = uint64(144)
	postponedExecution          = 3
	responseTimeout             = 40
	PrefixStorageResponsibility = "StorageResponsibility-" //DB Prefix for StorageResponsibility
	PrefixHeight                = "height-"                //DB Prefix for task
	errGetStorageResponsibility = "failed to get data from DB as I wished "
	errPutStorageResponsibility = "failed to get data from DB as I wished "
)

//Storage contract should not be empty
var emptyStorageContract = types.StorageContract{}

const (
	unresolved storageResponsibilityStatus = iota //Storage responsibility is initialization, no meaning
	rejected                                      //Storage responsibility never begins
	succeeded                                     // Successful storage responsibility
	failed                                        //failed storage responsibility
)

var (
	errEmptyOriginStorageContract = errors.New("storage contract has no storage responsibility")
	errEmptyRevisionSet           = errors.New("take the last revision ")
	errInsaneRevision             = errors.New("revision is not necessary")
	errNotAllowed                 = errors.New("time is not allowed")
	errTransactionNotConfirmed    = errors.New("transaction not confirmed")
)

type (
	//Storage contract management and maintenance on the storage host side
	StorageResponsibility struct {
		//Store the root set of related metadata
		SectorRoots       []common.Hash
		StorageContractID common.Hash

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

		FormContractConfirmed      bool
		StorageProofConfirmed      bool
		StorageProofConstructed    bool
		StorageRevisionConfirmed   bool
		StorageRevisionConstructed bool
	}

	storageResponsibilityStatus uint64
)

func (i storageResponsibilityStatus) String() string {
	if i == 0 {
		return "unresolved"
	}
	if i == 1 {
		return "rejected"
	}
	if i == 2 {
		return "succeeded"
	}
	if i == 3 {
		return "failed"
	}
	return "storageResponsibilityStatus(" + strconv.FormatInt(int64(i), 10) + ")"
}

func getStorageResponsibility(db ethdb.Database, sc common.Hash) (StorageResponsibility, error) {
	so, errGet := GetStorageResponsibility(db, sc)
	if errGet != nil {
		return StorageResponsibility{}, errGet
	}
	return so, nil
}

func putStorageResponsibility(db ethdb.Database, so StorageResponsibility) error {
	err := StoreStorageResponsibility(db, so.id(), so)
	if err != nil {
		return err
	}
	return nil
}

func deleteStorageResponsibility(db ethdb.Database, sc common.Hash) error {
	err := DeleteStorageResponsibility(db, sc)
	if err != nil {
		return err
	}
	return nil
}

//returns expired block number
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
	return so.StorageContractID
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

	err := StoreHeight(h.db, id, height)
	if err != nil {
		return err
	}

	return nil
}

//InsertStorageResponsibility insert a storage Responsibility to the storage host.
func (h *StorageHost) InsertStorageResponsibility(so StorageResponsibility) error {
	err := func() error {
		h.lock.Lock()
		defer h.lock.Unlock()
		if _, ok := h.lockedStorageResponsibility[so.id()]; ok {
			h.log.Info("insertStorageResponsibility called with an responsibility that is not locked")
		}

		//Submit revision time exceeds storage responsibility expiration time
		if h.blockHeight+postponedExecutionBuffer >= so.expiration() {
			h.log.Crit("failed to submit revision in storage responsibility due date")
			return errNotAllowed
		}

		//Not enough time to submit proof of storage, no need to put in the task force
		if so.expiration()+postponedExecution >= so.proofDeadline() {
			h.log.Crit("Not enough time to submit proof of storage")
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
			errPut := StoreStorageResponsibility(h.db, so.StorageContractID, so)
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
	//insert the check form contract task in the task queue.
	err1 := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
	err2 := h.queueTaskItem(h.blockHeight+postponedExecution*2, so.id())

	//insert the check revision task in the task queue.
	err3 := h.queueTaskItem(so.expiration()-postponedExecutionBuffer, so.id())
	err4 := h.queueTaskItem(so.expiration()-postponedExecutionBuffer+postponedExecution, so.id())

	//insert the check proof task in the task queue.
	err5 := h.queueTaskItem(so.expiration()+postponedExecution, so.id())
	err6 := h.queueTaskItem(so.expiration()+postponedExecution*2, so.id())
	err = common.ErrCompose(err1, err2, err3, err4, err5, err6)
	if err != nil {
		h.log.Info("Error with task item, redacting responsibility, id", so.id())
		return common.ErrCompose(err, h.removeStorageResponsibility(so, rejected))
	}

	return nil
}

//the virtual sector will need to appear in 'sectorsRemoved' multiple times. Same with 'sectorsGained'。
func (h *StorageHost) modifyStorageResponsibility(so StorageResponsibility, sectorsRemoved []common.Hash, sectorsGained []common.Hash, gainedSectorData [][]byte) error {
	if _, ok := h.lockedStorageResponsibility[so.id()]; ok {
		h.log.Info("modifyStorageResponsibility called with an responsibility that is not locked")
	}

	//Need enough time to submit revision
	if so.expiration()-postponedExecutionBuffer <= h.blockHeight {
		return errNotAllowed
	}

	//sectorsGained and gainedSectorData must have the same length
	if len(sectorsGained) != len(gainedSectorData) {
		h.log.Crit("sectorsGained length : ", len(sectorsGained), " and gainedSectorData length:", len(gainedSectorData))
		return errInsaneRevision
	}

	for _, data := range gainedSectorData {
		//No 4MB sector has no meaning
		if uint64(len(data)) != storage.SectorSize {
			h.log.Crit("No 4MB sector has no meaning,sector size length ", len(data))
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
			h.log.Crit("Error writing data to the sector", err)
			break
		}
	}
	//This operation is wrong, you need to restore the sector
	if err != nil {
		for j := 0; j < i; j++ {
			//The error of restoring a sector doesn't make any sense to us.
			h.RemoveSector(sectorsGained[j])
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

		errOld = putStorageResponsibility(h.db, so)
		if errOld != nil {
			return errOld
		}
		return nil
	}()

	if errDBso != nil {
		//This operation is wrong, you need to restore the sector
		for i := range sectorsGained {
			//The error of restoring a sector doesn't make any sense to us.
			h.RemoveSector(sectorsGained[i])
		}
		return errDBso
	}
	//Delete the deleted sector
	for k := range sectorsRemoved {
		//The error of restoring a sector doesn't make any sense to us.
		h.RemoveSector(sectorsRemoved[k])
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

//Remove stale storage responsibilities because these storage responsibilities will affect the financial metrics of the host
func (h *StorageHost) PruneStaleStorageResponsibilities() error {
	sos := h.StorageResponsibilities()
	var scids []common.Hash
	for _, so := range sos {
		if h.blockHeight > so.NegotiationBlockNumber+responseTimeout {
			scids = append(scids, so.StorageContractID)
		}
		if so.FormContractConfirmed == false {
			return errTransactionNotConfirmed
		}
	}

	// Delete storage responsibility from the database.
	err := h.deleteStorageResponsibilities(scids)
	if err != nil {
		h.log.Info("unable to delete responsibility: ", err)
		return err
	}

	// Update the financial metrics of the host.
	err = h.resetFinancialMetrics()
	if err != nil {
		h.log.Info("unable to reset host financial metrics: ", err)
		return err
	}

	return nil
}

//No matter what state the storage responsibility will be deleted
func (h *StorageHost) removeStorageResponsibility(so StorageResponsibility, sos storageResponsibilityStatus) error {

	//Unchecked error, even if there is an error, we want to delete
	h.RemoveSectorBatch(so.SectorRoots)

	if sos == unresolved {
		h.log.Info("storage responsibility 'unresolved' during call to removeStorageResponsibility, id", so.id())
	}
	if sos == rejected {
		if h.financialMetrics.TransactionFeeExpenses.Cmp(so.TransactionFeeExpenses) >= 0 {
			h.log.Info("Rejecting storage responsibility expiring at block ", so.expiration(), ", current height is ", h.blockHeight, ". Potential revenue is ", h.financialMetrics.PotentialContractCompensation.Add(h.financialMetrics.PotentialStorageRevenue).Add(h.financialMetrics.PotentialDownloadBandwidthRevenue).Add(h.financialMetrics.PotentialUploadBandwidthRevenue))

			// Remove the responsibility statistics as potential risk and income.
			h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
			h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedStorageDeposit)
			h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
			h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
			h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
			h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedStorageDeposit)
			h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Sub(so.TransactionFeeExpenses)
		}
	}

	if sos == succeeded {
		revenue := so.ContractCost.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
		//No storage responsibility for file upload or download does not require proof of storage
		if len(so.SectorRoots) == 0 {
			h.log.Info("No need to submit a storage proof for empty storage contract. Revenue is ", revenue)
		} else {
			h.log.Info("Successfully submitted a storage proof. Revenue is ", revenue)
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

	}

	if sos == failed {
		// Remove the responsibility statistics as potential risk and income.
		h.log.Info("Missed storage proof. Revenue would have been", so.ContractCost.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue))

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
	errDb := StoreStorageResponsibility(h.db, so.id(), so)
	if errDb != nil {
		return errDb
	}

	return nil
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
		if so.ResponsibilityStatus == unresolved {
			fm.ContractCount++
			fm.PotentialContractCompensation = fm.PotentialContractCompensation.Add(so.ContractCost)
			fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.LockedStorageDeposit)
			fm.PotentialStorageRevenue = fm.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
			fm.RiskedStorageDeposit = fm.RiskedStorageDeposit.Add(so.RiskedStorageDeposit)
			fm.PotentialDownloadBandwidthRevenue = fm.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.PotentialUploadBandwidthRevenue = fm.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		}
		if so.ResponsibilityStatus == succeeded {
			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			fm.StorageRevenue = fm.StorageRevenue.Add(so.PotentialStorageRevenue)
			fm.DownloadBandwidthRevenue = fm.DownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.UploadBandwidthRevenue = fm.UploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		}
		if so.ResponsibilityStatus == failed {
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

	// Lock the storage responsibility
	h.checkAndLockStorageResponsibility(soid)
	defer func() {
		h.checkAndUnlockStorageResponsibility(soid)
	}()

	// Fetch the storage Responsibility associated with the storage responsibility id.
	h.lock.RLock()
	so, err := getStorageResponsibility(h.db, soid)
	if err != nil {
		h.log.Info("Could not get storage Responsibility:", err)
		return
	}

	//Skip if the storage obligation has been completed
	if so.ResponsibilityStatus != unresolved {
		return
	}

	if !so.FormContractConfirmed {
		if h.blockHeight > so.expiration() {
			h.log.Info("If the storage contract has expired and the contract transaction has not been confirmed, delete the storage responsibility, id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, rejected)
			if err != nil {
				h.log.Info("failed to delete storage responsibility", err)
			}
			h.lock.Unlock()
			return
		}

		//It is possible that the signing of the storage contract transaction has not yet been executed, and it is waiting in the task queue.
		h.lock.Lock()
		err := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Info("Error queuing task item:", err)
		}
		return
	}

	//If revision meets the condition, a revision transaction will be submitted.
	if !so.StorageRevisionConfirmed && len(so.StorageContractRevisions) > 0 && h.blockHeight >= so.expiration()-postponedExecutionBuffer {
		if h.blockHeight > so.expiration() {
			h.log.Info("If the storage contract has expired and the revision transaction has not been confirmed, delete the storage responsibility, id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, rejected)
			if err != nil {
				h.log.Info("failed to delete storage responsibility", err)
			}
			h.lock.Unlock()
			return
		}

		//It is possible that the signing of the storage contract transaction has not yet been executed, and it is waiting in the task queue.
		h.lock.Lock()
		err := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Info("Error queuing action item:", err)
		}

		scrv := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]
		scBytes, err := rlp.EncodeToBytes(scrv)
		if err != nil {
			h.log.Info("Error when serializing revision:", err)
			return
		}

		//The host sends a revision transaction to the transaction pool.
		if _, err := storage.SendContractRevisionTX(h.b, scrv.NewValidProofOutputs[1].Address, scBytes); err != nil {
			h.log.Info("Error sending a revision transaction:", err)
			return
		}
	}

	//If revision meets the condition, a proof transaction will be submitted.
	if !so.StorageProofConfirmed && h.blockHeight >= so.expiration()+postponedExecution {
		h.log.Debug("The host is ready to submit a proof of transaction,id: ", so.id())

		if len(so.SectorRoots) == 0 {
			h.log.Debug("The sector is empty and no storage operation appears, id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, succeeded)
			h.lock.Unlock()
			if err != nil {
				h.log.Info("Error removing storage Responsibility:", err)
			}
			return
		}

		if so.proofDeadline() < h.blockHeight {
			h.log.Info("If the storage contract has expired and the proof transaction has not been confirmed, delete the storage responsibility, id", so.id())
			h.lock.Lock()
			err := h.removeStorageResponsibility(so, failed)
			h.lock.Unlock()
			if err != nil {
				h.log.Info("Error removing storage Responsibility:", err)
			}
			return
		}

		//The storage host side gets the index of the data containing the segment
		segmentIndex, err := h.storageProofSegment(so.OriginStorageContract)
		if err != nil {
			h.log.Debug("An error occurred while getting the storage certificate from the storage host:", err)
			return
		}

		sectorIndex := segmentIndex / (storage.SectorSize / merkle.LeafSize)
		sectorRoot := so.SectorRoots[sectorIndex]
		sectorBytes, err := h.ReadSector(sectorRoot)
		//No content can be read from the memory, indicating that the storage host is not storing.
		if err != nil {
			h.log.Debug("the storage host is not storing, error:", err)
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
			h.log.Crit("cannot call SetIndex on Tree ", err)
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
			h.log.Info("There was an error opening the wallet:", err)
			return
		}
		spSign, err := wallet.SignHash(account, sp.RLPHash().Bytes())
		if err != nil {
			h.log.Info("Error when sign data:", err)
			return
		}
		sp.Signature = spSign

		scBytes, err := rlp.EncodeToBytes(sp)
		if err != nil {
			h.log.Info("Error when serializing proof:", err)
			return
		}

		//The host sends a storage proof transaction to the transaction pool.
		if _, err := storage.SendStorageProofTX(h.b, fromAddress, scBytes); err != nil {
			h.log.Info("Error sending a storage proof transaction:", err)
			return
		}

		h.lock.Lock()
		//insert the check proof task in the task queue.
		err = h.queueTaskItem(so.proofDeadline(), so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Info("Error queuing task item:", err)
		}
	}

	// Save the storage Responsibility.
	errDB := StoreStorageResponsibility(h.db, soid, so)
	if errDB != nil {
		h.log.Info("Error updating the storage Responsibility", errDB)
	}

	//If the submission of the storage certificate is successful during the non-expiration period, this deletes the storage responsibility
	if so.StorageProofConfirmed && h.blockHeight >= so.proofDeadline() {
		h.log.Info("This storage responsibility is responsible for the completion of the storage contract, id", so.id())
		h.lock.Lock()
		err := h.removeStorageResponsibility(so, succeeded)
		if err != nil {
			h.log.Info("failed to delete storage responsibility", err)
		}
		h.lock.Unlock()
	}

}

// Get the stooge proof
func MerkleProof(b []byte, proofIndex uint64) (base []byte, hashSet []common.Hash) {
	t := merkle.NewTree()
	//This error doesn't mean anything to us.
	t.SetIndex(proofIndex)

	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		t.Push(buf.Next(merkle.LeafSize))
	}

	// Get the stooge proof
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
	triggerHerght := fc.WindowStart - 1

	block, errGetHeight := h.ethBackend.GetBlockByNumber(triggerHerght)
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

//Handle when a new block is generated or a block is rolled back
func (h *StorageHost) ProcessConsensusChange(cce core.ChainChangeEvent) {

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
		h.log.Info("ERROR: could not save during ProcessConsensusChange:", err)
	}

}

//Block executing the main chain
func (h *StorageHost) ApplyBlockHashesStorageResponsibility(blocks []common.Hash) []common.Hash {
	var taskItems []common.Hash
	for _, blockApply := range blocks {
		//apply contract transaction
		formContractIDsApply, revisionIDsApply, storageProofIDsApply, number, errGetBlock := h.GetAllStorageContractIDsWithBlockHash(blockApply)
		if errGetBlock != nil {
			continue
		}

		for _, id := range formContractIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			if errGet != nil {
				h.log.Crit(errGetStorageResponsibility, errGet)
				continue
			}
			so.FormContractConfirmed = true
			errPut := putStorageResponsibility(h.db, so)
			if errPut != nil {
				h.log.Crit(errPutStorageResponsibility, errPut)
				continue
			}
		}

		for _, id := range revisionIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			if errGet != nil {
				h.log.Crit(errGetStorageResponsibility, errGet)
				continue
			}
			so.StorageRevisionConfirmed = true
			errPut := putStorageResponsibility(h.db, so)
			if errPut != nil {
				h.log.Crit(errPutStorageResponsibility, errPut)
				continue
			}
		}

		for _, id := range storageProofIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			if errGet != nil {
				h.log.Crit(errGetStorageResponsibility, errGet)
				continue
			}
			so.StorageProofConfirmed = true
			errPut := putStorageResponsibility(h.db, so)
			if errPut != nil {
				h.log.Crit(errPutStorageResponsibility, errPut)
				continue
			}
		}

		if number != 0 {
			h.blockHeight++
		}
		existingTtems, err := GetHeight(h.db, h.blockHeight)
		if err != nil {
			continue
		}

		// From the existing items, pull out a storage responsibility.
		knownActionItems := make(map[common.Hash]struct{})
		responsibilityIDs := make([]common.Hash, len(existingTtems)/common.HashLength)
		for i := 0; i < len(existingTtems); i += common.HashLength {
			copy(responsibilityIDs[i/common.HashLength][:], existingTtems[i:i+common.HashLength])
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

//Handling rolled back blocks
func (h *StorageHost) RevertedBlockHashesStorageResponsibility(blocks []common.Hash) {
	for _, blockReverted := range blocks {
		//Rollback contract transaction
		formContractIDs, revisionIDs, storageProofIDs, number, errGetBlock := h.GetAllStorageContractIDsWithBlockHash(blockReverted)
		if errGetBlock != nil {
			h.log.Crit("Failed to get the data from the block as expected ", errGetBlock)
			continue
		}
		for _, id := range formContractIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			if errGet != nil {
				h.log.Crit(errGetStorageResponsibility, errGet)
				continue
			}
			so.FormContractConfirmed = false
			errPut := putStorageResponsibility(h.db, so)
			if errPut != nil {
				h.log.Crit(errPutStorageResponsibility, errPut)
				continue
			}
		}

		for _, id := range revisionIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			if errGet != nil {
				h.log.Crit(errGetStorageResponsibility, errGet)
				continue
			}
			so.StorageRevisionConfirmed = false
			errPut := putStorageResponsibility(h.db, so)
			if errPut != nil {
				h.log.Crit(errPutStorageResponsibility, errPut)
				continue
			}
		}

		for _, id := range storageProofIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			if errGet != nil {
				h.log.Crit(errGetStorageResponsibility, errGet)
				continue
			}
			so.StorageProofConfirmed = false
			errPut := putStorageResponsibility(h.db, so)
			if errPut != nil {
				h.log.Crit(errPutStorageResponsibility, errPut)
				continue
			}
		}

		if number != 0 && h.blockHeight > 0 {
			h.blockHeight--
		}

	}

}

//Analyze the block structure and get three kinds of transaction collections: contractCreate, revision, and proof、block height.
func (h *StorageHost) GetAllStorageContractIDsWithBlockHash(blockHashs common.Hash) (formContractIDs []common.Hash, revisionIDs []common.Hash, storageProofIDs []common.Hash, number uint64, errGet error) {
	precompiles := vm.PrecompiledEVMFileContracts
	block, err := h.ethBackend.GetBlockByHash(blockHashs)
	if err != nil {
		errGet = err
		return
	}
	number = block.NumberU64()
	txs := block.Transactions()
	for _, tx := range txs {
		p, ok := precompiles[*tx.To()]
		if !ok {
			continue
		}
		switch p {
		case vm.FormContractTransaction:
			var sc types.StorageContract
			err := rlp.DecodeBytes(tx.Data(), &sc)
			if err != nil {
				h.log.Crit("Error when serializing storage contract:", err)
				continue
			}
			formContractIDs = append(formContractIDs, sc.RLPHash())
		case vm.CommitRevisionTransaction:
			var scr types.StorageContractRevision
			err := rlp.DecodeBytes(tx.Data(), &scr)
			if err != nil {
				h.log.Crit("Error when serializing revision:", err)
				continue
			}
			revisionIDs = append(formContractIDs, scr.ParentID)
		case vm.StorageProofTransaction:
			var sp types.StorageProof
			err := rlp.DecodeBytes(tx.Data(), &sp)
			if err != nil {
				h.log.Crit("Error when serializing proof:", err)
				continue
			}
			storageProofIDs = append(formContractIDs, sp.ParentID)
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
		so, err := GetStorageResponsibility(h.db, i)
		if err != nil {
			h.log.Crit(errGetStorageResponsibility, err)
			continue
		}

		sos = append(sos, so)
	}

	return sos
}

//storage storage responsibility from DB
func StoreStorageResponsibility(db ethdb.Database, storageContractID common.Hash, so StorageResponsibility) error {
	scdb := ethdb.StorageContractDB{db}
	data, err := rlp.EncodeToBytes(so)
	if err != nil {
		return err
	}
	return scdb.StoreWithPrefix(storageContractID, data, PrefixStorageResponsibility)
}

//Delete storage responsibility from DB
func DeleteStorageResponsibility(db ethdb.Database, storageContractID common.Hash) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(storageContractID, PrefixStorageResponsibility)
}

//Read storage responsibility from DB
func GetStorageResponsibility(db ethdb.Database, storageContractID common.Hash) (StorageResponsibility, error) {
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

//Storage task by block height
func StoreHeight(db ethdb.Database, storageContractID common.Hash, height uint64) error {
	scdb := ethdb.StorageContractDB{db}

	existingItems, err := GetHeight(db, height)
	if err != nil {
		existingItems = make([]byte, 1)
	}

	existingItems = append(existingItems, storageContractID[:]...)

	return scdb.StoreWithPrefix(storageContractID, existingItems, PrefixHeight)
}

//Delete task by block height
func DeleteHeight(db ethdb.Database, height uint64) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(height, PrefixHeight)
}

//Get the task by block height
func GetHeight(db ethdb.Database, height uint64) ([]byte, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(height, PrefixHeight)
	if err != nil {
		return nil, err
	}

	return valueBytes, nil
}
