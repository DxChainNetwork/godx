// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"bytes"
	"math/big"
	"reflect"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
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
)

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

// storageResponsibilities fetches the set of storage Responsibility in the host and
// returns metadata on them.
func (h *StorageHost) storageResponsibilities() (sos []StorageResponsibility) {
	if len(h.lockedStorageResponsibility) < 1 {
		return nil
	}
	for i := range h.lockedStorageResponsibility {
		so, err := getStorageResponsibility(h.db, i)
		if err != nil {
			h.log.Warn("Failed to get storage responsibility", "err", err)
			continue
		}
		sos = append(sos, so)
	}
	return sos
}

//Schedule a task to execute at the specified block number
func (h *StorageHost) queueTaskItem(height uint64, id common.Hash) error {

	if height < h.blockHeight {
		h.log.Info("It is not appropriate to arrange such a task")
	}

	return storeHeight(h.db, id, height)
}

//insertStorageResponsibility insert a storage Responsibility to the storage host.
func (h *StorageHost) insertStorageResponsibility(so StorageResponsibility) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	err := func() error {
		//Submit revision time exceeds storage responsibility expiration time
		//if h.blockHeight+postponedExecutionBuffer >= so.expiration() {
		//	h.log.Warn("responsibilityFailed to submit revision in storage responsibility due date")
		//	return errNotAllowed
		//}

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
		return common.ErrCompose(err, h.removeStorageResponsibility(so, responsibilityRejected))
	}

	return nil
}

//the virtual sector will need to appear in 'sectorsRemoved' multiple times. Same with 'sectorsGained'。
func (h *StorageHost) modifyStorageResponsibility(so StorageResponsibility, sectorsRemoved []common.Hash, sectorsGained []common.Hash, gainedSectorData [][]byte) error {
	if _, ok := h.lockedStorageResponsibility[so.id()]; !ok {
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

//pruneStaleStorageResponsibilities remove stale storage responsibilities because these storage responsibilities will affect the financial metrics of the host
func (h *StorageHost) pruneStaleStorageResponsibilities() error {
	h.lock.RLock()
	sos := h.storageResponsibilities()
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
	case responsibilityUnresolved:
		h.log.Info("storage responsibility 'responsibilityUnresolved' during call to removeStorageResponsibility", "id", so.id())
	case responsibilityRejected:
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
	case responsibilitySucceeded:
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

	case responsibilityFailed:
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
	sos := h.storageResponsibilities()
	for _, so := range sos {
		// Submit transaction fee first
		fm.TransactionFeeExpenses = fm.TransactionFeeExpenses.Add(so.TransactionFeeExpenses)
		// Update the other financial values based on the responsibility status.
		switch so.ResponsibilityStatus {
		case responsibilityUnresolved:
			fm.ContractCount++
			fm.PotentialContractCompensation = fm.PotentialContractCompensation.Add(so.ContractCost)
			fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.LockedStorageDeposit)
			fm.PotentialStorageRevenue = fm.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
			fm.RiskedStorageDeposit = fm.RiskedStorageDeposit.Add(so.RiskedStorageDeposit)
			fm.PotentialDownloadBandwidthRevenue = fm.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.PotentialUploadBandwidthRevenue = fm.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		case responsibilitySucceeded:
			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			fm.StorageRevenue = fm.StorageRevenue.Add(so.PotentialStorageRevenue)
			fm.DownloadBandwidthRevenue = fm.DownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.UploadBandwidthRevenue = fm.UploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		case responsibilityFailed:
			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			if !so.RiskedStorageDeposit.IsNeg() {
				// Storage responsibility responsibilityFailed with risked collateral.
				fm.LostRevenue = fm.LostRevenue.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
				fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.RiskedStorageDeposit)
			}
		}
	}

	h.financialMetrics = fm
	return nil
}

