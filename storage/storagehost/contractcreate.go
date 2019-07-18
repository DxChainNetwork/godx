package storagehost

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/log"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

func (h *StorageHost) ContractCreateHandler(sp storage.Peer, contractCreateReqMsg p2p.Msg) {
	var contractCreateErr error
	defer func() {
		if contractCreateErr != nil {
			log.Error("contract create failed", "err", contractCreateErr.Error())
			sp.TriggerError(contractCreateErr)
		}
	}()

	if !h.externalConfig().AcceptingContracts {
		contractCreateErr = errors.New("host is not accepting new contracts")
		return
	}
	// 1. Read ContractCreateRequest msg
	var req storage.ContractCreateRequest
	if err := contractCreateReqMsg.Decode(&req); err != nil {
		contractCreateErr = fmt.Errorf("failed to decode the contract create request message: %s", err.Error())
		return
	}

	sc := req.StorageContract
	clientPK, err := crypto.SigToPub(sc.RLPHash().Bytes(), req.Sign)
	if err != nil {
		contractCreateErr = fmt.Errorf("failed to recover the public key from the signature: %s", err.Error())
		sp.TriggerError(err)
		return
	}

	// Check host balance >= storage contract cost
	hostAddress := sc.ValidProofOutputs[1].Address
	stateDB, err := h.ethBackend.GetBlockChain().State()
	if err != nil {
		contractCreateErr = fmt.Errorf("failed to get the state db: %s", err.Error())
		return
	}

	// check the storage host balance
	if stateDB.GetBalance(hostAddress).Cmp(sc.HostCollateral.Value) < 0 {
		contractCreateErr = fmt.Errorf("insufficient host balance")
		return
	}

	// based on the address, get the storage host's account used for signing the contract
	account := accounts.Account{Address: hostAddress}
	wallet, err := h.ethBackend.AccountManager().Find(account)
	if err != nil {
		contractCreateErr = fmt.Errorf("failed to get the account from the storage host: %s", err.Error())
		return
	}

	// sign the storage client
	hostContractSign, err := wallet.SignHash(account, sc.RLPHash().Bytes())
	if err != nil {
		contractCreateErr = fmt.Errorf("storage hostfailed to sign contract: %s", err.Error())
		return
	}

	// recover host pk for setup unlock conditions
	hostPK, err := crypto.SigToPub(sc.RLPHash().Bytes(), hostContractSign)
	if err != nil {
		contractCreateErr = fmt.Errorf("failed to recover the storage host's public key from the signature: %s", err.Error())
		return
	}

	sc.Signatures = [][]byte{req.Sign, hostContractSign}

	// Check an incoming storage contract matches the host's expectations for a valid contract
	if req.Renew {
		oldContractID := req.OldContractID
		err = verifyRenewedContract(h, &sc, clientPK, hostPK, oldContractID)
		if err != nil {
			contractCreateErr = fmt.Errorf("storage host failed to verify the renewed storage contract: %s", err.Error())
			return
		}
	} else {
		err = verifyStorageContract(h, &sc, clientPK, hostPK)
		if err != nil {
			contractCreateErr = fmt.Errorf("storage host failed to verify the storage contract: %s", err.Error())
			return
		}
	}

	// 2. After check, send host contract sign to client
	if err := sp.SendContractCreationHostSign(hostContractSign); err != nil {
		contractCreateErr = fmt.Errorf("storage host failed to send contract creation host sign: %s", err.Error())
		return
	}

	// 3. Wait for the client revision sign
	var clientRevisionSign []byte
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		contractCreateErr = fmt.Errorf("storage host failed to get client revision sign: %s", err.Error())
		return
	}

	if err = msg.Decode(&clientRevisionSign); err != nil {
		contractCreateErr = fmt.Errorf("storage host failed to decode client revision sign: %s", err.Error())
		return
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
		contractCreateErr = fmt.Errorf("storage host failed to sign the contract revision: %s", err.Error())
		return
	}

	storageContractRevision.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}

	h.lock.RLock()
	height := h.blockHeight
	h.lock.RUnlock()

	so := StorageResponsibility{
		SectorRoots:              nil,
		ContractCost:             h.externalConfig().ContractPrice,
		LockedStorageDeposit:     common.NewBigInt(sc.ValidProofOutputs[1].Value.Int64()).Sub(h.externalConfig().ContractPrice),
		PotentialStorageRevenue:  common.BigInt0,
		RiskedStorageDeposit:     common.BigInt0,
		NegotiationBlockNumber:   height,
		OriginStorageContract:    sc,
		StorageContractRevisions: []types.StorageContractRevision{storageContractRevision},
	}
	if req.Renew {
		h.lock.RLock()
		oldSr, err := getStorageResponsibility(h.db, req.OldContractID)
		h.lock.RUnlock()

		if err != nil {
			h.log.Warn("Unable to get old storage responsibility when renewing", "err", err)
		} else {
			so.SectorRoots = oldSr.SectorRoots
		}
		renewRevenue := renewBasePrice(so, h.externalConfig(), req.StorageContract)
		so.ContractCost = common.NewBigInt(req.StorageContract.ValidProofOutputs[1].Value.Int64()).Sub(h.externalConfig().ContractPrice).Sub(renewRevenue)
		so.PotentialStorageRevenue = renewRevenue
		so.RiskedStorageDeposit = renewBaseDeposit(so, h.externalConfig(), req.StorageContract)
	}
	if err := finalizeStorageResponsibility(h, so); err != nil {
		contractCreateErr = fmt.Errorf("storage host failed to finialize storage responsibility: %s", err.Error())
		return
	}

	if err := sp.SendContractCreationHostRevisionSign(hostRevisionSign); err != nil {
		contractCreateErr = fmt.Errorf("storage host failed to send contract creation revision sign: %s", err.Error())
		return
	}
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

// finalizeStorageResponsibility insert storage responsibility
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
	if sc.WindowStart <= blockHeight+postponedExecutionBuffer {
		return errEarlyWindow
	}

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
