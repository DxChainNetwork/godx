package storagehost

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

func handleContractCreate(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {
	s.SetBusy()
	defer s.ResetBusy()

	// this RPC call contains two request/response exchanges.
	s.SetDeadLine(storage.ContractCreateTime)

	if !h.externalConfig().AcceptingContracts {
		err := errors.New("host is not accepting new contracts")
		return err
	}

	// 1. Read ContractCreateRequest msg
	var req storage.ContractCreateRequest
	if err := beginMsg.Decode(&req); err != nil {
		return err
	}

	sc := req.StorageContract
	clientPK, err := crypto.SigToPub(sc.RLPHash().Bytes(), req.Sign)
	if err != nil {
		return ExtendErr("recover publicKey from signature failed", err)
	}

	// Check host balance >= storage contract cost
	hostAddress := sc.ValidProofOutputs[1].Address
	stateDB, err := h.ethBackend.GetBlockChain().State()
	if err != nil {
		return ExtendErr("get state db error", err)
	}

	if stateDB.GetBalance(hostAddress).Cmp(sc.HostCollateral.Value) < 0 {
		return ExtendErr("host balance insufficient", err)
	}

	account := accounts.Account{Address: hostAddress}
	wallet, err := h.ethBackend.AccountManager().Find(account)
	if err != nil {
		return ExtendErr("find host account error", err)
	}

	hostContractSign, err := wallet.SignHash(account, sc.RLPHash().Bytes())
	if err != nil {
		return ExtendErr("host account sign storage contract error", err)
	}

	// Ecrecover host pk for setup unlock conditions
	hostPK, err := crypto.SigToPub(sc.RLPHash().Bytes(), hostContractSign)
	if err != nil {
		return ExtendErr("Ecrecover pk from sign error", err)
	}

	sc.Signatures = [][]byte{req.Sign, hostContractSign}

	// Check an incoming storage contract matches the host's expectations for a valid contract
	if req.Renew {
		oldContractID := req.OldContractID
		err = verifyRenewedContract(h, &sc, clientPK, hostPK, oladContractID)
		if err != nil {
			return ExtendErr("host verify renewed storage contract failed ", err)
		}
	} else {
		err = verifyStorageContract(h, &sc, clientPK, hostPK)
		if err != nil {
			return ExtendErr("host verify storage contract failed ", err)
		}
	}

	// 2. After check, send host contract sign to client
	if err := s.SendStorageContractCreationHostSign(hostContractSign); err != nil {
		return ExtendErr("send storage contract create sign by host", err)
	}

	// 3. Wait for the client revision sign
	var clientRevisionSign []byte
	msg, err := s.ReadMsg()
	if err != nil {
		return err
	}

	if err = msg.Decode(&clientRevisionSign); err != nil {
		return err
	}

	// Reconstruct revision locally by host
	storageContractRevision := types.StorageContractRevision{
		ParentID: sc.RLPHash(),
		UnlockConditions: types.UnlockConditions{
			PaymentAddresses: []common.Address{
				crypto.PubkeyToAddress(*clientPK),
				crypto.PubkeyToAddress(*hostPK),
			},
			SignaturesRequired: 2,
		},
		NewRevisionNumber:     1,
		NewFileSize:           sc.FileSize,
		NewFileMerkleRoot:     sc.FileMerkleRoot,
		NewWindowStart:        sc.WindowStart,
		NewWindowEnd:          sc.WindowEnd,
		NewValidProofOutputs:  sc.ValidProofOutputs,
		NewMissedProofOutputs: sc.MissedProofOutputs,
		NewUnlockHash:         sc.UnlockHash,
	}

	// Sign revision by storage host
	hostRevisionSign, err := wallet.SignHash(account, storageContractRevision.RLPHash().Bytes())
	if err != nil {
		return ExtendErr("host sign revison error", err)
	}

	storageContractRevision.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}

	so := StorageResponsibility{
		SectorRoots:              nil,
		ContractCost:             h.externalConfig().ContractPrice,
		LockedStorageDeposit:     common.NewBigInt(sc.ValidProofOutputs[1].Value.Int64()).Sub(h.externalConfig().ContractPrice),
		PotentialStorageRevenue:  common.BigInt0,
		RiskedStorageDeposit:     common.BigInt0,
		NegotiationBlockNumber:   h.blockHeight,
		OriginStorageContract:    sc,
		StorageContractRevisions: []types.StorageContractRevision{storageContractRevision},
	}

	if req.Renew {
		h.lock.RLock()
		oldso, err := getStorageResponsibility(h.db, req.OldContractID)
		h.lock.RUnlock()
		if err != nil {
			h.log.Warn("Unable to get old storage responsibility when renewing", "err", err)
		} else {
			so.SectorRoots = oldso.SectorRoots
		}
		renewRevenue := renewBasePrice(so, h.externalConfig(), req.StorageContract)
		so.ContractCost = common.NewBigInt(req.StorageContract.ValidProofOutputs[1].Value.Int64()).Sub(h.externalConfig().ContractPrice).Sub(renewRevenue)
		so.PotentialStorageRevenue = renewRevenue
		so.RiskedStorageDeposit = renewBaseDeposit(so, h.externalConfig(), req.StorageContract)
	}

	if err := finalizeStorageResponsibility(h, so); err != nil {
		return ExtendErr("finalize storage responsibility error", err)
	}

	return s.SendStorageContractCreationHostRevisionSign(hostRevisionSign)
}

