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
	SectorSize               = uint64(1 << 22)
	revisionSubmissionBuffer = uint64(144)
	resubmissionTimeout      = 3
	RespendTimeout           = 40
	HashSize                 = 32
	SegmentSize              = 64
	PrefixStorageObligation  = "storageobligation-"
	PrefixHeight             = "height-"
)

var EmptyStorageContract = types.StorageContract{}

const (
	obligationUnresolved storageObligationStatus = iota // Indicatees that an unitialized value was used. Unresolved
	obligationRejected                                  // Indicates that the obligation never got started, no revenue gained or lost.
	obligationSucceeded                                 // Indicates that the obligation was completed, revenues were gained.
	obligationFailed                                    // Indicates that the obligation failed, revenues and collateral were lost.
)

var (
	// errDuplicateStorageObligation is returned when the storage obligation
	// database already has a storage obligation with the provided file
	// contract. This error should only happen in the event of a developer
	// mistake.
	errDuplicateStorageObligation = errors.New("storage obligation has a file contract which conflicts with an existing storage obligation")
	// errInsaneFileContractOutputCounts is returned when a file contract has
	// the wrong number of outputs for either the valid or missed payouts.
	errInsaneFileContractOutputCounts = errors.New("file contract has incorrect number of outputs for the valid or missed payouts")
	// errInsaneFileContractRevisionOutputCounts is returned when a file
	// contract has the wrong number of outputs for either the valid or missed
	// payouts.
	errInsaneFileContractRevisionOutputCounts = errors.New("file contract revision has incorrect number of outputs for the valid or missed payouts")
	// errInsaneOriginSetFileContract is returned is the final transaction of
	// the origin transaction set of a storage obligation does not have a file
	// contract in the final transaction - there should be a file contract
	// associated with every storage obligation.
	errInsaneOriginSetFileContract = errors.New("origin transaction set of storage obligation should have one file contract in the final transaction")
	// original storage contract of storage obligation is empty - there should be a file contract associated
	// with every storage obligation
	ErrEmptyOriginStorageContract = errors.New("origin storage contract of storage obligation is empty")
	// errInsaneRevisionSetRevisionCount is returned if the final transaction
	// in the revision transaction set of a storage obligation has more or less
	// than one file contract revision.
	ErrEmptyRevisionSet = errors.New("storage contract revisions of storage obligation should have one file contract revision in the final transaction")
	// errInsaneStorageObligationRevision is returned if there is an attempted
	// storage obligation revision which does not have sensical inputs.
	errInsaneStorageObligationRevision = errors.New("revision to storage obligation does not make sense")
	// errInsaneStorageObligationRevisionData is returned if there is an
	// attempted storage obligation revision which does not have sensical
	// inputs.
	errInsaneStorageObligationRevisionData = errors.New("revision to storage obligation has insane data")
	// errNoBuffer is returned if there is an attempted storage obligation that
	// needs to have the storage proof submitted in less than
	// revisionSubmissionBuffer blocks.
	errNoBuffer = errors.New("file contract rejected because storage proof window is too close")
	// errNoStorageObligation is returned if the requested storage obligation
	// is not found in the database.
	errNoStorageObligation = errors.New("storage obligation not found in database")
	// errObligationUnlocked is returned when a storage obligation is being
	// removed from lock, but is already unlocked.
	errObligationUnlocked = errors.New("storage obligation is unlocked, and should not be getting unlocked")
	//Transaction not confirmed
	errTransactionNotConfired = errors.New("Transaction not confirmed")
)

type (
	StorageObligation struct {
		// Storage obligations are broken up into ordered atomic sectors that are
		// exactly 4MiB each. By saving the roots of each sector, storage proofs
		// and modifications to the data can be made inexpensively by making use of
		// the merkletree.CachedTree. Sectors can be appended, modified, or deleted
		// and the host can recompute the Merkle root of the whole file without
		// much computational or I/O expense.
		SectorRoots       []common.Hash
		StorageContractID common.Hash
		TransactionId     common.Hash

		// Variables about the file contract that enforces the storage obligation.
		// The origin an revision transaction are stored as a set, where the set
		// contains potentially unconfirmed transactions.
		ContractCost             common.BigInt
		LockedCollateral         common.BigInt
		PotentialDownloadRevenue common.BigInt
		PotentialStorageRevenue  common.BigInt
		PotentialUploadRevenue   common.BigInt
		RiskedCollateral         common.BigInt
		TransactionFeesAdded     common.BigInt

		// The negotiation height specifies the block height at which the file
		// contract was negotiated. If the origin transaction set is not accepted
		// onto the blockchain quickly enough, the contract is pruned from the
		// host. The origin and revision transaction set contain the contracts +
		// revisions as well as all parent transactions. The parents are necessary
		// because after a restart the transaction pool may be emptied out.
		NegotiationHeight        uint64
		OriginStorageContract    types.StorageContract
		StorageContractRevisions []types.StorageContractRevision

		// Variables indicating whether the critical transactions in a storage
		// obligation have been confirmed on the blockchain.
		ObligationStatus    storageObligationStatus
		OriginConfirmed     bool
		ProofConfirmed      bool
		ProofConstructed    bool
		RevisionConfirmed   bool
		RevisionConstructed bool
	}

	storageObligationStatus uint64
)

