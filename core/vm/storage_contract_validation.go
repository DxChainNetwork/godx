// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"crypto/ecdsa"
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
)

var (
	errZeroCollateral                          = errors.New("the payout of form contract is less 0")
	errZeroOutput                              = errors.New("the output of form contract is less 0")
	errStorageContractValidOutputSumViolation  = errors.New("file contract has invalid valid proof output sums")
	errStorageContractMissedOutputSumViolation = errors.New("file contract has invalid missed proof output sums")
	errStorageContractOutputSumViolation       = errors.New("file contract has ")

	errStorageContractWindowEndViolation   = errors.New("file contract window must end at least one block after it starts")
	errStorageContractWindowStartViolation = errors.New("file contract window must start in the future")

	errTimelockNotSatisfied  = errors.New("timelock has not been met")
	errLateRevision          = errors.New("file contract revision submitted after deadline")
	errLowRevisionNumber     = errors.New("transaction has a file contract with an outdated revision number")
	errWrongUnlockConditions = errors.New("transaction contains incorrect unlock conditions")
	errRevisionValidPayouts  = errors.New("file contract revision has altered valid payout")
	errRevisionMissedPayouts = errors.New("file contract revision has altered missed payout")
	errWrongUnlockCondition  = errors.New("the unlockhash of file contract not match unlockcondition")
	errInvalidRenterSig      = errors.New("invalid renter signatures")
	errInvalidHostSig        = errors.New("invalid host signatures")
	errNoStorageContractType = errors.New("no this file contract type")

	errInvalidStorageProof = errors.New("invalid storage proof")
)

const (
	SegmentSize = 64
)

func CheckFormContract(evm *EVM, fc types.StorageContract, currentHeight types.BlockHeight) error {

	// check if this file contract exist
	fcID := fc.ID()
	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	_, err := GetStorageContract(db, fcID)
	if err == nil {
		return errors.New("this file contract exist")
	}

	if fc.RenterCollateral.Value.Sign() <= 0 {
		return errZeroCollateral
	}
	if fc.HostCollateral.Value.Sign() <= 0 {
		return errZeroCollateral
	}

	// Check that start and expiration are reasonable values.
	if fc.WindowStart <= currentHeight {
		return errStorageContractWindowStartViolation
	}
	if fc.WindowEnd <= fc.WindowStart {
		return errStorageContractWindowEndViolation
	}

	// Check that the proof outputs sum to the payout
	validProofOutputSum := new(big.Int).SetInt64(0)
	missedProofOutputSum := new(big.Int).SetInt64(0)
	for _, output := range fc.ValidProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		validProofOutputSum = validProofOutputSum.Add(validProofOutputSum, output.Value)
	}
	for _, output := range fc.MissedProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		missedProofOutputSum = missedProofOutputSum.Add(missedProofOutputSum, output.Value)
	}

	payout := fc.RenterCollateral.Value.Add(fc.RenterCollateral.Value, fc.HostCollateral.Value)
	if validProofOutputSum.Cmp(payout) != 0 {
		return errStorageContractValidOutputSumViolation
	}
	if missedProofOutputSum.Cmp(payout) != 0 {
		return errStorageContractMissedOutputSumViolation
	}

	// check if balance is enough for collateral
	renterAddr := fc.RenterCollateral.Address
	renterCollateralAmount := fc.RenterCollateral.Value
	hostAddr := fc.HostCollateral.Address
	hostCollateralAmount := fc.HostCollateral.Value

	renterBalance := evm.StateDB.GetBalance(renterAddr)
	if renterBalance.Cmp(renterCollateralAmount) == -1 {
		return errors.New("renter has not enough balance for file contract collateral")
	}

	hostBalance := evm.StateDB.GetBalance(hostAddr)
	if hostBalance.Cmp(hostCollateralAmount) == -1 {
		return errors.New("host has not enough balance for file contract collateral")
	}

	err = CheckMultiSignatures(fc, currentHeight, fc.Signatures)
	if err != nil {
		log.Error("failed to check signature for form contract", "err", err)
		return err
	}

	return nil
}

func CheckReversionContract(evm *EVM, fcr types.StorageContractRevision, currentHeight types.BlockHeight) error {

	if fcr.UnlockConditions.Timelock > currentHeight {
		return errTimelockNotSatisfied
	}

	// Check that start and expiration are reasonable values.
	if fcr.NewWindowStart <= currentHeight {
		return errStorageContractWindowStartViolation
	}
	if fcr.NewWindowEnd <= fcr.NewWindowStart {
		return errStorageContractWindowEndViolation
	}

	// Check that the valid outputs and missed outputs sum to the same
	// value.
	validProofOutputSum := new(big.Int).SetInt64(0)
	missedProofOutputSum := new(big.Int).SetInt64(0)
	for _, output := range fcr.NewValidProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		validProofOutputSum = validProofOutputSum.Add(validProofOutputSum, output.Value)
	}
	for _, output := range fcr.NewMissedProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		missedProofOutputSum = missedProofOutputSum.Add(missedProofOutputSum, output.Value)
	}
	if validProofOutputSum.Cmp(missedProofOutputSum) != 0 {
		return errStorageContractOutputSumViolation
	}

	if err := CheckMultiSignatures(fcr, 0, fcr.Signatures); err != nil {
		return err
	}

	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	fc, err := GetStorageContract(db, fcr.ParentID)
	if err != nil {
		return err
	}

	// Check that the height is less than fc.WindowStart - revisions are
	// not allowed to be submitted once the storage proof window has
	// opened.  This reduces complexity for unconfirmed transactions.
	if currentHeight > fc.WindowStart {
		return errLateRevision
	}

	// Check that the revision number of the revision is greater than the
	// revision number of the existing file contract.
	if fc.RevisionNumber >= fcr.NewRevisionNumber {
		return errLowRevisionNumber
	}

	// Check that the unlock conditions match the unlock hash.
	if fcr.UnlockConditions.UnlockHash() != common.Hash(fc.UnlockHash) {
		return errWrongUnlockConditions
	}

	// Check that the payout of the revision matches the payout of the
	// original, and that the payouts match each other.
	validPayout := new(big.Int).SetInt64(0)
	missedPayout := new(big.Int).SetInt64(0)
	oldPayout := new(big.Int).SetInt64(0)
	for _, output := range fcr.NewValidProofOutputs {
		validPayout = validPayout.Add(validPayout, output.Value)
	}
	for _, output := range fcr.NewMissedProofOutputs {
		missedPayout = missedPayout.Add(missedPayout, output.Value)
	}
	for _, output := range fc.ValidProofOutputs {
		oldPayout = oldPayout.Add(oldPayout, output.Value)
	}
	if validPayout.Cmp(oldPayout) != 0 {
		return errRevisionValidPayouts
	}
	if missedPayout.Cmp(oldPayout) != 0 {
		return errRevisionMissedPayouts
	}

	return nil
}