//Handling storage responsibilities in the task queue
func (h *StorageHost) handleTaskItem(soid common.Hash) {
	// Lock the storage responsibility
	h.checkAndLockStorageResponsibility(soid)
	defer func() {
		h.checkAndUnlockStorageResponsibility(soid)
	}()

	// Fetch the storage Responsibility associated with the storage responsibility id.
	so, err := getStorageResponsibility(h.db, soid)
	if err != nil {
		h.log.Warn("Could not get storage Responsibility", "err", err)
		return
	}

	//Skip if the storage obligation has been completed
	if so.ResponsibilityStatus != responsibilityUnresolved {
		return
	}

	if !so.CreateContractConfirmed {
		if h.blockHeight > so.expiration() {
			h.log.Info("If the storage contract has expired and the contract transaction has not been confirmed, delete the storage responsibility", "id", so.id().String())
			err := h.removeStorageResponsibility(so, responsibilityRejected)
			if err != nil {
				h.log.Warn("responsibilityFailed to delete storage responsibility", "err", err)
			}
			return
		}

		//It is possible that the signing of the storage contract transaction has not yet been executed, and it is waiting in the task queue.
		err := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
		if err != nil {
			h.log.Warn("Error queuing task item", "err", err)
		}
		return
	}

	//If revision meets the condition, a revision transaction will be submitted.
	if !so.StorageRevisionConfirmed && len(so.StorageContractRevisions) > 0 && h.blockHeight >= so.expiration()-postponedExecutionBuffer {
		if h.blockHeight > so.expiration() {
			h.log.Info("If the storage contract has expired and the revision transaction has not been confirmed, delete the storage responsibility", "id", so.id().String())
			err := h.removeStorageResponsibility(so, responsibilityRejected)
			if err != nil {
				h.log.Warn("responsibilityFailed to delete storage responsibility", "err", err)
			}
			return
		}

		//It is possible that the signing of the storage contract transaction has not yet been executed, and it is waiting in the task queue.
		err := h.queueTaskItem(h.blockHeight+postponedExecution, so.id())
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
		if _, err := h.sendStorageContractRevisionTx(scrv.NewValidProofOutputs[1].Address, scBytes); err != nil {
			h.log.Warn("Error sending a revision transaction", "err", err)
			return
		}
	}

	//If revision meets the condition, a proof transaction will be submitted.
	if !so.StorageProofConfirmed && h.blockHeight >= so.expiration()+postponedExecution {
		if len(so.SectorRoots) == 0 {
			h.log.Info("The sector is empty and no storage operation appears", "id", so.id().String())
			err := h.removeStorageResponsibility(so, responsibilitySucceeded)
			if err != nil {
				h.log.Warn("Error removing storage Responsibility", "err", err)
			}
			return
		}

		if so.proofDeadline() < h.blockHeight {
			h.log.Info("If the storage contract has expired and the proof transaction has not been confirmed, delete the storage responsibility", "id", so.id().String())
			err := h.removeStorageResponsibility(so, responsibilityFailed)
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
		base, cachedHashSet := merkleProof(sectorBytes, sectorSegment)
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
		wallet, err := h.am.Find(account)
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
		if _, err := h.sendStorageProofTx(fromAddress, spBytes); err != nil {
			h.log.Warn("Error sending a storage proof transaction", "err", err)
			return
		}

		//Insert the check proof task in the task queue.
		err = h.queueTaskItem(so.proofDeadline(), so.id())
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
		err := h.removeStorageResponsibility(so, responsibilitySucceeded)
		if err != nil {
			h.log.Warn("responsibilityFailed to delete storage responsibility", "err", err)
		}
	}

}

//merkleProof get the storage proof
func merkleProof(b []byte, proofIndex uint64) (base []byte, hashSet []common.Hash) {
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

// sendStorageContractRevisionTx send revision contract tx
func (h *StorageHost) sendStorageContractRevisionTx(from common.Address, input []byte) (common.Hash, error) {
	return h.parseAPI.StorageTx.SendContractRevisionTX(from, input)
}

// SendStorageProofTx send storage proof tx
func (h *StorageHost) sendStorageProofTx(from common.Address, input []byte) (common.Hash, error) {
	return h.parseAPI.StorageTx.SendStorageProofTX(from, input)
}