func (i storageObligationStatus) String() string {
	if i == 0 {
		return "obligationUnresolved"
	}
	if i == 1 {
		return "obligationRejected"
	}
	if i == 2 {
		return "obligationSucceeded"
	}
	if i == 3 {
		return "obligationFailed"
	}
	return "storageObligationStatus(" + strconv.FormatInt(int64(i), 10) + ")"
}

// getStorageObligation fetches a storage obligation from the database
func getStorageObligation(db ethdb.Database, sc common.Hash) (StorageObligation, error) {
	so, errGet := GetStorageObligation(db, sc)
	if errGet != nil {
		return StorageObligation{}, errGet
	}
	return so, nil
}

// putStorageObligation places a storage obligation into the database,
// overwriting the existing storage obligation if there is one.
func putStorageObligation(db ethdb.Database, so StorageObligation) error {
	err := StoreStorageObligation(db, so.id(), so)
	if err != nil {
		return err
	}
	return nil
}

func deleteStorageObligation(db ethdb.Database, sc common.Hash) error {
	err := DeleteStorageObligation(db, sc)
	if err != nil {
		return err
	}
	return nil
}

// expiration returns the height at which the storage obligation expires.
func (so *StorageObligation) expiration() uint64 {
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewWindowStart
	}
	return so.OriginStorageContract.WindowStart
}

// fileSize returns the size of the data protected by the obligation.
func (so *StorageObligation) fileSize() uint64 {
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewFileSize
	}
	return so.OriginStorageContract.FileSize
}

// id returns the id of the storage obligation, which is defined by the file
// contract id of the storage contract that governs the storage contract.
func (so *StorageObligation) id() (scid common.Hash) {
	return so.StorageContractID
}

// isSane checks that required assumptions about the storage obligation are
func (so *StorageObligation) isSane() error {
	if reflect.DeepEqual(so.OriginStorageContract, EmptyStorageContract) {
		return ErrEmptyOriginStorageContract
	}

	if len(so.StorageContractRevisions) == 0 {
		return ErrEmptyRevisionSet
	}

	return nil
}

// merkleRoot returns the file merkle root of a storage obligation.
func (so *StorageObligation) merkleRoot() common.Hash {
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewFileMerkleRoot
	}
	return so.OriginStorageContract.FileMerkleRoot
}

// payouts returns the set of valid payouts and missed payouts that represent
// the latest revision for the storage obligation.
func (so *StorageObligation) payouts() ([]types.DxcoinCharge, []types.DxcoinCharge) {
	validProofOutputs := make([]types.DxcoinCharge, 2)
	missedProofOutputs := make([]types.DxcoinCharge, 2)
	if len(so.StorageContractRevisions) > 0 {
		copy(validProofOutputs, so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewValidProofOutputs)
		copy(missedProofOutputs, so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewMissedProofOutputs)
		return validProofOutputs, missedProofOutputs
	}
	validProofOutputs = so.OriginStorageContract.ValidProofOutputs
	missedProofOutputs = so.OriginStorageContract.MissedProofOutputs
	return validProofOutputs, missedProofOutputs
}

// proofDeadline returns the height by which the storage proof must be
func (so *StorageObligation) ProofDeadline() uint64 {
	if len(so.StorageContractRevisions) > 0 {
		return so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewWindowEnd
	}
	return so.OriginStorageContract.WindowEnd

}

func (so StorageObligation) value() common.BigInt {
	return so.ContractCost.Add(so.PotentialDownloadRevenue).Add(so.PotentialStorageRevenue).Add(so.PotentialUploadRevenue).Add(so.RiskedCollateral)
}

// deleteStorageObligations deletes obligations from the database.
// It is assumed the deleted obligations don't belong in the database in the first place,
// so no financial metrics are updated.
func (h *StorageHost) deleteStorageObligations(soids []common.Hash) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	for _, soid := range soids {
		err := deleteStorageObligation(h.db, soid)
		if err != nil {
			return err
		}
	}

	return nil
}

// queueActionItem adds an action item to the host at the input height so that
// the host knows to perform maintenance on the associated storage obligation
// when that height is reached.
func (h *StorageHost) queueActionItem(height uint64, id common.Hash) error {

	if height < h.blockHeight {
		h.log.Info("action item queued improperly")
	}

	err := StoreHeight(h.db, id, height)

	if err != nil {
		return err
	}

	return nil
}

