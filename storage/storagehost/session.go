package storagehost

import (
	"errors"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"time"
)

// handlerMap is the map for p2p handler
var handlerMap = map[uint64]func(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error{
	storage.HostSettingMsg:                    handleHostSettingRequest,
	storage.StorageContractCreationMsg:        handleContractCreate,
	storage.StorageContractUploadRequestMsg:   handleUpload,
	storage.StorageContractDownloadRequestMsg: handleDownload,
}

// HandleSession is the function to be called from Ethereum handle method.
func (h *StorageHost) HandleSession(s *storage.Session) error {
	if s == nil {
		return errors.New("host session is nil")
	}

	if s.IsBusy() {
		// wait for 3 seconds and retry
		<-time.After(3 * time.Second)
		h.log.Warn("session is busy, we will retry later")
		return nil
	}

	msg, err := s.ReadMsg()
	if err != nil {
		h.log.Error("read message error", "err", err.Error())
		return err
	}

	if handler, ok := handlerMap[msg.Code]; ok {
		return handler(h, s, msg)
	} else {
		h.log.Error("failed to get handler", "message", msg.Code)
		return errors.New("failed to get handler")
	}
}

// handleHostSettingRequest is the function to be called when calling HostSettingMsg
func handleHostSettingRequest(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {
	s.SetBusy()
	defer s.ResetBusy()

	s.SetDeadLine(storage.HostSettingTime)

	settings := h.externalConfig()
	if err := s.SendHostExtSettingsResponse(settings); err != nil {
		h.log.Error("SendHostExtSettingResponse Error", "err", err)
		return errors.New("host setting request done")
	}

	h.log.Error("successfully sent the host external setting response")

	return nil
}

//return the externalConfig for host
func (h *StorageHost) externalConfig() storage.HostExtConfig {
	h.lock.Lock()
	defer h.lock.Unlock()

	// Get the total and remaining disk space
	var totalStorageSpace uint64
	var remainingStorageSpace uint64
	hs := h.StorageManager.AvailableSpace()
	totalStorageSpace = storage.SectorSize * hs.TotalSectors
	remainingStorageSpace = storage.SectorSize * hs.FreeSectors

	acceptingContracts := h.config.AcceptingContracts
	MaxDeposit := h.config.MaxDeposit
	paymentAddress := h.config.PaymentAddress

	if paymentAddress == (common.Address{}) {
		acceptingContracts = false
		return storage.HostExtConfig{AcceptingContracts: false}
	}

	account := accounts.Account{Address: paymentAddress}
	wallet, err := h.am.Find(account)
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
		if balance.Cmp(MaxDeposit.BigIntPtr()) < 0 {
			MaxDeposit = common.PtrBigInt(balance)
		}
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
		Deposit:                h.config.Deposit,
		MaxDeposit:             MaxDeposit,
		BaseRPCPrice:           h.config.MinBaseRPCPrice,
		ContractPrice:          h.config.MinContractPrice,
		DownloadBandwidthPrice: h.config.MinDownloadBandwidthPrice,
		SectorAccessPrice:      h.config.MinSectorAccessPrice,
		StoragePrice:           h.config.MinStoragePrice,
		UploadBandwidthPrice:   h.config.MinUploadBandwidthPrice,
		Version:                storage.ConfigVersion,
	}
}
