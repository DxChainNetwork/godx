// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

var sectorHeight = func() uint64 {
	height := uint64(0)
	for 1<<height < (storage.SectorSize / merkle.LeafSize) {
		height++
	}
	return height
}()

// verifyRevision checks that the revision pays the host correctly, and that
// the revision does not attempt any malicious or unexpected changes.
func VerifyRevision(so *StorageResponsibility, revision *types.StorageContractRevision, blockHeight uint64, expectedExchange, expectedCollateral common.BigInt) error {
	// Check that the revision is well-formed.
	if len(revision.NewValidProofOutputs) != 2 || len(revision.NewMissedProofOutputs) != 2 {
		return errBadContractOutputCounts
	}

	// Check that the time to finalize and submit the file contract revision
	// has not already passed.
	if so.expiration()-postponedExecutionBuffer <= blockHeight {
		return errLateRevision
	}

	oldFCR := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]

	// Host payout addresses shouldn't change
	if revision.NewValidProofOutputs[1].Address != oldFCR.NewValidProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	if revision.NewMissedProofOutputs[1].Address != oldFCR.NewMissedProofOutputs[1].Address {
		return errors.New("host payout address changed")
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
	fromRenter := common.NewBigInt(oldFCR.NewValidProofOutputs[0].Value.Int64()).Sub(common.NewBigInt(revision.NewValidProofOutputs[0].Value.Int64()))
	// Verify that enough money was transferred.
	if fromRenter.Cmp(expectedExchange) < 0 {
		s := fmt.Sprintf("expected at least %v to be exchanged, but %v was exchanged: ", expectedExchange, fromRenter)
		return ExtendErr(s, errHighRenterValidOutput)
	}

	// Determine the amount of money that was transferred to the host.
	if oldFCR.NewValidProofOutputs[1].Value.Cmp(revision.NewValidProofOutputs[1].Value) > 0 {
		return ExtendErr("host valid proof output was decreased: ", errLowHostValidOutput)
	}
	toHost := common.NewBigInt(revision.NewValidProofOutputs[1].Value.Int64()).Sub(common.NewBigInt(oldFCR.NewValidProofOutputs[1].Value.Int64()))

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
		collateral := common.NewBigInt(oldFCR.NewMissedProofOutputs[1].Value.Int64()).Sub(common.NewBigInt(revision.NewMissedProofOutputs[1].Value.Int64()))
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
	if revision.NewFileMerkleRoot != merkle.CachedTreeRoot(so.SectorRoots, sectorHeight) {
		return errBadFileMerkleRoot
	}

	return nil
}