// managedAddStorageObligation adds a storage obligation to the host. Because
// this operation can return errors, the transactions should not be submitted to
// the blockchain until after this function has indicated success. All of the
// sectors that are present in the storage obligation should already be on disk,
// which means that addStorageObligation should be exclusively called when
// creating a new, empty file contract or when renewing an existing file
func (h *StorageHost) managedAddStorageObligation(so StorageObligation) error {
	err := func() error {
		h.lock.Lock()
		defer h.lock.Unlock()
		if _, ok := h.lockedStorageObligations[so.id()]; ok {
			h.log.Info("addStorageObligation called with an obligation that is not locked")
		}

		// Sanity check - There needs to be enough time left on the file contract
		// for the host to safely submit the file contract revision.
		if h.blockHeight+revisionSubmissionBuffer >= so.expiration() {
			return errNoBuffer
		}

		// Sanity check - the resubmission timeout needs to be smaller than storage
		// proof window.
		if so.expiration()+resubmissionTimeout >= so.ProofDeadline() {
			return errors.New("fill me in")
		}

		errDB := func() error {

			if len(so.SectorRoots) != 0 {
				err := h.AddSectorBatch(so.SectorRoots)
				if err != nil {
					return err
				}
			}

			errPut := StoreStorageObligation(h.db, so.StorageContractID, so)
			if errPut != nil {
				return errPut
			}
			return nil

		}()

		if errDB != nil {
			return errDB
		}

		// Update the host financial metrics with regards to this storage obligation.
		h.financialMetrics.ContractCount++
		h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Add(so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Add(so.LockedCollateral)
		h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Add(so.RiskedCollateral)
		h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Add(so.TransactionFeesAdded)

		return nil
	}()

	if err != nil {
		return err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	// The file contract was already submitted to the blockchain, need to check
	// after the resubmission timeout that it was submitted successfully.
	err1 := h.queueActionItem(h.blockHeight+resubmissionTimeout, so.id())
	err2 := h.queueActionItem(h.blockHeight+resubmissionTimeout*2, so.id()) // Paranoia
	// Queue an action item to submit the file contract revision - if there is
	// never a file contract revision, the handling of this action item will be
	// a no-op.
	err3 := h.queueActionItem(so.expiration()-revisionSubmissionBuffer, so.id())
	err4 := h.queueActionItem(so.expiration()-revisionSubmissionBuffer+resubmissionTimeout, so.id()) // Paranoia
	// The storage proof should be submitted
	err5 := h.queueActionItem(so.expiration()+resubmissionTimeout, so.id())
	err6 := h.queueActionItem(so.expiration()+resubmissionTimeout*2, so.id()) // Paranoia
	err = common.ErrCompose(err1, err2, err3, err4, err5, err6)
	if err != nil {
		h.log.Info("Error with transaction set, redacting obligation, id", so.id())
		return common.ErrCompose(err, h.removeStorageObligation(so, obligationRejected))
	}

	return nil
}

// modifyStorageObligation will take an updated storage obligation along with a
// list of sector changes and update the database to account for all of it. The
// sector modifications are only used to update the sector database, they will
// not be used to modify the storage obligation (most importantly, this means
// that sectorRoots needs to be updated by the calling function). Virtual
// sectors will be removed the number of times that they are listed, to remove
// multiple instances of the same virtual sector, the virtural sector will need
// to appear in 'sectorsRemoved' multiple times. Same with 'sectorsGained'.
func (h *StorageHost) modifyStorageObligation(so StorageObligation, sectorsRemoved []common.Hash, sectorsGained []common.Hash, gainedSectorData [][]byte) error {
	if _, ok := h.lockedStorageObligations[so.id()]; ok {
		h.log.Info("modifyStorageObligation called with an obligation that is not locked")
	}

	// Sanity check - there needs to be enough time to submit the file contract
	// revision to the blockchain.
	if so.expiration()-revisionSubmissionBuffer <= h.blockHeight {
		return errNoBuffer
	}

	// Sanity check - sectorsGained and gainedSectorData need to have the same length.
	if len(sectorsGained) != len(gainedSectorData) {
		return errInsaneStorageObligationRevision
	}
	// Sanity check - all of the sector data should be modules.SectorSize
	for _, data := range gainedSectorData {
		if uint64(len(data)) != uint64(1<<22) { //Sector Size	4 MiB
			return errInsaneStorageObligationRevision
		}
	}

	//Note, for safe error handling, the operation order should be: add
	// sectors, update database, remove sectors. If the adding or update fails,
	// the added sectors should be removed and the storage obligation shoud be
	// considered invalid. If the removing fails, this is okay, it's ignored
	// and left to consistency checks and user actions to fix (will reduce host
	// capacity, but will not inhibit the host's ability to submit storage
	// proofs)
	var i int
	var err error
	for i = range sectorsGained {
		err = h.AddSector(sectorsGained[i], gainedSectorData[i])
		if err != nil {
			break
		}
	}
	if err != nil {
		// Because there was an error, all of the sectors that got added need
		// to be reverted.
		for j := 0; j < i; j++ {
			// Error is not checked because there's nothing useful that can be
			// done about an error.
			_ = h.RemoveSector(sectorsGained[j])
		}
		return err
	}

	var oldso StorageObligation
	var errOld error
	errDBso := func() error {

		oldso, errOld = getStorageObligation(h.db, so.id())
		if errOld != nil {
			return errOld
		}

		errOld = putStorageObligation(h.db, so)
		if errOld != nil {
			return errOld
		}
		return nil
	}()

	if errDBso != nil {
		// Because there was an error, all of the sectors that got added need
		// to be reverted.
		for i := range sectorsGained {
			// Error is not checked because there's nothing useful that can be
			// done about an error.
			_ = h.RemoveSector(sectorsGained[i])
		}
		return errDBso
	}

	//Call removeSector for all of the sectors that have been removed.
	for k := range sectorsRemoved {
		// Error is not checkeed because there's nothing useful that can be
		// done about an error. Failing to remove a sector is not a terrible
		// place to be, especially if the host can run consistency checks.
		_ = h.RemoveSector(sectorsRemoved[k])
	}

	// Update the financial information for the storage obligation - apply the
	h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Add(so.ContractCost)
	h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Add(so.LockedCollateral)
	h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
	h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
	h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
	h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Add(so.RiskedCollateral)
	h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Add(so.TransactionFeesAdded)

	// Update the financial information for the storage obligation - remove the
	h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(oldso.ContractCost)
	h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(oldso.LockedCollateral)
	h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(oldso.PotentialStorageRevenue)
	h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(oldso.PotentialDownloadRevenue)
	h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(oldso.PotentialUploadRevenue)
	h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(oldso.RiskedCollateral)
	h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Sub(oldso.TransactionFeesAdded)

	return nil

}

// PruneStaleStorageObligations will delete storage obligations from the host
// that, for whatever reason, did not make it on the block chain.
// As these stale storage obligations have an impact on the host financial metrics,
// this method updates the host financial metrics to show the correct values.
func (h *StorageHost) PruneStaleStorageObligations() error {

	sos := h.StorageObligations()
	var scids []common.Hash
	for _, so := range sos {
		if so.OriginConfirmed == false {
			return errTransactionNotConfired
		}
		if h.blockHeight > so.NegotiationHeight+RespendTimeout {
			scids = append(scids, so.StorageContractID)
		}

	}

	// Delete stale obligations from the database.
	err := h.deleteStorageObligations(scids)
	if err != nil {
		return err
	}

	// Update the financial metrics of the host.
	err = h.resetFinancialMetrics()
	if err != nil {
		return err
	}

	return nil
}

// removeStorageObligation will remove a storage obligation from the host,
// either due to failure or success.
func (h *StorageHost) removeStorageObligation(so StorageObligation, sos storageObligationStatus) error {

	// Error is not checked, we want to call remove on every sector even if
	// there are problems - disk health information will be updated.
	_ = h.RemoveSectorBatch(so.SectorRoots)

	// Update the host revenue metrics based on the status of the obligation.
	if sos == obligationUnresolved {
		h.log.Info("storage obligation 'unresolved' during call to removeStorageObligation, id", so.id())
	}
	if sos == obligationRejected {
		if h.financialMetrics.TransactionFeeExpenses.Cmp(so.TransactionFeesAdded) >= 0 {
			// Remove the obligation statistics as potential risk and income.
			h.log.Info("Rejecting storage obligation expiring at block ", so.expiration(), ", current height is ", h.blockHeight, ". Potential revenue is ", h.financialMetrics.PotentialContractCompensation.Add(h.financialMetrics.PotentialStorageRevenue).Add(h.financialMetrics.PotentialDownloadBandwidthRevenue).Add(h.financialMetrics.PotentialUploadBandwidthRevenue))

			h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
			h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedCollateral)
			h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
			h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
			h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
			h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedCollateral)
			h.financialMetrics.TransactionFeeExpenses = h.financialMetrics.TransactionFeeExpenses.Sub(so.TransactionFeesAdded)
		}
	}

	if sos == obligationSucceeded {
		// Empty obligations don't submit a storage proof. The revenue for an empty
		// storage obligation should equal the contract cost of the obligation
		revenue := so.ContractCost.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
		if len(so.SectorRoots) == 0 {
			h.log.Info("No need to submit a storage proof for empty contract. Revenue is ", revenue)
		} else {
			h.log.Info("Successfully submitted a storage proof. Revenue is ", revenue)
		}

		// Remove the obligation statistics as potential risk and income.
		h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedCollateral)
		h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedCollateral)

		// Add the obligation statistics as actual income.
		h.financialMetrics.ContractCompensation = h.financialMetrics.ContractCompensation.Add(so.ContractCost)
		h.financialMetrics.StorageRevenue = h.financialMetrics.StorageRevenue.Add(so.PotentialStorageRevenue)
		h.financialMetrics.DownloadBandwidthRevenue = h.financialMetrics.DownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
		h.financialMetrics.UploadBandwidthRevenue = h.financialMetrics.UploadBandwidthRevenue.Add(so.PotentialUploadRevenue)

	}

	if sos == obligationFailed {
		// Remove the obligation statistics as potential risk and income.
		h.log.Info("Missed storage proof. Revenue would have been", so.ContractCost.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue))

		h.financialMetrics.PotentialContractCompensation = h.financialMetrics.PotentialContractCompensation.Sub(so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Sub(so.LockedCollateral)
		h.financialMetrics.PotentialStorageRevenue = h.financialMetrics.PotentialStorageRevenue.Sub(so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = h.financialMetrics.PotentialDownloadBandwidthRevenue.Sub(so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = h.financialMetrics.PotentialUploadBandwidthRevenue.Sub(so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = h.financialMetrics.RiskedStorageDeposit.Sub(so.RiskedCollateral)

		// Add the obligation statistics as loss.
		h.financialMetrics.LockedStorageDeposit = h.financialMetrics.LockedStorageDeposit.Add(so.RiskedCollateral)
		h.financialMetrics.LostRevenue = h.financialMetrics.LostRevenue.Add(so.ContractCost).Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
	}

	// Update the storage obligation to be finalized but still in-database. The
	// obligation status is updated so that the user can see how the obligation
	// ended up, and the sector roots are removed because they are large
	// objects with little purpose once storage proofs are no longer needed.
	h.financialMetrics.ContractCount--
	so.ObligationStatus = sos
	so.SectorRoots = []common.Hash{}

	errDb := StoreStorageObligation(h.db, so.id(), so)
	if errDb != nil {
		return errDb
	}

	return nil
}

func (h *StorageHost) resetFinancialMetrics() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	fm := HostFinancialMetrics{}
	sos := h.StorageObligations()
	for _, so := range sos {
		// Transaction fees are always added.
		fm.TransactionFeeExpenses = fm.TransactionFeeExpenses.Add(so.TransactionFeesAdded)
		// Update the other financial values based on the obligation status.
		if so.ObligationStatus == obligationUnresolved {
			fm.ContractCount++
			fm.PotentialContractCompensation = fm.PotentialContractCompensation.Add(so.ContractCost)
			fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.LockedCollateral)
			fm.PotentialStorageRevenue = fm.PotentialStorageRevenue.Add(so.PotentialStorageRevenue)
			fm.RiskedStorageDeposit = fm.RiskedStorageDeposit.Add(so.RiskedCollateral)
			fm.PotentialDownloadBandwidthRevenue = fm.PotentialDownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.PotentialUploadBandwidthRevenue = fm.PotentialUploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		}
		if so.ObligationStatus == obligationSucceeded {
			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			fm.StorageRevenue = fm.StorageRevenue.Add(so.PotentialStorageRevenue)
			fm.DownloadBandwidthRevenue = fm.DownloadBandwidthRevenue.Add(so.PotentialDownloadRevenue)
			fm.UploadBandwidthRevenue = fm.UploadBandwidthRevenue.Add(so.PotentialUploadRevenue)
		}
		if so.ObligationStatus == obligationFailed {

			fm.ContractCompensation = fm.ContractCompensation.Add(so.ContractCost)
			if !so.RiskedCollateral.IsNeg() {
				// Storage obligation failed with risked collateral.
				fm.LostRevenue = fm.LostRevenue.Add(so.PotentialStorageRevenue).Add(so.PotentialDownloadRevenue).Add(so.PotentialUploadRevenue)
				fm.LockedStorageDeposit = fm.LockedStorageDeposit.Add(so.RiskedCollateral)
			}

		}

	}

	h.financialMetrics = fm
	return nil
}

