package storagehost

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

var handlerMap = map[uint64]func(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error{
	storage.HostSettingMsg:                    handleHostSettingRequest,
	storage.StorageContractCreationMsg:        handleContractCreate,
	storage.StorageContractUploadRequestMsg:   handleUpload,
	storage.StorageContractDownloadRequestMsg: handleDownload,
}

//return the externalConfig for host
func (h *StorageHost) externalConfig() storage.HostExtConfig {
	//Each time you update the configuration, number plus one
	h.revisionNumber++

	// Get the total and remaining disk space
	var totalStorageSpace uint64
	var remainingStorageSpace uint64
	hs := h.StorageManager.AvailableSpace()
	totalStorageSpace = storage.SectorSize * hs.TotalSectors
	remainingStorageSpace = storage.SectorSize * hs.FreeSectors

	acceptingContracts := h.config.AcceptingContracts
	MaxDeposit := h.config.MaxDeposit
	paymentAddress := h.config.PaymentAddress
	if paymentAddress != (common.Address{}) {
		account := accounts.Account{Address: paymentAddress}
		wallet, err := h.ethBackend.AccountManager().Find(account)
		if err != nil {
			h.log.Warn("Failed to find the wallet", "err", err)
			acceptingContracts = false
		}
		//If the wallet is locked, you will not be able to enter the signing phase.
		status, err := wallet.Status()
		if status == "Locked" || err != nil {
			h.log.Warn("Wallet is not unlocked", "err", err)
			acceptingContracts = false
		}

		stateDB, err := h.ethBackend.GetBlockChain().State()
		if err != nil {
			h.log.Warn("Failed to find the stateDB", "err", err)
		} else {
			balance := stateDB.GetBalance(paymentAddress)
			//If the maximum deposit amount exceeds the account balance, set it as the account balance
			if balance.Cmp(&MaxDeposit) < 0 {
				MaxDeposit = *balance
			}
		}

	} else {
		h.log.Error("paymentAddress must be explicitly specified")
	}

	return storage.HostExtConfig{
		AcceptingContracts:     acceptingContracts,
		MaxDownloadBatchSize:   h.config.MaxDownloadBatchSize,
		MaxDuration:            h.config.MaxDuration,
		MaxReviseBatchSize:     h.config.MaxReviseBatchSize,
		SectorSize:             storage.SectorSize,
		WindowSize:             h.config.WindowSize,
		PaymentAddress:         paymentAddress,
		TotalStorage:           totalStorageSpace,
		RemainingStorage:       remainingStorageSpace,
		Deposit:                common.NewBigInt(h.config.Deposit.Int64()),
		MaxDeposit:             common.NewBigInt(MaxDeposit.Int64()),
		BaseRPCPrice:           common.NewBigInt(h.config.MinBaseRPCPrice.Int64()),
		ContractPrice:          common.NewBigInt(h.config.MinContractPrice.Int64()),
		DownloadBandwidthPrice: common.NewBigInt(h.config.MinDownloadBandwidthPrice.Int64()),
		SectorAccessPrice:      common.NewBigInt(h.config.MinSectorAccessPrice.Int64()),
		StoragePrice:           common.NewBigInt(h.config.MinStoragePrice.Int64()),
		UploadBandwidthPrice:   common.NewBigInt(h.config.MinUploadBandwidthPrice.Int64()),
		RevisionNumber:         h.revisionNumber,
		Version:                storage.ConfigVersion,
	}
}
