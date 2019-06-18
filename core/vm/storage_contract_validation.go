// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"hash"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/rawdb"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"golang.org/x/crypto/sha3"
)

var (
	errZeroCollateral                          = errors.New("the payout of form contract is less 0")
	errZeroOutput                              = errors.New("the output of form contract is less 0")
	errStorageContractValidOutputSumViolation  = errors.New("storage contract has invalid valid proof output sums")
	errStorageContractMissedOutputSumViolation = errors.New("storage contract has invalid missed proof output sums")
	errStorageContractOutputSumViolation       = errors.New("the missed proof ouput sum and valid proof output sum not equal")

	errStorageContractWindowEndViolation   = errors.New("storage contract window must end at least one block after it starts")
	errStorageContractWindowStartViolation = errors.New("storage contract window must start in the future")

	errTimelockNotSatisfied  = errors.New("timelock has not been met")
	errLateRevision          = errors.New("storage contract revision submitted after deadline")
	errLowRevisionNumber     = errors.New("transaction has a storage contract with an outdated revision number")
	errRevisionValidPayouts  = errors.New("storage contract revision has altered valid payout")
	errRevisionMissedPayouts = errors.New("storage contract revision has altered missed payout")
	errWrongUnlockCondition  = errors.New("the unlockhash of storage contract not match unlockcondition")
	errInvalidClientSig      = errors.New("invalid client signatures")
	errInvalidHostSig        = errors.New("invalid host signatures")
	errNoStorageContractType = errors.New("no this storage contract type")

	errInvalidStorageProof = errors.New("invalid storage proof")

	errUnfinishedStorageContract = errors.New("storage contract has not yet opened")
)

const (
	SegmentSize = 64
)

func CheckFormContract(evm *EVM, sc types.StorageContract, currentHeight uint64) error {

	// check if this file contract exist
	scID := sc.ID()
	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	_, err := GetStorageContract(db, scID)
	if err == nil {
		return errors.New("this file contract exist")
	}

	if sc.ClientCollateral.Value.Sign() <= 0 {
		return errZeroCollateral
	}
	if sc.HostCollateral.Value.Sign() <= 0 {
		return errZeroCollateral
	}

	// Check that start and expiration are reasonable values.
	if sc.WindowStart <= currentHeight {
		return errStorageContractWindowStartViolation
	}
	if sc.WindowEnd <= sc.WindowStart {
		return errStorageContractWindowEndViolation
	}

	// Check that the proof outputs sum to the payout
	validProofOutputSum := new(big.Int).SetInt64(0)
	missedProofOutputSum := new(big.Int).SetInt64(0)
	for _, output := range sc.ValidProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		validProofOutputSum = validProofOutputSum.Add(validProofOutputSum, output.Value)
	}
	for _, output := range sc.MissedProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		missedProofOutputSum = missedProofOutputSum.Add(missedProofOutputSum, output.Value)
	}

	payout := sc.ClientCollateral.Value.Add(sc.ClientCollateral.Value, sc.HostCollateral.Value)
	if validProofOutputSum.Cmp(payout) != 0 {
		return errStorageContractValidOutputSumViolation
	}
	if missedProofOutputSum.Cmp(payout) != 0 {
		return errStorageContractMissedOutputSumViolation
	}

	// check if balance is enough for collateral
	clientAddr := sc.ClientCollateral.Address
	clientCollateralAmount := sc.ClientCollateral.Value
	hostAddr := sc.HostCollateral.Address
	hostCollateralAmount := sc.HostCollateral.Value

	clientBalance := evm.StateDB.GetBalance(clientAddr)
	if clientBalance.Cmp(clientCollateralAmount) == -1 {
		return errors.New("client has not enough balance for file contract collateral")
	}

	hostBalance := evm.StateDB.GetBalance(hostAddr)
	if hostBalance.Cmp(hostCollateralAmount) == -1 {
		return errors.New("host has not enough balance for file contract collateral")
	}

	err = CheckMultiSignatures(sc, currentHeight, sc.Signatures)
	if err != nil {
		log.Error("failed to check signature for form contract", "err", err)
		return err
	}

	return nil
}