// threadedHandleActionItem will look at a storage obligation and determine
// which action is necessary for the storage obligation to succeed.
func (h *StorageHost) threadedHandleActionItem(soid common.Hash) {

	// Lock the storage obligation in question.
	h.managedLockStorageObligation(soid)
	defer func() {
		h.managedUnlockStorageObligation(soid)
	}()

	// Fetch the storage obligation associated with the storage obligation id.
	h.lock.RLock()
	so, errGetso := getStorageObligation(h.db, soid)
	if errGetso != nil {
		h.log.Info("Could not get storage obligation:", errGetso)
		return
	}

	// Check whether the storage obligation has already been completed.
	if so.ObligationStatus != obligationUnresolved {
		// Storage obligation has already been completed, skip action item.
		return
	}

	if !so.OriginConfirmed {
		if h.blockHeight > so.expiration() {
			h.log.Info("Full time has elapsed, but the revision transaction could not be submitted to consensus, id", so.id())
			h.lock.Lock()
			h.removeStorageObligation(so, obligationRejected)
			h.lock.Unlock()
			return
		}

		// Queue another action item to check the status of the transaction.
		h.lock.Lock()
		err := h.queueActionItem(h.blockHeight+resubmissionTimeout, so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Info("Error queuing action item:", err)
		}

		return
	}

	// Check if the file contract revision is ready for submission. Check for death.
	if !so.RevisionConfirmed && len(so.StorageContractRevisions) > 0 && h.blockHeight >= so.expiration()-revisionSubmissionBuffer {
		// Sanity check - there should be a file contract revision.
		rtsLen := len(so.StorageContractRevisions)
		if rtsLen < 1 {
			h.log.Info("transaction revision marked as unconfirmed, yet there is no transaction revision")
			return
		}

		// Check if the revision has failed to submit correctly.
		if h.blockHeight > so.expiration() {
			h.log.Info("Full time has elapsed, but the revision transaction could not be submitted to consensus, id", so.id())
			h.lock.Lock()
			h.removeStorageObligation(so, obligationRejected)
			h.lock.Unlock()
			return
		}

		// Queue another action item to check the status of the transaction.
		h.lock.Lock()
		err := h.queueActionItem(h.blockHeight+resubmissionTimeout, so.id())
		h.lock.Unlock()
		if err != nil {
			h.log.Info("Error queuing action item:", err)
		}

		scrv := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]

		scBytes, err := rlp.EncodeToBytes(scrv)
		if err != nil {
			return
		}

		if _, err := storage.SendContractRevisionTX(h.ethBackend, scrv.NewValidProofOutputs[1].Address, scBytes); err != nil {
			return
		}

	}

	// Check whether a storage proof is ready to be provided, and whether it
	// has been accepted. Check for death.	检查存储证明准备提交和是否被接收，检查状态是否销毁
	if !so.ProofConfirmed && h.blockHeight >= so.expiration()+resubmissionTimeout {
		h.log.Info("Host is attempting a storage proof for", so.id())

		// If the obligation has no sector roots, we can remove the obligation and not
		// submit a storage proof. The host payout for a failed empty contract
		// includes the contract cost and locked collateral.
		if len(so.SectorRoots) == 0 {
			h.log.Debug("storage proof not submitted for empty contract, id", so.id())
			h.lock.Lock()
			err := h.removeStorageObligation(so, obligationSucceeded)
			h.lock.Unlock()
			if err != nil {
				h.log.Info("Error removing storage obligation:", err)
			}
			return
		}
		// If the window has closed, the host has failed and the obligation can
		// be removed.
		if so.ProofDeadline() < h.blockHeight {
			h.log.Info("storage proof not confirmed by deadline, id", so.id())
			h.lock.Lock()
			err := h.removeStorageObligation(so, obligationFailed)
			h.lock.Unlock()
			if err != nil {
				h.log.Info("Error removing storage obligation:", err)
			}
			return
		}

		segmentIdex, errSe := h.storageProofSegment(so.OriginStorageContract)
		if errSe != nil {
			h.log.Debug("Host got an error when fetching a storage proof segment:", errSe)
			return
		}

		sectorIndex := segmentIdex / (SectorSize / SegmentSize)
		sectorRoot := so.SectorRoots[sectorIndex]
		sectorBytes, err := h.ReadSector(sectorRoot)
		if err != nil {
			h.log.Debug("ReadSector error:", err)
			return
		}
		sectorSegment := segmentIdex % (SectorSize / SegmentSize)
		base, cachedHashSet := MerkleProof(sectorBytes, sectorSegment)

		// Using the sector, build a cached root.
		log2SectorSize := uint64(0)
		for 1<<log2SectorSize < (SectorSize / SegmentSize) {
			log2SectorSize++
		}
		ct := merkle.NewCachedTree(log2SectorSize)
		ct.SetIndex(segmentIdex)
		for _, root := range so.SectorRoots {
			ct.Push(root)
		}
		hashSet := ct.Prove(base, cachedHashSet)
		sp := types.StorageProof{
			ParentID: so.id(),
			HashSet:  hashSet,
		}
		copy(sp.Segment[:], base)

		fromAddress := so.OriginStorageContract.ValidProofOutputs[1].Address
		account := accounts.Account{Address: fromAddress}
		wallet, err := h.ethBackend.AccountManager().Find(account)
		spSign, err := wallet.SignHash(account, sp.RLPHash().Bytes())
		sp.Signature = spSign

		scBytes, err := rlp.EncodeToBytes(sp)
		if err != nil {
			return
		}

		if _, err := storage.SendContractRevisionTX(h.ethBackend, fromAddress, scBytes); err != nil {
			return
		}

		h.lock.Lock()
		errGet := h.queueActionItem(so.ProofDeadline(), so.id())
		h.lock.Unlock()
		if errGet != nil {
			h.log.Info("Error queuing action item:", errGet)
		}
	}

	// Save the storage obligation to account for any fee changes.
	errDB := StoreStorageObligation(h.db, soid, so)
	if errDB != nil {
		h.log.Info("Error updating the storage obligations", errDB)
	}

	// Check if all items have succeeded with the required confirmations. Report
	if so.ProofConfirmed && h.blockHeight >= so.ProofDeadline() {
		h.log.Info("file contract complete, id", so.id())
		h.lock.Lock()
		h.removeStorageObligation(so, obligationSucceeded)
		h.lock.Unlock()
	}

}

