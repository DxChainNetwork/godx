// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiation

import "fmt"

type negotiationError string

func (ne negotiationError) Error() string {
	return fmt.Sprintf("contract create negotiation failed, host rejected client's request: %s", ne)
}

var (
	errNoNewContract              = negotiationError("storage host does not accept new storage contract")
	errHostInsufficientBalance    = negotiationError("storage host has insufficient balance")
	errEarlyWindow                = negotiationError("storage contract window start too soon")
	errSmallWindowSize            = negotiationError("storage contract has small window size")
	errLongDuration               = negotiationError("client proposed a storage contract with long duration")
	errBadUnlockHash              = negotiationError("storage contract has different unlock hash")
	errBadValidProofPaybackCount  = negotiationError("unexpected amount of validProofPayback")
	errBadMissedProofPaybackCount = negotiationError("unexpected amount of missedProofPayback")
	errBadValidProofAddress       = negotiationError("host address contained in the validProofPayback is incorrect")
	errBadMissedProofAddress      = negotiationError("host address contained in the missedProofPayback is incorrect")
	errBadMaxDeposit              = negotiationError("client expected host to pay more deposit than the max allowed deposit")
	errLowHostExpectedValidPayout = negotiationError("host rejected the contract for low expected paying host valid payout")
	errExpectedDepositReachBudget = negotiationError("host expected deposit has reached to its deposit budget and cannot accept any storage contract")
	errLowMissedPayout            = negotiationError("host rejected the contract for low paying missed payout")
	errBadRenewFileSize           = negotiationError("file size from the storage contract does not match with the one stored in the storage responsibility")
	errBadRenewFileRoot           = negotiationError("file merkle root from the storage contract does not match with the one stored in the storage responsibility")
	errExceedDeposit              = negotiationError("deposit requested by the storage client exceed the host's max deposit configuration")
	errReachBudget                = negotiationError("host has reached its deposit budget and cannot accept any new contract")
	errPayoutMismatch             = negotiationError("host valid payout and missed payout not equal to each other")
	errLowHostValidPayout         = negotiationError("host validPayout is too low")
	errBadFileSizeCreate          = negotiationError("the file size should be 0 when creating the storage contract")
	errBadFileRootCreate          = negotiationError("the file merkle root should be 0 when creating the storage contract")
)