func CheckReversionContract(evm *EVM, scr types.StorageContractRevision, currentHeight uint64) error {

	if scr.UnlockConditions.Timelock > currentHeight {
		return errTimelockNotSatisfied
	}

	// Check that start and expiration are reasonable values.
	if scr.NewWindowStart <= currentHeight {
		return errStorageContractWindowStartViolation
	}
	if scr.NewWindowEnd <= scr.NewWindowStart {
		return errStorageContractWindowEndViolation
	}

	// Check that the valid outputs and missed outputs sum to the same
	// value.
	validProofOutputSum := new(big.Int).SetInt64(0)
	missedProofOutputSum := new(big.Int).SetInt64(0)
	for _, output := range scr.NewValidProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		validProofOutputSum = validProofOutputSum.Add(validProofOutputSum, output.Value)
	}
	for _, output := range scr.NewMissedProofOutputs {
		if output.Value.Sign() <= 0 {
			return errZeroOutput
		}
		missedProofOutputSum = missedProofOutputSum.Add(missedProofOutputSum, output.Value)
	}
	if validProofOutputSum.Cmp(missedProofOutputSum) != 0 {
		return errStorageContractOutputSumViolation
	}

	if err := CheckMultiSignatures(scr, 0, scr.Signatures); err != nil {
		return err
	}

	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	sc, err := GetStorageContract(db, scr.ParentID)
	if err != nil {
		return err
	}

	// Check that the height is less than sc.WindowStart - revisions are
	// not allowed to be submitted once the storage proof window has
	// opened.  This reduces complexity for unconfirmed transactions.
	if currentHeight > sc.WindowStart {
		return errLateRevision
	}

	// Check that the revision number of the revision is greater than the
	// revision number of the existing file contract.
	if sc.RevisionNumber >= scr.NewRevisionNumber {
		return errLowRevisionNumber
	}

	// Check that the unlock conditions match the unlock hash.
	if scr.UnlockConditions.UnlockHash() != common.Hash(sc.UnlockHash) {
		return errWrongUnlockCondition
	}

	// Check that the payout of the revision matches the payout of the
	// original, and that the payouts match each other.
	validPayout := new(big.Int).SetInt64(0)
	missedPayout := new(big.Int).SetInt64(0)
	oldPayout := new(big.Int).SetInt64(0)
	for _, output := range scr.NewValidProofOutputs {
		validPayout = validPayout.Add(validPayout, output.Value)
	}
	for _, output := range scr.NewMissedProofOutputs {
		missedPayout = missedPayout.Add(missedPayout, output.Value)
	}
	for _, output := range sc.ValidProofOutputs {
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

func CheckMultiSignatures(originalData types.StorageContractRLPHash, currentHeight uint64, signatures [][]byte) error {
	if len(signatures) == 0 {
		return errors.New("no signatures for verification")
	}

	var (
		singleSig, clientSig, hostSig          []byte
		singlePubkey, clientPubkey, hostPubkey ecdsa.PublicKey
		err                                    error
		uc                                     types.UnlockConditions
	)

	dataHash := originalData.RLPHash()

	if len(signatures) == 1 {
		singleSig = signatures[0]
		singlePubkey, err = RecoverPubkeyFromSignature(dataHash, singleSig)
		if err != nil {
			return err
		}

		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&singlePubkey), dataHash[:], singleSig) {
			return errInvalidHostSig
		}
	} else if len(signatures) == 2 {
		clientSig = signatures[0]
		hostSig = signatures[1]
		clientPubkey, err = RecoverPubkeyFromSignature(dataHash, clientSig)
		if err != nil {
			return err
		}
		hostPubkey, err = RecoverPubkeyFromSignature(dataHash, hostSig)
		if err != nil {
			return err
		}

		uc = types.UnlockConditions{
			Timelock:           currentHeight,
			PaymentAddresses:   []common.Address{crypto.PubkeyToAddress(clientPubkey), crypto.PubkeyToAddress(hostPubkey)},
			SignaturesRequired: 2,
		}

		originUnlockHash := common.Hash{}
		switch dataType := originalData.(type) {
		case types.StorageContract:
			originUnlockHash = dataType.UnlockHash
		case types.StorageContractRevision:
			originUnlockHash = dataType.NewUnlockHash
		default:
			return errNoStorageContractType
		}

		if uc.UnlockHash() != common.Hash(originUnlockHash) {
			return errWrongUnlockCondition
		}
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&clientPubkey), dataHash[:], clientSig) {
			return errInvalidClientSig
		}
		if !VerifyStorageContractSignatures(crypto.FromECDSAPub(&hostPubkey), dataHash[:], hostSig) {
			return errInvalidHostSig
		}
	}

	return nil
}

func CheckStorageProof(evm *EVM, sp types.StorageProof, currentHeight uint64) error {
	db := evm.StateDB.Database().TrieDB().DiskDB().(ethdb.Database)
	sc, err := GetStorageContract(db, sp.ParentID)
	if err != nil {
		return err
	}

	if sc.WindowStart > currentHeight {
		return errors.New("too early to submit storage proof")
	}

	if sc.WindowEnd < currentHeight {
		return errors.New("too late to submit storage proof")
	}

	// check signature
	err = CheckMultiSignatures(sp, currentHeight, [][]byte{sp.Signature})
	if err != nil {
		log.Error("failed to check signature for storage proof", "err", err)
		return err
	}

	// Check that the storage proof itself is valid.
	segmentIndex, err := storageProofSegment(db, sp.ParentID, currentHeight)
	if err != nil {
		return err
	}

	leaves := CalculateLeaves(sc.FileSize)

	segmentLen := uint64(SegmentSize)

	// If this segment chosen is the final segment, it should only be as
	// long as necessary to complete the filesize.
	if segmentIndex == leaves-1 {
		segmentLen = sc.FileSize % SegmentSize
	}

	if segmentLen == 0 {
		segmentLen = uint64(SegmentSize)
	}

	verified := VerifySegment(
		sp.Segment[:segmentLen],
		sp.HashSet,
		leaves,
		segmentIndex,
		sc.FileMerkleRoot,
	)
	if !verified && sc.FileSize > 0 {
		return errInvalidStorageProof
	}

	return nil
}