func MerkleProof(b []byte, proofIndex uint64) (base []byte, hashSet []common.Hash) {
	t := merkle.NewTree()
	t.SetIndex(proofIndex)

	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		t.Push(buf.Next(SegmentSize))
	}

	// Get the proof and convert it to a base + hash set.
	_, proof, _, _ := t.Prove()
	if len(proof) == 0 {
		// There's no proof, because there's no data. Return blank values.
		return nil, nil
	}

	base = proof[0]
	hashSet = make([]common.Hash, len(proof)-1)
	for i, p := range proof[1:] {
		copy(hashSet[i][:], p)
	}

	return base, hashSet
}

// storageProofSegment returns the index of the segment that needs to be proven
// exists in a file contract.
func (h *StorageHost) storageProofSegment(fc types.StorageContract) (uint64, error) {
	fcid := fc.RLPHash()
	triggerHerght := fc.WindowStart - 1

	block, errGetHeight := h.ethBackend.GetBlockByNumber(triggerHerght)
	if errGetHeight != nil {
		return 0, errGetHeight
	}

	triggerID := block.Hash()
	seed := crypto.Keccak256Hash(triggerID[:], fcid[:])
	numSegments := int64(CalculateLeaves(fc.FileSize))
	seedInt := new(big.Int).SetBytes(seed[:])
	index := seedInt.Mod(seedInt, big.NewInt(numSegments)).Uint64()

	return index, nil
}

