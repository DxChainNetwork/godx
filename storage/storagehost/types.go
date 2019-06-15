package storagehost

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rpc"
)

var (

	// errBadContractOutputCounts is returned if the presented file contract
	// revision has the wrong number of outputs for either the valid or the
	// missed proof outputs.
	errBadContractOutputCounts = ErrorRevision("rejected for having an unexpected number of outputs")

	// errBadContractParent is returned when a file contract revision is
	// presented which has a parent id that doesn't match the file contract
	// which is supposed to be getting revised.
	errBadContractParent = ErrorRevision("could not find contract's parent")

	// errBadFileMerkleRoot is returned if the renter incorrectly updates the
	// file merkle root during a file contract revision.
	errBadFileMerkleRoot = ErrorRevision("rejected for bad file merkle root")

	// errBadFileSize is returned if the renter incorrectly download and
	// changes the file size during a file contract revision.
	errBadFileSize = ErrorRevision("rejected for bad file size")

	// errBadModificationIndex is returned if the renter requests a change on a
	// sector root that is not in the file contract.
	errBadModificationIndex = ErrorRevision("renter has made a modification that points to a nonexistent sector")

	// errBadParentID is returned if the renter incorrectly download and
	// provides the wrong parent id during a file contract revision.
	errBadParentID = ErrorRevision("rejected for bad parent id")

	// errBadPayoutUnlockHashes is returned if the renter incorrectly sets the
	// payout unlock hashes during contract formation.
	errBadPayoutUnlockHashes = ErrorRevision("rejected for bad unlock hashes in the payout")

	// errBadRevisionNumber number is returned if the renter incorrectly
	// download and does not increase the revision number during a file
	// contract revision.
	errBadRevisionNumber = ErrorRevision("rejected for bad revision number")

	// errBadSectorSize is returned if the renter provides a sector to be
	// inserted that is the wrong size.
	errBadSectorSize = ErrorRevision("renter has provided an incorrectly sized sector")

	// errBadUnlockConditions is returned if the renter incorrectly download
	// and does not provide the right unlock conditions in the payment
	// revision.
	errBadUnlockConditions = ErrorRevision("rejected for bad unlock conditions")

	// errBadUnlockHash is returned if the renter incorrectly updates the
	// unlock hash during a file contract revision.
	errBadUnlockHash = ErrorRevision("rejected for bad new unlock hash")

	// errBadWindowEnd is returned if the renter incorrectly download and
	// changes the window end during a file contract revision.
	errBadWindowEnd = ErrorRevision("rejected for bad new window end")

	// errBadWindowStart is returned if the renter incorrectly updates the
	// window start during a file contract revision.
	errBadWindowStart = ErrorRevision("rejected for bad new window start")

	// errEarlyWindow is returned if the file contract provided by the renter
	// has a storage proof window that is starting too near in the future.
	errEarlyWindow = ErrorRevision("rejected for a window that starts too soon")

	// errEmptyObject is returned if the renter sends an empty or nil object
	// unexpectedly.
	errEmptyObject = ErrorRevision("renter has unexpectedly send an empty/nil object")

	// errHighRenterMissedOutput is returned if the renter incorrectly download
	// and deducts an insufficient amount from the renter missed outputs during
	// a file contract revision.
	errHighRenterMissedOutput = ErrorRevision("rejected for high paying renter missed output")

	// errHighRenterValidOutput is returned if the renter incorrectly download
	// and deducts an insufficient amount from the renter valid outputs during
	// a file contract revision.
	errHighRenterValidOutput = ErrorRevision("rejected for high paying renter valid output")

	// errIllegalOffsetAndLength is returned if the renter tries perform a
	// modify operation that uses a troublesome combination of offset and
	// length.
	errIllegalOffsetAndLength = ErrorRevision("renter is trying to do a modify with an illegal offset and length")

	// errLargeSector is returned if the renter sends a RevisionAction that has
	// data which creates a sector that is larger than what the host uses.
	errLargeSector = ErrorRevision("renter has sent a sector that exceeds the host's sector size")

	// errLateRevision is returned if the renter is attempting to revise a
	// revision after the revision deadline. The host needs time to submit the
	// final revision to the blockchain to guarantee payment, and therefore
	// will not accept revisions once the window start is too close.
	errLateRevision = ErrorRevision("renter is requesting revision after the revision deadline")

	// errLongDuration is returned if the renter proposes a file contract with
	// an experation that is too far into the future according to the host's
	// settings.
	errLongDuration = ErrorRevision("renter proposed a file contract with a too-long duration")

	// errLowHostMissedOutput is returned if the renter incorrectly updates the
	// host missed proof output during a file contract revision.
	errLowHostMissedOutput = ErrorRevision("rejected for low paying host missed output")

	// errLowHostValidOutput is returned if the renter incorrectly updates the
	// host valid proof output during a file contract revision.
	errLowHostValidOutput = ErrorRevision("rejected for low paying host valid output")

	// errLowTransactionFees is returned if the renter provides a transaction
	// that the host does not feel is able to make it onto the blockchain.
	errLowTransactionFees = ErrorRevision("rejected for including too few transaction fees")

	// errLowVoidOutput is returned if the renter has not allocated enough
	// funds to the void output.
	errLowVoidOutput = ErrorRevision("rejected for low value void output")

	// errMismatchedHostPayouts is returned if the renter incorrectly sets the
	// host valid and missed payouts to different values during contract
	// formation.
	errMismatchedHostPayouts = ErrorRevision("rejected because host valid and missed payouts are not the same value")

	// errSmallWindow is returned if the renter suggests a storage proof window
	// that is too small.
	errSmallWindow = ErrorRevision("rejected for small window size")

	// errUnknownModification is returned if the host receives a modification
	// action from the renter that it does not understand.
	errUnknownModification = ErrorRevision("renter is attempting an action that the host does not understand")

	// errCollateralBudgetExceeded is returned if the host does not have enough
	// room in the collateral budget to accept a particular file contract.
	errCollateralBudgetExceeded = errors.New("host has reached its collateral budget and cannot accept the file contract")

	// errMaxCollateralReached is returned if a file contract is provided which
	// would require the host to supply more collateral than the host allows
	// per file contract.
	errMaxCollateralReached = errors.New("file contract proposal expects the host to pay more than the maximum allowed collateral")
)

