package contractmanager

import (
	"crypto/ecdsa"
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"math/big"
	"testing"
)

func TestContractManager_ResumeContracts(t *testing.T) {
	// create new contract manager
	cm, err := createNewContractManager()
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	// insert data into active contracts
	amount := 1000
	if testing.Short() {
		amount = 10
	}
	defer cm.activeContracts.Close()
	defer cm.activeContracts.EmptyDB()

	for i := 0; i < amount; i++ {
		_, err := cm.activeContracts.InsertContract(randomCanceledContractGenerator(), randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}
	}

	// call resumeContracts
	if err := cm.resumeContracts(); err != nil {
		t.Fatalf("failed to resume contracts")
	}

	// check the contract status
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		if contract.Status.UploadAbility {
			t.Fatalf("the uploadability should be false, instead got true")
		}

		if contract.Status.RenewAbility {
			t.Fatalf("the renewability should be false, instead got true")
		}

		if contract.Status.Canceled {
			t.Fatalf("the canceled status should be false, instead got true")
		}
	}
}

func TestContractManager_MaintainExpiration(t *testing.T) {

}

func TestContractManager_RemoveDuplications(t *testing.T) {

}

func TestContractManager_MaintainHostToContractIDMapping(t *testing.T) {

}

func randomCanceledContractGenerator() (ch contractset.ContractHeader) {
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: randomEnodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			ParentID:          randomHashGenerator(),
			NewRevisionNumber: 15,
			NewValidProofOutputs: []types.DxcoinCharge{
				{randomAddressGenerator(), big.NewInt(0)},
			},
			UnlockConditions: types.UnlockConditions{
				PublicKeys: []ecdsa.PublicKey{
					{nil, nil, nil},
					{nil, nil, nil},
				},
			},
		},
		PrivateKey:   "12345678910goodMorningOhMyGod",
		StartHeight:  100,
		DownloadCost: common.RandomBigInt(),
		UploadCost:   common.RandomBigInt(),
		TotalCost:    common.RandomBigInt(),
		StorageCost:  common.RandomBigInt(),
		GasFee:       common.RandomBigInt(),
		ContractFee:  common.RandomBigInt(),
		Status: storage.ContractStatus{
			UploadAbility: false,
			RenewAbility:  false,
			Canceled:      true,
		},
	}

	return
}

func randomActiveContractGenerator() (ch contractset.ContractHeader) {
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: randomEnodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			ParentID:          randomHashGenerator(),
			NewRevisionNumber: 15,
			NewValidProofOutputs: []types.DxcoinCharge{
				{randomAddressGenerator(), big.NewInt(0)},
			},
			UnlockConditions: types.UnlockConditions{
				PublicKeys: []ecdsa.PublicKey{
					{nil, nil, nil},
					{nil, nil, nil},
				},
			},
		},
		PrivateKey:   "12345678910",
		StartHeight:  100,
		DownloadCost: common.RandomBigInt(),
		UploadCost:   common.RandomBigInt(),
		TotalCost:    common.RandomBigInt(),
		StorageCost:  common.RandomBigInt(),
		GasFee:       common.RandomBigInt(),
		ContractFee:  common.RandomBigInt(),
	}

	return
}

func randomExpiredContractGenerator() (ch contractset.ContractHeader) {
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: randomEnodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			ParentID:          randomHashGenerator(),
			NewRevisionNumber: 15,
			NewValidProofOutputs: []types.DxcoinCharge{
				{randomAddressGenerator(), big.NewInt(0)},
			},
			UnlockConditions: types.UnlockConditions{
				PublicKeys: []ecdsa.PublicKey{
					{nil, nil, nil},
					{nil, nil, nil},
				},
			},
		},
		PrivateKey:   "12345678910",
		StartHeight:  100,
		DownloadCost: common.RandomBigInt(),
		UploadCost:   common.RandomBigInt(),
		TotalCost:    common.RandomBigInt(),
		StorageCost:  common.RandomBigInt(),
		GasFee:       common.RandomBigInt(),
		ContractFee:  common.RandomBigInt(),
	}

	return
}

func randomRootsGenerator(rootCount int) (roots []common.Hash) {
	for i := 0; i < rootCount; i++ {
		roots = append(roots, randomHashGenerator())
	}
	return
}

func randomEnodeIDGenerator() (id enode.ID) {
	rand.Read(id[:])
	return
}

func randomAddressGenerator() (a common.Address) {
	rand.Read(a[:])
	return
}

func storageContractIDGenerator() (id storage.ContractID) {
	rand.Read(id[:])
	return
}

func randomHashGenerator() (h common.Hash) {
	rand.Read(h[:])
	return
}