func CalculateLeaves(dataSize uint64) uint64 {
	numSegments := dataSize / SegmentSize
	if dataSize == 0 || dataSize%SegmentSize != 0 {
		numSegments++
	}
	return numSegments
}

func (h *StorageHost) ProcessConsensusChange(cce core.ChainChangeEvent) {

	h.lock.Lock()
	defer h.lock.Unlock()

	var actionItems []common.Hash

	err := func() error {

		//Handling rolled back blocks
		for _, blockReverted := range cce.RevertedBlockHashes {
			//Rollback contract transaction
			formContractIDs, revisionIDs, storageProofIDs, number, errGetBlock := h.GetAllStrageContractIDsWithBlockHash(blockReverted)
			if errGetBlock != nil {
				continue
			}
			if len(formContractIDs) > 0 {
				for _, id := range formContractIDs {
					so, errGet := getStorageObligation(h.db, id)
					if errGet != nil {
						continue
					}
					so.OriginConfirmed = false
					errPut := putStorageObligation(h.db, so)
					if errPut != nil {
						continue
					}
				}
			}

			if len(revisionIDs) > 0 {
				for _, id := range revisionIDs {
					so, errGet := getStorageObligation(h.db, id)
					if errGet != nil {
						continue
					}
					so.RevisionConfirmed = false
					errPut := putStorageObligation(h.db, so)
					if errPut != nil {
						continue
					}
				}
			}
			if len(storageProofIDs) > 0 {
				for _, id := range storageProofIDs {
					so, errGet := getStorageObligation(h.db, id)
					if errGet != nil {
						continue
					}
					so.ProofConfirmed = false
					errPut := putStorageObligation(h.db, so)
					if errPut != nil {
						continue
					}
				}
			}

			if number != 0 && h.blockHeight > 0 {
				h.blockHeight--
			}

		}

		//Block executing the main chain
		for _, blockApply := range cce.AppliedBlockHashes {
			//apply contract transaction
			formContractIDsApply, revisionIDsApply, storageProofIDsApply, number, errGetBlock := h.GetAllStrageContractIDsWithBlockHash(blockApply)
			if errGetBlock != nil {
				continue
			}

			if len(formContractIDsApply) > 0 {
				for _, id := range formContractIDsApply {
					so, errGet := getStorageObligation(h.db, id)
					if errGet != nil {
						continue
					}
					so.OriginConfirmed = true
					errPut := putStorageObligation(h.db, so)
					if errPut != nil {
						continue
					}
				}
			}

			if len(revisionIDsApply) > 0 {
				for _, id := range revisionIDsApply {
					so, errGet := getStorageObligation(h.db, id)
					if errGet != nil {
						continue
					}
					so.RevisionConfirmed = true
					errPut := putStorageObligation(h.db, so)
					if errPut != nil {
						continue
					}
				}
			}
			if len(storageProofIDsApply) > 0 {
				for _, id := range storageProofIDsApply {
					so, errGet := getStorageObligation(h.db, id)
					if errGet != nil {
						continue
					}
					so.ProofConfirmed = true
					errPut := putStorageObligation(h.db, so)
					if errPut != nil {
						continue
					}
				}
			}

			if number != 0 {
				h.blockHeight++
			}
			existingTtems, err := GetHeight(h.db, h.blockHeight)
			if err != nil {
				continue
			}

			// From the existing items, pull out a storage obligation.
			knownActionItems := make(map[common.Hash]struct{})
			obligationIDs := make([]common.Hash, len(existingTtems)/HashSize)
			for i := 0; i < len(existingTtems); i += HashSize {
				copy(obligationIDs[i/HashSize][:], existingTtems[i:i+HashSize])
			}
			for _, soid := range obligationIDs {
				_, exists := knownActionItems[soid]
				if !exists {
					actionItems = append(actionItems, soid)
					knownActionItems[soid] = struct{}{}
				}
			}

		}

		return nil
	}()

	if err != nil {
		h.log.Info("ProcessConsensusChangeError:", err)
	}

	for i := range actionItems {
		go h.threadedHandleActionItem(actionItems[i])
	}

	err = h.syncConfig()

	if err != nil {
		h.log.Info("ERROR: could not save during ProcessConsensusChange:", err)
	}

}

