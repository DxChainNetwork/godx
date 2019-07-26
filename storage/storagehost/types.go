package storagehost

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
)

var (

	// errBadContractOutputCounts is returned if the presented file contract
	// revision has the wrong number of outputs for either the valid or the
	// missed proof outputs.
	errBadContractOutputCounts = ErrorRevision("responsibilityRejected for having an unexpected number of outputs")

	// errBadContractParent is returned when a file contract revision is
	// presented which has a parent id that doesn't match the file contract
	// which is supposed to be getting revised.
	errBadContractParent = ErrorRevision("could not find contract's parent")

	// errBadFileMerkleRoot is returned if the client incorrectly updates the
	// file merkle root during a file contract revision.
	errBadFileMerkleRoot = ErrorRevision("responsibilityRejected for bad file merkle root")

	// errBadFileSize is returned if the client incorrectly download and
	// changes the file size during a file contract revision.
	errBadFileSize = ErrorRevision("responsibilityRejected for bad file size")

	// errBadParentID is returned if the client incorrectly download and
	// provides the wrong parent id during a file contract revision.
	errBadParentID = ErrorRevision("responsibilityRejected for bad parent id")

	// errBadPayoutUnlockHashes is returned if the client incorrectly sets the
	// payout unlock hashes during contract formation.
	errBadPayoutUnlockHashes = ErrorRevision("responsibilityRejected for bad unlock hashes in the payout")

	// errBadRevisionNumber number is returned if the client incorrectly
	// download and does not increase the revision number during a file
	// contract revision.
	errBadRevisionNumber = ErrorRevision("responsibilityRejected for bad revision number")

	// errBadUnlockConditions is returned if the client incorrectly download
	// and does not provide the right unlock conditions in the payment
	// revision.
	errBadUnlockConditions = ErrorRevision("responsibilityRejected for bad unlock conditions")

	// errBadUnlockHash is returned if the client incorrectly updates the
	// unlock hash during a file contract revision.
	errBadUnlockHash = ErrorRevision("responsibilityRejected for bad new unlock hash")

	// errBadWindowEnd is returned if the client incorrectly download and
	// changes the window end during a file contract revision.
	errBadWindowEnd = ErrorRevision("responsibilityRejected for bad new window end")

	// errBadWindowStart is returned if the client incorrectly updates the
	// window start during a file contract revision.
	errBadWindowStart = ErrorRevision("responsibilityRejected for bad new window start")

	// errEarlyWindow is returned if the file contract provided by the client
	// has a storage proof window that is starting too near in the future.
	errEarlyWindow = ErrorRevision("responsibilityRejected for a window that starts too soon")

	// errHighClientMissedOutput is returned if the client incorrectly download
	// and deducts an insufficient amount from the client missed outputs during
	// a file contract revision.
	errHighClientMissedOutput = ErrorRevision("responsibilityRejected for high paying client missed output")

	// errHighClientValidOutput is returned if the client incorrectly download
	// and deducts an insufficient amount from the client valid outputs during
	// a file contract revision.
	errHighClientValidOutput = ErrorRevision("responsibilityRejected for high paying client valid output")

	// errLateRevision is returned if the client is attempting to revise a
	// revision after the revision deadline. The host needs time to submit the
	// final revision to the blockchain to guarantee payment, and therefore
	// will not accept revisions once the window start is too close.
	errLateRevision = ErrorRevision("client is requesting revision after the revision deadline")

	// errLongDuration is returned if the client proposes a file contract with
	// an expiration that is too far into the future according to the host's
	// settings.
	errLongDuration = ErrorRevision("client proposed a file contract with a too-long duration")

	// errLowHostMissedOutput is returned if the client incorrectly updates the
	// host missed proof output during a file contract revision.
	errLowHostMissedOutput = ErrorRevision("responsibilityRejected for low paying host missed output")

	// errLowHostValidOutput is returned if the client incorrectly updates the
	// host valid proof output during a file contract revision.
	errLowHostValidOutput = ErrorRevision("responsibilityRejected for low paying host valid output")

	// errMismatchedHostPayouts is returned if the client incorrectly sets the
	// host valid and missed payouts to different values during contract
	// formation.
	errMismatchedHostPayouts = ErrorRevision("responsibilityRejected because host valid and missed payouts are not the same value")

	// errSmallWindow is returned if the client suggests a storage proof window
	// that is too small.
	errSmallWindow = ErrorRevision("responsibilityRejected for small window size")

	// errCollateralBudgetExceeded is returned if the host does not have enough
	// room in the collateral budget to accept a particular file contract.
	errCollateralBudgetExceeded = errors.New("host has reached its collateral budget and cannot accept the file contract")

	// errMaxCollateralReached is returned if a file contract is provided which
	// would require the host to supply more collateral than the host allows
	// per file contract.
	errMaxCollateralReached = errors.New("file contract proposal expects the host to pay more than the maximum allowed collateral")

	errEmptyOriginStorageContract = errors.New("storage contract has no storage responsibility")
	errEmptyRevisionSet           = errors.New("take the last revision ")
	errInsaneRevision             = errors.New("revision is not necessary")
	errNotAllowed                 = errors.New("time is not allowed")
	errTransactionNotConfirmed    = errors.New("transaction not confirmed")
)

// ExtendErr wraps a error with a string
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

	// ErrorRevision is some error that occurs in revision
	ErrorRevision string

	// ErrorCreateContract is some error that occurs in contract creation
	ErrorCreateContract string
)

func (e ErrorRevision) Error() string {
	return "revision error: " + string(e)
}

func (e ErrorCreateContract) Error() string {
	return "create contract error:" + string(e)
}