func CheckMultiSignatures(originalData interface{}, currentHeight types.BlockHeight, signatures []types.Signature) error {
	if len(signatures) == 0 {
		return errors.New("no signatures")
	}

	var (
		singleSig, renterSig, hostSig          types.Signature
		singlePubkey, renterPubkey, hostPubkey ecdsa.PublicKey
		err                                    error
		uc                                     types.UnlockConditions
	)

	if len(signatures) == 1 {
		singleSig = signatures[0]
		singlePubkey, err = RecoverPubkeyFromSignature(singleSig)
		if err != nil {
			return err
		}
	}

	if len(signatures) == 2 {
		renterSig = signatures[0]
		hostSig = signatures[1]
		renterPubkey, err = RecoverPubkeyFromSignature(renterSig)
		if err != nil {
			return err
		}
		hostPubkey, err = RecoverPubkeyFromSignature(hostSig)
		if err != nil {
			return err
		}

		uc = types.UnlockConditions{
			Timelock:           currentHeight,
			PublicKeys:         []ecdsa.PublicKey{renterPubkey, hostPubkey},
			SignaturesRequired: 2,
		}
	}

	// TODO: 代码需要优化下，golang中case也可以逗号并列，但是RLPHash这个方法就无法识别。。。
	switch dataType := originalData.(type) {
	case types.HostAnnouncement:
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&singlePubkey), dataType.RLPHash().Bytes(), singleSig) {
			return errInvalidHostSig
		}

	case types.StorageProof:
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&singlePubkey), dataType.RLPHash().Bytes(), singleSig) {
			return errInvalidHostSig
		}

	case types.StorageContract:
		if uc.UnlockHash() != common.Hash(dataType.UnlockHash) {
			return errWrongUnlockCondition
		}
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&renterPubkey), dataType.RLPHash().Bytes(), renterSig) {
			return errInvalidRenterSig
		}
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&hostPubkey), dataType.RLPHash().Bytes(), hostSig) {
			return errInvalidHostSig
		}

	case types.StorageContractRevision:
		if uc.UnlockHash() != common.Hash(dataType.NewUnlockHash) {
			return errWrongUnlockCondition
		}
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&renterPubkey), dataType.RLPHash().Bytes(), renterSig) {
			return errInvalidRenterSig
		}
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&hostPubkey), dataType.RLPHash().Bytes(), hostSig) {
			return errInvalidHostSig
		}

	default:
		return errNoStorageContractType
	}

	return nil
}

func CheckStorageProof(evm *EVM, sp types.StorageProof, currentHeight types.BlockHeight) error {
	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	fc, err := GetStorageContract(db, sp.ParentID)
	if err != nil {
		return err
	}

	if fc.WindowStart > currentHeight {
		return errors.New("too early to submit storage proof")
	}

	if fc.WindowEnd < currentHeight {
		return errors.New("too late to submit storage proof")
	}

	// Check that the storage proof itself is valid.
	segmentIndex, err := storageProofSegment(db, sp.ParentID)
	if err != nil {
		return err
	}

	leaves := CalculateLeaves(fc.FileSize)

	segmentLen := uint64(SegmentSize)

	// If this segment chosen is the final segment, it should only be as
	// long as necessary to complete the filesize.
	if segmentIndex == leaves-1 {
		segmentLen = fc.FileSize % SegmentSize
	}

	if segmentLen == 0 {
		segmentLen = uint64(SegmentSize)
	}

	verified := VerifySegment(
		sp.Segment[:segmentLen],
		sp.HashSet,
		leaves,
		segmentIndex,
		fc.FileMerkleRoot,
	)
	if !verified && fc.FileSize > 0 {
		return errInvalidStorageProof
	}

	return nil
}

// TODO:

func VerifySegment(segment []byte, hashSet []common.Hash, leaves, segmentIndex uint64, merkleRoot common.Hash) bool {
	return true
}

func storageProofSegment(db ethdb.Database, ParentID types.StorageContractID) (uint64, error) {
	return 0, nil
}

func CalculateLeaves(fileSize uint64) uint64 {
	return 0
}