func (h *StorageHost) GetAllStrageContractIDsWithBlockHash(blockHashs common.Hash) (formContractIDs []common.Hash, revisionIDs []common.Hash, storageProofIDs []common.Hash, number uint64, errGet error) {
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
			errUnRlp := rlp.DecodeBytes(tx.Data(), &sc)
			if errUnRlp != nil {
				continue
			}
			formContractIDs = append(formContractIDs, sc.RLPHash())
		case vm.CommitRevisionTransaction:
			var scr types.StorageContractRevision
			errUnRlp := rlp.DecodeBytes(tx.Data(), &scr)
			if errUnRlp != nil {
				continue
			}
			revisionIDs = append(formContractIDs, scr.RLPHash())
		case vm.StorageProofTransaction:
			var sp types.StorageProof
			errUnRlp := rlp.DecodeBytes(tx.Data(), &sp)
			if errUnRlp != nil {
				continue
			}
			storageProofIDs = append(formContractIDs, sp.RLPHash())
		default:
			continue
		}
	}

	return
}

// StorageObligations fetches the set of storage obligations in the host and
// returns metadata on them.
func (h *StorageHost) StorageObligations() (sos []StorageObligation) {

	if len(h.lockedStorageObligations) < 1 {
		return nil
	}

	for i := range h.lockedStorageObligations {
		so, err := GetStorageObligation(h.db, i)
		if err != nil {
			continue
		}

		sos = append(sos, so)
	}

	return sos
}