func VerifyStorageContract(h *StorageHost, sc *types.StorageContract, clientPK *ecdsa.PublicKey, hostPK *ecdsa.PublicKey) error {
	h.lock.RLock()
	blockHeight := h.blockHeight
	lockedStorageDeposit := h.financialMetrics.LockedStorageDeposit
	hostAddress := crypto.PubkeyToAddress(*hostPK)
	config := h.config
	externalConfig := h.externalConfig()
	h.lock.RUnlock()

	// A new file contract should have a file size of zero
	if sc.FileSize != 0 {
		return errBadFileSize
	}

	if sc.FileMerkleRoot != (common.Hash{}) {
		return errBadFileMerkleRoot
	}

	// WindowStart must be at least postponedExecutionBuffer blocks into the future
	if sc.WindowStart <= blockHeight+postponedExecutionBuffer {
		h.log.Debug("A renter tried to form a contract that had a window start which was too soon. The contract started at %v, the current height is %v, the postponedExecutionBuffer is %v, and the comparison was %v <= %v\n", sc.WindowStart, blockHeight, postponedExecutionBuffer, sc.WindowStart, blockHeight+postponedExecutionBuffer)
		return errEarlyWindow
	}

	// WindowEnd must be at least settings.WindowSize blocks after WindowStart
	if sc.WindowEnd < sc.WindowStart+config.WindowSize {
		return errSmallWindow
	}

	// WindowStart must not be more than settings.MaxDuration blocks into the future
	if sc.WindowStart > blockHeight+config.MaxDuration {
		return errLongDuration
	}

	// ValidProofOutputs should have 2 outputs (client + host) and missed
	// outputs should have 2 (client + host)
	if len(sc.ValidProofOutputs) != 2 || len(sc.MissedProofOutputs) != 2 {
		return errBadContractOutputCounts
	}
	// The unlock hashes of the valid and missed proof outputs for the host
	// must match the host's unlock hash
	if sc.ValidProofOutputs[1].Address != hostAddress || sc.MissedProofOutputs[1].Address != hostAddress {
		return errBadPayoutUnlockHashes
	}
	// Check that the payouts for the valid proof outputs and the missed proof
	// outputs are the same - this is important because no data has been added
	// to the file contract yet.
	if sc.ValidProofOutputs[1].Value.Cmp(sc.MissedProofOutputs[1].Value) != 0 {
		return errMismatchedHostPayouts
	}
	// Check that there's enough payout for the host to cover at least the
	// contract price. This will prevent negative currency panics when working
	// with the collateral.
	if sc.ValidProofOutputs[1].Value.Cmp(externalConfig.ContractPrice.BigIntPtr()) < 0 {
		return errLowHostValidOutput
	}
	// Check that the collateral does not exceed the maximum amount of
	// collateral allowed.
	depositMinusContractPrice := common.NewBigInt(sc.ValidProofOutputs[1].Value.Int64()).Sub(externalConfig.ContractPrice)
	if depositMinusContractPrice.Cmp(common.NewBigInt(config.MaxDeposit.Int64())) > 0 {
		return errMaxCollateralReached
	}
	// Check that the host has enough room in the collateral budget to add this
	// collateral.
	if lockedStorageDeposit.Add(depositMinusContractPrice).Cmp(common.NewBigInt(config.DepositBudget.Int64())) > 0 {
		return errCollateralBudgetExceeded
	}

	// The unlock hash for the file contract must match the unlock hash that
	// the host knows how to spend.
	expectedUH := types.UnlockConditions{
		PaymentAddresses: []common.Address{
			crypto.PubkeyToAddress(*clientPK),
			crypto.PubkeyToAddress(*hostPK),
		},
		SignaturesRequired: 2,
	}.UnlockHash()
	if sc.UnlockHash != expectedUH {
		return errBadUnlockHash
	}

	return nil
}

func FinalizeStorageResponsibility(h *StorageHost, so StorageResponsibility) error {
	// Get a lock on the storage responsibility
	lockErr := h.checkAndTryLockStorageResponsibility(so.id(), storage.ResponsibilityLockTimeout)
	if lockErr != nil {
		return lockErr
	}
	defer h.checkAndUnlockStorageResponsibility(so.id())

	if err := h.InsertStorageResponsibility(so); err != nil {
		return err
	}
	return nil
}

