// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"reflect"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/rawdb"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
)

var (
	errZeroCollateral                          = errors.New("the payout of storage contract is less 0")
	errZeroOutput                              = errors.New("the output of storage contract is less 0")
	errStorageContractValidOutputSumViolation  = errors.New("storage contract has invalid valid proof output sums")
	errStorageContractMissedOutputSumViolation = errors.New("storage contract has invalid missed proof output sums")
	errRevisionOutputSumViolation              = errors.New("the missed proof ouput sum and valid proof output sum equal")
	errStorageContractWindowEndViolation       = errors.New("storage contract window must end at least one block after it starts")
	errStorageContractWindowStartViolation     = errors.New("storage contract window must start in the future")
	errLateRevision                            = errors.New("storage contract revision submitted after deadline")
	errLowRevisionNumber                       = errors.New("transaction has a storage contract with an outdated revision number")
	errRevisionValidPayouts                    = errors.New("storage contract revision has altered valid payout")
	errRevisionMissedPayouts                   = errors.New("storage contract revision has altered missed payout")
	errWrongUnlockCondition                    = errors.New("the unlock hash of storage contract not match unlock condition")
	errNoStorageContractType                   = errors.New("no this storage contract type")
	errInvalidStorageProof                     = errors.New("invalid storage proof")
	errUnfinishedStorageContract               = errors.New("storage contract has not yet opened")
)

// check whether a new StorageContract is valid
func CheckCreateContract(state StateDB, sc types.StorageContract, currentHeight uint64) error {
	if sc.ClientCollateral.Value.Sign() <= 0 {
		return errZeroCollateral
	}
	if sc.HostCollateral.Value.Sign() <= 0 {
		return errZeroCollateral
	}

	// check that start and expiration are reasonable values.
	if sc.WindowStart <= currentHeight {
		return errStorageContractWindowStartViolation
	}
	if sc.WindowEnd <= sc.WindowStart {
		return errStorageContractWindowEndViolation
	}

	// check that the proof outputs sum to the payout
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

	payout := new(big.Int).SetInt64(0)
	payout.Add(sc.ClientCollateral.Value, sc.HostCollateral.Value)
	if validProofOutputSum.Cmp(payout) != 0 {
		return errStorageContractValidOutputSumViolation
	}

	// if the missedProofOutputSum is greater than the payout, error
	// for contract renew, money will be burnt as punishment for
	// not able to submit the storage proof
	if missedProofOutputSum.Cmp(payout) > 0 {
		return errStorageContractMissedOutputSumViolation
	}

	// check if balance is enough for collateral
	clientAddr := sc.ClientCollateral.Address
	clientCollateralAmount := sc.ClientCollateral.Value
	hostAddr := sc.HostCollateral.Address
	hostCollateralAmount := sc.HostCollateral.Value

	clientBalance := state.GetBalance(clientAddr)
	if clientBalance.Cmp(clientCollateralAmount) == -1 {
		return errors.New("client has not enough balance for storage contract collateral")
	}

	hostBalance := state.GetBalance(hostAddr)
	if hostBalance.Cmp(hostCollateralAmount) == -1 {
		return errors.New("host has not enough balance for storage contract collateral")
	}

	err := CheckMultiSignatures(sc, sc.Signatures)
	if err != nil {
		log.Error("failed to check signature for create contract", "err", err)
		return err
	}

	return nil
}

