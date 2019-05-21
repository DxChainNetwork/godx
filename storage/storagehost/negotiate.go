/*
 * // Copyright 2019 DxChain, All rights reserved.
 * // Use of this source code is governed by an Apache
 * // License 2.0 that can be found in the LICENSE file.
 */

package storagehost

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
	"math/big"
)

// verifyRevision checks that the revision pays the host correctly, and that
// the revision does not attempt any malicious or unexpected changes.
func VerifyRevision(so *StorageObligation, revision *types.StorageContractRevision, blockHeight uint64, expectedExchange, expectedCollateral *big.Int) error {
	// Check that the revision is well-formed.
	if len(revision.NewValidProofOutputs) != 2 || len(revision.NewMissedProofOutputs) != 3 {
		return errBadContractOutputCounts
	}

	// Check that the time to finalize and submit the file contract revision
	// has not already passed.
	if so.expiration()-revisionSubmissionBuffer <= blockHeight {
		return errLateRevision
	}

	oldFCR := so.Revision[len(so.Revision)-1]

	// Host payout addresses shouldn't change
	if revision.NewValidProofOutputs[1].Address != oldFCR.NewValidProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	if revision.NewMissedProofOutputs[1].Address != oldFCR.NewMissedProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	// Make sure the lost collateral still goes to the void
	if revision.NewMissedProofOutputs[2].Address != oldFCR.NewMissedProofOutputs[2].Address {
		return errors.New("lost collateral address was changed")
	}

	// Check that all non-volatile fields are the same.
	if oldFCR.ParentID != revision.ParentID {
		return errBadContractParent
	}
	if oldFCR.UnlockConditions.UnlockHash() != revision.UnlockConditions.UnlockHash() {
		return errBadUnlockConditions
	}
	if oldFCR.NewRevisionNumber >= revision.NewRevisionNumber {
		return errBadRevisionNumber
	}
	if revision.NewFileSize != uint64(len(so.SectorRoots))*storage.SectorSize {
		return errBadFileSize
	}
	if oldFCR.NewWindowStart != revision.NewWindowStart {
		return errBadWindowStart
	}
	if oldFCR.NewWindowEnd != revision.NewWindowEnd {
		return errBadWindowEnd
	}
	if oldFCR.NewUnlockHash != revision.NewUnlockHash {
		return errBadUnlockHash
	}

	// Determine the amount that was transferred from the client.
	if revision.NewValidProofOutputs[0].Value.Cmp(oldFCR.NewValidProofOutputs[0].Value) > 0 {
		return fmt.Errorf("client increased its valid proof output: %v", errHighRenterValidOutput)
	}
	fromRenter := new(big.Int).Sub(oldFCR.NewValidProofOutputs[0].Value, revision.NewValidProofOutputs[0].Value)
	// Verify that enough money was transferred.
	if fromRenter.Cmp(expectedExchange) < 0 {
		s := fmt.Sprintf("expected at least %v to be exchanged, but %v was exchanged: ", expectedExchange, fromRenter)
		return ExtendErr(s, errHighRenterValidOutput)
	}

	// Determine the amount of money that was transferred to the host.
	if oldFCR.NewValidProofOutputs[1].Value.Cmp(revision.NewValidProofOutputs[1].Value) > 0 {
		return ExtendErr("host valid proof output was decreased: ", errLowHostValidOutput)
	}
	toHost := new(big.Int).Sub(revision.NewValidProofOutputs[1].Value, oldFCR.NewValidProofOutputs[1].Value)
	// Verify that enough money was transferred.
	if toHost.Cmp(fromRenter) != 0 {
		s := fmt.Sprintf("expected exactly %v to be transferred to the host, but %v was transferred: ", fromRenter, toHost)
		return ExtendErr(s, errLowHostValidOutput)
	}

	// If the renter's valid proof output is larger than the renter's missed
	// proof output, the renter has incentive to see the host fail. Make sure
	// that this incentive is not present.
	if revision.NewValidProofOutputs[0].Value.Cmp(revision.NewMissedProofOutputs[0].Value) > 0 {
		return ExtendErr("client has incentive to see host fail: ", errHighRenterMissedOutput)
	}

	// Check that the host is not going to be posting more collateral than is
	// expected. If the new misesd output is greater than the old one, the host
	// is actually posting negative collateral, which is fine.
	if revision.NewMissedProofOutputs[1].Value.Cmp(oldFCR.NewMissedProofOutputs[1].Value) <= 0 {
		collateral := new(big.Int).Sub(oldFCR.NewMissedProofOutputs[1].Value, revision.NewMissedProofOutputs[1].Value)
		if collateral.Cmp(expectedCollateral) > 0 {
			s := fmt.Sprintf("host expected to post at most %v collateral, but contract has host posting %v: ", expectedCollateral, collateral)
			return ExtendErr(s, errLowHostMissedOutput)
		}
	}

	// Check that the revision count has increased.
	if revision.NewRevisionNumber <= oldFCR.NewRevisionNumber {
		return errBadRevisionNumber
	}

	// The Merkle root is checked last because it is the most expensive check.
	if revision.NewFileMerkleRoot != storage.CachedMerkleRoot(so.SectorRoots) {
		return errBadFileMerkleRoot
	}

	return nil
}