func StoreStorageObligation(db ethdb.Database, storageContractID common.Hash, so StorageObligation) error {
	scdb := ethdb.StorageContractDB{db}
	data, err := rlp.EncodeToBytes(so)
	if err != nil {
		return err
	}
	return scdb.StoreWithPrefix(storageContractID, data, PrefixStorageObligation)
}

func DeleteStorageObligation(db ethdb.Database, storageContractID common.Hash) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(storageContractID, PrefixStorageObligation)
}

func GetStorageObligation(db ethdb.Database, storageContractID common.Hash) (StorageObligation, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(storageContractID, PrefixStorageObligation)
	if err != nil {
		return StorageObligation{}, err
	}
	var so StorageObligation
	err = rlp.DecodeBytes(valueBytes, &so)
	if err != nil {
		return StorageObligation{}, err
	}
	return so, nil
}

func StoreHeight(db ethdb.Database, storageContractID common.Hash, height uint64) error {
	scdb := ethdb.StorageContractDB{db}

	existingItems, err := GetHeight(db, height)
	if err != nil {
		existingItems = make([]byte, 1)
	}

	existingItems = append(existingItems, storageContractID[:]...)

	return scdb.StoreWithPrefix(storageContractID, existingItems, PrefixHeight)
}

func DeleteHeight(db ethdb.Database, height uint64) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(height, PrefixHeight)
}

func GetHeight(db ethdb.Database, height uint64) ([]byte, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(height, PrefixHeight)
	if err != nil {
		return nil, err
	}

	return valueBytes, nil
}