// check whether a new StorageContractRevision is valid
func CheckRevisionContract(state StateDB, scr types.StorageContractRevision, currentHeight uint64, contractAddr common.Address) error {

	// check whether it has proofed
	windowEndStr := strconv.FormatUint(scr.NewWindowEnd, 10)
	statusAddr := common.BytesToAddress([]byte(StrPrefixExpSC + windowEndStr))

	statusContent := state.GetState(statusAddr, scr.ParentID)
	flag := statusContent.Bytes()[11:12]
	if bytes.Equal(flag, []byte{'1'}) {
		return errors.New("can not do contract revision after storage proof")
	}

	// check that start and expiration are reasonable values.
	if scr.NewWindowStart <= currentHeight {
		return errStorageContractWindowStartViolation
	}
	if scr.NewWindowEnd <= scr.NewWindowStart {
		return errStorageContractWindowEndViolation
	}

	// check that the valid outputs and missed outputs sum whether are the same
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

	// validProofOutputSum must be greater or equal to missedProofOutputSum
	if validProofOutputSum.Cmp(missedProofOutputSum) < 0 {
		return errRevisionOutputSumViolation
	}

	if err := CheckMultiSignatures(scr, scr.Signatures); err != nil {
		return err
	}

	// retrieve origin storage contract
	windowStartHash := state.GetState(contractAddr, KeyWindowStart)
	revisionNumHash := state.GetState(contractAddr, KeyRevisionNumber)
	unHash := state.GetState(contractAddr, KeyUnlockHash)
	clientVpoHash := state.GetState(contractAddr, KeyClientValidProofOutput)
	hostVpoHash := state.GetState(contractAddr, KeyHostValidProofOutput)
	clientMpoHash := state.GetState(contractAddr, KeyClientMissedProofOutput)
	hostMpoHash := state.GetState(contractAddr, KeyHostMissedProofOutput)

	// Check that the height is less than sc.WindowStart - revisions are
	// not allowed to be submitted once the storage proof window has
	// opened.  This reduces complexity for unconfirmed transactions.
	wStart := new(big.Int).SetBytes(windowStartHash.Bytes()).Uint64()
	if currentHeight > wStart {
		return errLateRevision
	}

	// Check that the revision number of the revision is greater than the
	// revision number of the existing storage contract.
	reNum := new(big.Int).SetBytes(revisionNumHash.Bytes()).Uint64()
	if reNum > scr.NewRevisionNumber {
		return errLowRevisionNumber
	}

	// Check that the unlock conditions match the unlock hash.
	if scr.UnlockConditions.UnlockHash() != unHash {
		return errWrongUnlockCondition
	}

	// Check that the payout of the revision matches the payout of the
	// original, and that the payouts match each other.
	oldValidPayout := new(big.Int).SetInt64(0)
	oldMissedPayout := new(big.Int).SetInt64(0)

	clientVpo := new(big.Int).SetBytes(clientVpoHash.Bytes())
	hostVpo := new(big.Int).SetBytes(hostVpoHash.Bytes())
	oldValidPayout.Add(clientVpo, hostVpo)

	clientMpo := new(big.Int).SetBytes(clientMpoHash.Bytes())
	hostMpo := new(big.Int).SetBytes(hostMpoHash.Bytes())
	oldMissedPayout.Add(clientMpo, hostMpo)

	if validProofOutputSum.Cmp(oldValidPayout) != 0 {
		return errRevisionValidPayouts
	}

	// For missed outputs only have 2 out: client and host, and client's deduction not add to host.
	// So the sum of missed outputs is less than or equal to old payout.
	if missedProofOutputSum.Cmp(oldMissedPayout) == 1 {
		return errRevisionMissedPayouts
	}

	return nil
}

