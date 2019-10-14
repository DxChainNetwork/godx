// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import "fmt"

type uploadNegotiationError string

func (une uploadNegotiationError) Error() string {
	return fmt.Sprintf("upload negotiation failed, host rejected client's upload request: %s", une)
}

var (
	errParseAction               = uploadNegotiationError("failed to parse the upload action, unknown upload action type")
	errReachDeadline             = uploadNegotiationError("storage client is requesting the storage revision after the revision deadline has reached")
	errBadValidPaybackCounts     = uploadNegotiationError("the expected valid proof payback counts is incorrect")
	errBadMissedPaybackCounts    = uploadNegotiationError("the expected missed proof payback counts is incorrect")
	errBadValidHostAddress       = uploadNegotiationError("host address contained in the valid proof payback is incorrect")
	errBadMissedHostAddress      = uploadNegotiationError("host address contained in the missed proof payback is incorrect")
	errBadParentID               = uploadNegotiationError("old revision's parentID does not match with new revision's parent ID")
	errBadUnlockCondition        = uploadNegotiationError("old revision's unlock condition does not match with the new revision's unlock condition")
	errSmallRevisionNumber       = uploadNegotiationError("old revision's revision number is bigger than the new revision's revision number")
	errBadWindowStart            = uploadNegotiationError("old revision's window start does not match with new revision's window start")
	errBadWindowEnd              = uploadNegotiationError("old revision's window end does not match with new revision's window end")
	errBadUnlockHash             = uploadNegotiationError("old revision's unlock hash does not match with new revision's unlock hash")
	errHighClientMissedPayback   = uploadNegotiationError("high client missed payback")
	errSmallerClientValidPayback = uploadNegotiationError("new revision client's validProofPayback should be smaller than old revision validProofPayback")
	errSmallHostValidPayback     = uploadNegotiationError("host's new revision validProofPayback should be greater than old revision validProofPayback")
	errClientPayment             = uploadNegotiationError("the expected storage client payment is not equivalent to the payment made by the storage client")
	errHostPaymentReceived       = uploadNegotiationError("payment from the storage client does not equivalent to payment received by the storage host")
	errBadMerkleRoot             = uploadNegotiationError("new storage revision contains bad merkle root")
	errBadFileSize               = uploadNegotiationError("new storage revision contains bad file size")
)