// check whether host store the file
func VerifySegment(segment []byte, hashSet []common.Hash, leaves, segmentIndex uint64, merkleRoot common.Hash) bool {
	// convert base and hashSet to proofSet
	proofSet := make([][]byte, len(hashSet)+1)
	proofSet[0] = segment
	for i := range hashSet {
		proofSet[i+1] = hashSet[i][:]
	}
	return VerifyProof(merkleRoot[:], proofSet, segmentIndex, leaves)
}

func storageProofSegment(db ethdb.Database, ParentID common.Hash, currentHeight uint64) (uint64, error) {

	// Check that the parent storage contract exists.
	sc, err := GetStorageContract(db, ParentID)
	if err != nil {
		return 0, err
	}

	// Get the trigger block id that parent of windowStart.
	triggerHeight := sc.WindowStart - 1
	if triggerHeight > currentHeight {
		return 0, errUnfinishedStorageContract
	}

	blockHash := rawdb.ReadCanonicalHash(db, uint64(triggerHeight))
	scID := sc.ID()
	seed := crypto.Keccak256Hash(blockHash[:], scID[:])
	numSegments := int64(CalculateLeaves(sc.FileSize))

	// index = seedInt % numSegments，index in [0，numSegments]
	seedInt := new(big.Int).SetBytes(seed[:])
	index := seedInt.Mod(seedInt, big.NewInt(numSegments)).Uint64()
	return index, nil
}

func CalculateLeaves(fileSize uint64) uint64 {
	numSegments := fileSize / SegmentSize
	if fileSize == 0 || fileSize%SegmentSize != 0 {
		numSegments++
	}
	return numSegments
}

func VerifyProof(merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64) bool {
	hasher := sha3.NewLegacyKeccak256()

	if merkleRoot == nil {
		return false
	}

	if proofIndex >= numLeaves {
		return false
	}

	height := 0
	if len(proofSet) <= height {
		return false
	}

	// proofSet[0] is the segment of the file
	sum := HashSum(hasher, proofSet[height])
	height++

	// While the current subtree (of height 'height') is complete, determine
	// the position of the next sibling using the complete subtree algorithm.
	// 'stableEnd' tells us the ending index of the last full subtree. It gets
	// initialized to 'proofIndex' because the first full subtree was the
	// subtree of height 1, created above (and had an ending index of
	// 'proofIndex').
	stableEnd := proofIndex
	for {
		// Determine if the subtree is complete. This is accomplished by
		// rounding down the proofIndex to the nearest 1 << 'height', adding 1
		// << 'height', and comparing the result to the number of leaves in the
		// Merkle tree.
		subTreeStartIndex := (proofIndex / (1 << uint(height))) * (1 << uint(height)) // round down to the nearest 1 << height
		subTreeEndIndex := subTreeStartIndex + (1 << (uint(height))) - 1              // subtract 1 because the start index is inclusive
		if subTreeEndIndex >= numLeaves {
			// If the Merkle tree does not have a leaf at index
			// 'subTreeEndIndex', then the subtree of the current height is not
			// a complete subtree.
			break
		}
		stableEnd = subTreeEndIndex

		// Determine if the proofIndex is in the first or the second half of
		// the subtree.
		if len(proofSet) <= height {
			return false
		}
		if proofIndex-subTreeStartIndex < 1<<uint(height-1) {
			sum = HashSum(hasher, sum, proofSet[height])
		} else {
			sum = HashSum(hasher, proofSet[height], sum)
		}
		height++
	}

	// Determine if the next hash belongs to an orphan that was elevated. This
	// is the case IFF 'stableEnd' (the last index of the largest full subtree)
	// is equal to the number of leaves in the Merkle tree.
	if stableEnd != numLeaves-1 {
		if len(proofSet) <= height {
			return false
		}
		sum = HashSum(hasher, sum, proofSet[height])
		height++
	}

	// All remaining elements in the proof set will belong to a left sibling.
	for height < len(proofSet) {
		sum = HashSum(hasher, proofSet[height], sum)
		height++
	}

	// Compare our calculated Merkle root to the desired Merkle root.
	if bytes.Equal(sum, merkleRoot) {
		return true
	}
	return false
}

// returns the hash of the input data using the specified algorithm.
func HashSum(h hash.Hash, data ...[]byte) []byte {
	h.Reset()
	for _, d := range data {
		// the Hash interface specifies that Write never returns an error
		_, _ = h.Write(d)
	}
	return h.Sum(nil)
}
