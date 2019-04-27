package storagehost

import (
	"github.com/davecgh/go-spew/spew"
	"math/big"
)

func (h *StorageHost) GetPersistDir() string{
	return h.persistDir
}

func (h *StorageHost) PrintIntSetting() {
	persist := h.extractPersistence()
	spew.Dump(persist)
}


func (h *StorageHost) SetBroadCast(b bool){
	h.broadcast = b
}

func (h *StorageHost) SetRevisionNumber(num int){
	h.revisionNumber = uint64(num)
}

func (h *StorageHost) SetAcceptingContract(b bool){
	h.settings.AcceptingContracts = b
}

func (h *StorageHost) SetDeposit(num int){
	h.settings.Deposit = *big.NewInt(int64(num))
}