// check whether a new StorageContractRevision is valid
func CheckMultiSignatures(originalData types.StorageContractRLPHash, signatures [][]byte) error {
	if len(signatures) == 0 {
		return errors.New("no signatures for verification")
	}

	var (
		singleSig, clientSig, hostSig []byte
		clientPubkey, hostPubkey      *ecdsa.PublicKey
		err                           error
		uc                            types.UnlockConditions
	)

	dataHash := originalData.RLPHash()

	if len(signatures) == 1 {
		singleSig = signatures[0]

		// if we can recover the public key, indicate that check sig is ok
		recoverKey, err := crypto.SigToPub(dataHash.Bytes(), singleSig)
		if err != nil {
			return err
		}

		// if it's a host announce, we must check the node public key is equal to the recover key
		if ha, ok := originalData.(types.HostAnnouncement); ok {
			hostNode, err := enode.ParseV4(ha.NetAddress)
			if err != nil {
				return fmt.Errorf("invalid host announce address: %v", err)
			}

			urlKey := hostNode.Pubkey()
			if !crypto.IsEqualPublicKey(recoverKey, urlKey) {
				return fmt.Errorf("announced host net address is not generated by self hostnode")
			}
		}
	} else if len(signatures) == 2 {
		clientSig = signatures[0]
		hostSig = signatures[1]
		clientPubkey, err = crypto.SigToPub(dataHash.Bytes(), clientSig)
		if err != nil {
			return err
		}
		hostPubkey, err = crypto.SigToPub(dataHash.Bytes(), hostSig)
		if err != nil {
			return err
		}

		uc = types.UnlockConditions{
			PaymentAddresses:   []common.Address{crypto.PubkeyToAddress(*clientPubkey), crypto.PubkeyToAddress(*hostPubkey)},
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

		if uc.UnlockHash() != originUnlockHash {
			return errWrongUnlockCondition
		}
	}

	return nil
}

// check whether a new StorageProof is valid
func CheckStorageProof(state StateDB, sp types.StorageProof, currentHeight uint64, statusAddr common.Address, contractAddr common.Address) error {

	// check whether it proofed repeatedly
	statusContent := state.GetState(statusAddr, sp.ParentID)
	flag := statusContent.Bytes()[11:12]
	if bytes.Equal(flag, []byte{'1'}) {
		return errors.New("can not submit storage proof repeatedly")
	}

	// retrieve the storage contract info
	windowStartHash := state.GetState(contractAddr, KeyWindowStart)
	windowStart := new(big.Int).SetBytes(windowStartHash.Bytes()).Uint64()

	windowEndHash := state.GetState(contractAddr, KeyWindowEnd)
	windowEnd := new(big.Int).SetBytes(windowEndHash.Bytes()).Uint64()

	fileMerkleRoot := state.GetState(contractAddr, KeyFileMerkleRoot)

	fileSizeHash := state.GetState(contractAddr, KeyFileSize)
	fileSize := new(big.Int).SetBytes(fileSizeHash.Bytes()).Uint64()

	if windowStart > currentHeight {
		return errors.New("too early to submit storage proof")
	}

	if windowEnd < currentHeight {
		return errors.New("too late to submit storage proof")
	}

	// check signature
	err := CheckMultiSignatures(sp, [][]byte{sp.Signature})
	if err != nil {
		log.Error("failed to check signature for storage proof", "err", err)
		return err
	}

	// check that the storage proof itself is valid.

	segmentIndex, err := storageProofSegment(state, windowStart, fileSize, sp.ParentID, currentHeight)
	if err != nil {
		return err
	}

	leaves := CalculateLeaves(fileSize)

	segmentLen := uint64(merkle.LeafSize)

	// if this segment chosen is the final segment, it should only be as
	// long as necessary to complete the file size.
	if segmentIndex == leaves-1 {
		segmentLen = fileSize % merkle.LeafSize
	}

	if segmentLen == 0 {
		segmentLen = uint64(merkle.LeafSize)
	}

	verified := VerifySegment(
		sp.Segment[:segmentLen],
		sp.HashSet,
		leaves,
		segmentIndex,
		fileMerkleRoot,
	)
	if !verified && fileSize > 0 {
		return errInvalidStorageProof
	}

	return nil
}

// check whether host has really stored the file
func VerifySegment(segment []byte, hashSet []common.Hash, leaves, segmentIndex uint64, merkleRoot common.Hash) bool {

	// convert base and hashSet to proofSet
	proofSet := make([][]byte, len(hashSet)+1)
	proofSet[0] = segment
	for i := range hashSet {
		proofSet[i+1] = hashSet[i][:]
	}
	return VerifyProof(merkleRoot[:], proofSet, segmentIndex, leaves)
}

// get segment index by random
func storageProofSegment(state StateDB, windowStart, fileSize uint64, scID common.Hash, currentHeight uint64) (uint64, error) {

	// Get the trigger block id that parent of windowStart.
	triggerHeight := windowStart - 1
	if triggerHeight > currentHeight {
		return 0, errUnfinishedStorageContract
	}

	db := state.Database().TrieDB().DiskDB().(ethdb.Database)
	blockHash := rawdb.ReadCanonicalHash(db, uint64(triggerHeight))
	if reflect.DeepEqual(blockHash, common.Hash{}) {
		return 0, errors.New("can not read block hash of the trigger height for storage proof seed")
	}

	seed := crypto.Keccak256Hash(blockHash[:], scID[:])
	numSegments := int64(CalculateLeaves(fileSize))

	// index = seedInt % numSegments，index in [0，numSegments]
	seedInt := new(big.Int).SetBytes(seed[:])
	index := seedInt.Mod(seedInt, big.NewInt(numSegments)).Uint64()
	return index, nil
}

// calculate the num of leaves formed by the given file
func CalculateLeaves(fileSize uint64) uint64 {
	numSegments := fileSize / merkle.LeafSize
	if fileSize == 0 || fileSize%merkle.LeafSize != 0 {
		numSegments++
	}
	return numSegments
}

// verify merkle root of given segment
func VerifyProof(merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64) bool {
	hasher := sha256.New()

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
	sum := leafHash(hasher, proofSet[height])
	height++
	stableEnd := proofIndex

	for {
		subTreeStartIndex := (proofIndex / (1 << uint(height))) * (1 << uint(height))
		subTreeEndIndex := subTreeStartIndex + (1 << (uint(height))) - 1
		if subTreeEndIndex >= numLeaves {
			break
		}

		stableEnd = subTreeEndIndex

		if len(proofSet) <= height {
			return false
		}

		if proofIndex-subTreeStartIndex < 1<<uint(height-1) {
			sum = nodeHash(hasher, sum, proofSet[height])
		} else {
			sum = nodeHash(hasher, proofSet[height], sum)
		}
		height++
	}

	if stableEnd != numLeaves-1 {
		if len(proofSet) <= height {
			return false
		}

		sum = nodeHash(hasher, sum, proofSet[height])
		height++
	}

	for height < len(proofSet) {
		sum = nodeHash(hasher, proofSet[height], sum)
		height++
	}

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

func leafHash(h hash.Hash, data []byte) []byte {
	return HashSum(h, []byte{0x00}, data)
}

func nodeHash(h hash.Hash, a, b []byte) []byte {
	return HashSum(h, []byte{0x01}, a, b)
}