// verifyPaymentRevision verifies that the revision being provided to pay for
// the data has transferred the expected amount of money from the renter to the
// host.
func VerifyPaymentRevision(existingRevision, paymentRevision types.StorageContractRevision, blockHeight uint64, expectedTransfer *big.Int) error {
	// Check that the revision is well-formed.
	if len(paymentRevision.NewValidProofOutputs) != 2 || len(paymentRevision.NewMissedProofOutputs) != 3 {
		return errBadContractOutputCounts
	}

	// Check that the time to finalize and submit the file contract revision
	// has not already passed.
	if existingRevision.NewWindowStart-postponedExecutionBuffer <= blockHeight {
		return errLateRevision
	}

	// Host payout addresses shouldn't change
	if paymentRevision.NewValidProofOutputs[1].Address != existingRevision.NewValidProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	if paymentRevision.NewMissedProofOutputs[1].Address != existingRevision.NewMissedProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	// Make sure the lost collateral still goes to the void
	if paymentRevision.NewMissedProofOutputs[2].Address != existingRevision.NewMissedProofOutputs[2].Address {
		return errors.New("lost collateral address was changed")
	}

	// Determine the amount that was transferred from the renter.
	if paymentRevision.NewValidProofOutputs[0].Value.Cmp(existingRevision.NewValidProofOutputs[0].Value) > 0 {
		return ExtendErr("renter increased its valid proof output: ", errHighRenterValidOutput)
	}
	fromRenter := existingRevision.NewValidProofOutputs[0].Value.Sub(existingRevision.NewValidProofOutputs[0].Value, paymentRevision.NewValidProofOutputs[0].Value)
	// Verify that enough money was transferred.
	if fromRenter.Cmp(expectedTransfer) < 0 {
		s := fmt.Sprintf("expected at least %v to be exchanged, but %v was exchanged: ", expectedTransfer, fromRenter)
		return ExtendErr(s, errHighRenterValidOutput)
	}

	// Determine the amount of money that was transferred to the host.
	if existingRevision.NewValidProofOutputs[1].Value.Cmp(paymentRevision.NewValidProofOutputs[1].Value) > 0 {
		return ExtendErr("host valid proof output was decreased: ", errLowHostValidOutput)
	}
	toHost := paymentRevision.NewValidProofOutputs[1].Value.Sub(paymentRevision.NewValidProofOutputs[1].Value, existingRevision.NewValidProofOutputs[1].Value)
	// Verify that enough money was transferred.
	if toHost.Cmp(fromRenter) != 0 {
		s := fmt.Sprintf("expected exactly %v to be transferred to the host, but %v was transferred: ", fromRenter, toHost)
		return ExtendErr(s, errLowHostValidOutput)
	}

	// If the renter's valid proof output is larger than the renter's missed
	// proof output, the renter has incentive to see the host fail. Make sure
	// that this incentive is not present.
	if paymentRevision.NewValidProofOutputs[0].Value.Cmp(paymentRevision.NewMissedProofOutputs[0].Value) > 0 {
		return ExtendErr("renter has incentive to see host fail: ", errHighRenterMissedOutput)
	}

	// Check that the host is not going to be posting collateral.
	if paymentRevision.NewMissedProofOutputs[1].Value.Cmp(existingRevision.NewMissedProofOutputs[1].Value) < 0 {
		collateral := existingRevision.NewMissedProofOutputs[1].Value.Sub(existingRevision.NewMissedProofOutputs[1].Value, paymentRevision.NewMissedProofOutputs[1].Value)
		s := fmt.Sprintf("host not expecting to post any collateral, but contract has host posting %v collateral: ", collateral)
		return ExtendErr(s, errLowHostMissedOutput)
	}

	// Check that the revision count has increased.
	if paymentRevision.NewRevisionNumber <= existingRevision.NewRevisionNumber {
		return errBadRevisionNumber
	}

	// Check that all of the non-volatile fields are the same.
	if paymentRevision.ParentID != existingRevision.ParentID {
		return errBadParentID
	}
	if paymentRevision.UnlockConditions.UnlockHash() != existingRevision.UnlockConditions.UnlockHash() {
		return errBadUnlockConditions
	}
	if paymentRevision.NewFileSize != existingRevision.NewFileSize {
		return errBadFileSize
	}
	if paymentRevision.NewFileMerkleRoot != existingRevision.NewFileMerkleRoot {
		return errBadFileMerkleRoot
	}
	if paymentRevision.NewWindowStart != existingRevision.NewWindowStart {
		return errBadWindowStart
	}
	if paymentRevision.NewWindowEnd != existingRevision.NewWindowEnd {
		return errBadWindowEnd
	}
	if paymentRevision.NewUnlockHash != existingRevision.NewUnlockHash {
		return errBadUnlockHash
	}
	if paymentRevision.NewMissedProofOutputs[1].Value.Cmp(existingRevision.NewMissedProofOutputs[1].Value) != 0 {
		return errLowHostMissedOutput
	}
	return nil
}