func ExtendErr(s string, err error) error {
	if err == nil {
		return nil
	}

	switch v := err.(type) {
	case ErrorRevision:
		return ErrorRevision(s) + v
	default:
		return fmt.Errorf("%v: %v", s, err)
	}
}

type (

	// Backend is an interface for full node and light node
	Backend interface {
		AccountManager() *accounts.Manager
		APIs() []rpc.API
	}

	// HostFinancialMetrics record the financial element for host
	HostFinancialMetrics struct {
		ContractCount                     uint64        `json:"contractcount"`
		ContractCompensation              common.BigInt `json:"contractcompensation"`
		PotentialContractCompensation     common.BigInt `json:"potentialcontractcompensation"`
		LockedStorageDeposit              common.BigInt `json:"lockedstoragedeposit"`
		LostRevenue                       common.BigInt `json:"lostrevenue"`
		LostStorageDeposit                common.BigInt `json:"loststoragedeposit"`
		PotentialStorageRevenue           common.BigInt `json:"potentialstoragerevenue"`
		RiskedStorageDeposit              common.BigInt `json:"riskedstoragedeposit"`
		StorageRevenue                    common.BigInt `json:"storagerevenue"`
		TransactionFeeExpenses            common.BigInt `json:"transactionfeeexpenses"`
		DownloadBandwidthRevenue          common.BigInt `json:"downloadbandwidthrevenue"`
		PotentialDownloadBandwidthRevenue common.BigInt `json:"potentialdownloadbandwidthrevenue"`
		PotentialUploadBandwidthRevenue   common.BigInt `json:"potentialuploadbandwidthrevenue"`
		UploadBandwidthRevenue            common.BigInt `json:"uploadbandwidthrevenue"`
	}

	ErrorRevision       string
	ErrorCreateContract string
)

func (e ErrorRevision) Error() string {
	return "revision error: " + string(e)
}

func (e ErrorCreateContract) Error() string {
	return "create contract error:" + string(e)
}
