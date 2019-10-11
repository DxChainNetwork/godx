// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiation

import "fmt"

type downloadNegotiationError string

func (dne downloadNegotiationError) Error() string {
	return fmt.Sprintf("download negotiation failed, host rejected client's download request: %s", dne)
}

var (
	errBadParentID             = downloadNegotiationError("errBadParentID-the new revision's parentID does not match with the old ones")
	errBadUnlockConditions     = downloadNegotiationError("errBadUnlockConditions-the new revision's unlockCondition does not match with the old ones")
	errBadFileSize             = downloadNegotiationError("errBadFileSize-the new revision's file size does not match with the old ones")
	errBadFileMerkleRoot       = downloadNegotiationError("errBadFileMerkleRoot-the new revision's file merkle root does not match with the old ones")
	errBadWindowStart          = downloadNegotiationError("errBadWindowStart-the new revision's window start does not match with the old ones")
	errBadWindowEnd            = downloadNegotiationError("errBadWindowEnd-the new revision's window end does not match with the old ones")
	errBadUnlockHash           = downloadNegotiationError("errBadUnlockHash-the new revision's unlockHash does not match with the old ones")
	errBadPaybackCount         = downloadNegotiationError("errBadPaybackCount-unexpected number of paybacks in the new revision")
	errValidPaybackAddress     = downloadNegotiationError("errPaybackAddress-storage host's valid payback addresses changed during the downloading process")
	errMissedPaybackAddress    = downloadNegotiationError("errPaybackAddress-storage host's missed payback addresses changed during the downloading process")
	errValidPaybackValue       = downloadNegotiationError("errValidPaybackValue-storage host's valid proof payback changed during the download process")
	errMissedPaybackValue      = downloadNegotiationError("errMissedPaybackValue-storage host's missed proof payback changed during the download process")
	errClientHighMissedPayback = downloadNegotiationError("errClientHighMissedPayback-storage client has incentive to let the storage host fail by putting a larger missed payback")
	errHostLowValidPayback     = downloadNegotiationError("errHostLowValidPayback-storage host valid proof payback decreased during downloading")
	errLowDownloadPay          = downloadNegotiationError("errLowDownloadPay-storage client does not transfer enough tokens to cover the download cost")
	errHostTokenReceive        = downloadNegotiationError("errHostTokenReceive-the amount of token received by the storage host is not equal to amount of token transferred by the storage client")
	errLateRevision            = downloadNegotiationError("errLateRevision-storage client is requesting the revision after the revision deadline")
	errBadRevNumber            = downloadNegotiationError("errBadRevNumber-the new revision number is smaller or equal to old revision number, which is prohibited")
	errFindWallet              = downloadNegotiationError("errFailedToFindWallet-failed to retrieve the host's wallet information using the address provided in the contract revision")
	errHostRevSign             = downloadNegotiationError("errHostRevSign-storage host failed to sign the storage contract revision")
	errFetchData               = downloadNegotiationError("errFetchData-storage host failed to get the data sector requested by the storage client locally")
	errFetchMerkleProof        = downloadNegotiationError("errFetchMerkleProof-storage host failed to construct merkle proof")
	errCommitFailed            = downloadNegotiationError("errCommitFailed-storage host failed to commit the modified storage responsibility")
	errSectorSize              = downloadNegotiationError("errDataSize-the download requested data sector is greater than the required data sector size")
	errSectorLength            = downloadNegotiationError("errSectorLength-the requested download data sector's length cannot be 0")
	errSectorMerkle            = downloadNegotiationError("errSectorMerkle-the requested download data sector's length and offset must be multiples of the segment size when requesting a merkle proof")
	errBadReqPaybackCount      = downloadNegotiationError("errBadReqPaybackCount-the proof payback contained in the request does not match with number of paybacks in the old contract revision")
)