// verifyStorageContract verify the validity of the storage contract. If discrepancy found, return error
func verifyStorageContract(h *StorageHost, sc *types.StorageContract, clientPK *ecdsa.PublicKey, hostPK *ecdsa.PublicKey) error {
	h.lock.RLock()
	blockHeight := h.blockHeight
	lockedStorageDeposit := h.financialMetrics.LockedStorageDeposit
	hostAddress := crypto.PubkeyToAddress(*hostPK)
	config := h.config
	h.lock.RUnlock()

	externalConfig := h.externalConfig()

	// A new file contract should have a file size of zero
	if sc.FileSize != 0 {
		return errBadFileSize
	}

	if sc.FileMerkleRoot != (common.Hash{}) {
		return errBadFileMerkleRoot
	}

	// WindowStart must be at least postponedExecutionBuffer blocks into the future
	//if sc.WindowStart <= blockHeight+postponedExecutionBuffer {
	//	h.log.Debug("A renter tried to form a contract that had a window start which was too soon. The contract started at %v, the current height is %v, the postponedExecutionBuffer is %v, and the comparison was %v <= %v\n", sc.WindowStart, blockHeight, postponedExecutionBuffer, sc.WindowStart, blockHeight+postponedExecutionBuffer)
	//	return errEarlyWindow
	//}

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
	if depositMinusContractPrice.Cmp(config.MaxDeposit) > 0 {
		return errMaxCollateralReached
	}
	// Check that the host has enough room in the collateral budget to add this
	// collateral.
	if lockedStorageDeposit.Add(depositMinusContractPrice).Cmp(config.DepositBudget) > 0 {
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

// finalizeStorageResponsibility
func finalizeStorageResponsibility(h *StorageHost, so StorageResponsibility) error {
	// Get a lock on the storage responsibility
	lockErr := h.checkAndTryLockStorageResponsibility(so.id(), storage.ResponsibilityLockTimeout)
	if lockErr != nil {
		return lockErr
	}
	defer h.checkAndUnlockStorageResponsibility(so.id())

	if err := h.insertStorageResponsibility(so); err != nil {
		return err
	}
	return nil
}

// renewBasePrice returns the base cost of the storage in the  contract,
// using the host external settings and the starting file contract.
func renewBasePrice(so StorageResponsibility, settings storage.HostExtConfig, fc types.StorageContract) common.BigInt {
	if fc.WindowEnd <= so.proofDeadline() {
		return common.BigInt0
	}
	timeExtension := fc.WindowEnd - so.proofDeadline()
	return settings.StoragePrice.Mult(common.NewBigIntUint64(fc.FileSize)).Mult(common.NewBigIntUint64(uint64(timeExtension)))
}

// renewBaseDeposit returns the base cost of the storage in the  contract,
// using the host external settings and the starting  contract.
func renewBaseDeposit(so StorageResponsibility, settings storage.HostExtConfig, fc types.StorageContract) common.BigInt {
	if fc.WindowEnd <= so.proofDeadline() {
		return common.BigInt0
	}
	timeExtension := fc.WindowEnd - so.proofDeadline()
	return settings.Deposit.Mult(common.NewBigIntUint64(fc.FileSize)).Mult(common.NewBigIntUint64(uint64(timeExtension)))
}

// verifyRenewedContract checks whether the renewed contract matches the previous and appropriate payments.
func verifyRenewedContract(h *StorageHost, sc *types.StorageContract, clientPK *ecdsa.PublicKey, hostPK *ecdsa.PublicKey, oldContractID common.Hash) error {
	h.lock.RLock()
	blockHeight := h.blockHeight
	lockedStorageDeposit := h.financialMetrics.LockedStorageDeposit
	hostAddress := crypto.PubkeyToAddress(*hostPK)
	config := h.config
	so, err := getStorageResponsibility(h.db, oldContractID)
	if err != nil {
		h.lock.RUnlock()
		return fmt.Errorf("failed to get storage responsibility in verifyRenewedContract,error: %v", err)
	}
	h.lock.RUnlock()

	externalConfig := h.externalConfig()

	// check that the file size and merkle root whether match the previous.
	if sc.FileSize != so.fileSize() {
		return errBadFileSize
	}
	if sc.FileMerkleRoot != so.merkleRoot() {
		return errBadFileMerkleRoot
	}

	// WindowStart must be at least revisionSubmissionBuffer blocks into the future
	//if sc.WindowStart <= blockHeight+postponedExecutionBuffer {
	//	return errEarlyWindow
	//}

	// WindowEnd must be at least settings.WindowSize blocks after WindowStart
	if sc.WindowEnd < sc.WindowStart+externalConfig.WindowSize {
		return errSmallWindow
	}

	// WindowStart must not be more than settings.MaxDuration blocks into the future
	if sc.WindowStart > blockHeight+externalConfig.MaxDuration {
		return errLongDuration
	}

	// ValidProofOutputs shoud have 2 outputs (renter + host) and missed
	// outputs should have 3 (renter + host)
	if len(sc.ValidProofOutputs) != 2 || len(sc.MissedProofOutputs) != 2 {
		return errBadContractOutputCounts
	}

	// The address of the valid and missed proof outputs must match the host's address
	if sc.ValidProofOutputs[1].Address != hostAddress || sc.MissedProofOutputs[1].Address != hostAddress {
		return errBadPayoutUnlockHashes
	}

	// Check that the collateral does not exceed the maximum amount of
	// collateral allowed.
	basePrice := renewBasePrice(so, externalConfig, *sc)
	expectedCollateral := common.NewBigInt(sc.ValidProofOutputs[1].Value.Int64()).Sub(externalConfig.ContractPrice).Sub(basePrice)
	if expectedCollateral.Cmp(externalConfig.MaxDeposit) > 0 {
		return errMaxCollateralReached
	}

	// Check that the host has enough room in the deposit budget to add this
	// collateral.
	if lockedStorageDeposit.Add(expectedCollateral).Cmp(config.DepositBudget) > 0 {
		return errCollateralBudgetExceeded
	}

	// Check that the valid and missed proof outputs contain enough money
	baseCollateral := renewBaseDeposit(so, externalConfig, *sc)
	totalPayout := basePrice.Add(baseCollateral)
	if sc.ValidProofOutputs[1].Value.Cmp(totalPayout.BigIntPtr()) < 0 {
		return errLowHostValidOutput
	}
	expectedHostMissedOutput := common.NewBigInt(sc.ValidProofOutputs[1].Value.Int64()).Sub(basePrice).Sub(baseCollateral)
	if sc.MissedProofOutputs[1].Value.Cmp(expectedHostMissedOutput.BigIntPtr()) < 0 {
		return errLowHostMissedOutput
	}

	// The unlock hash for the storage contract must match the unlock hash that
	// the host knows how to spend.
	expectedUH := types.UnlockConditions{
		PaymentAddresses: []common.Address{
			crypto.PubkeyToAddress(*clientPK),
			hostAddress,
		},
		SignaturesRequired: 2,
	}.UnlockHash()
	if sc.UnlockHash != expectedUH {
		return errBadUnlockHash
	}

	return nil
}
