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
	cm, err := createNewContractManager()
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	cm.blockHeight = 100

	// insert data into active contracts
	amount := 1000
	if testing.Short() {
		amount = 10
	}
	defer cm.activeContracts.Close()
	defer cm.activeContracts.EmptyDB()

	// create and insert expired contracts
	var expiredContracts []contractset.ContractHeader
	for i := 0; i < amount; i++ {
		expiredContract := randomContractGenerator(cm.blockHeight / 2)
		_, err := cm.activeContracts.InsertContract(expiredContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}
		expiredContracts = append(expiredContracts, expiredContract)
	}
	// create renew contracts
	var renewedContracts []contractset.ContractHeader
	for i := 0; i < amount; i++ {
		renewedContract := randomContractGenerator(cm.blockHeight * 2)
		_, err := cm.activeContracts.InsertContract(renewedContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}
		cm.renewedTo[renewedContract.ID] = storageContractIDGenerator()
		renewedContracts = append(renewedContracts, renewedContract)
	}

	// create active contracts
	var activeContracts []contractset.ContractHeader
	for i := 0; i < amount; i++ {
		activeContract := randomContractGenerator(cm.blockHeight * 2)
		_, err := cm.activeContracts.InsertContract(activeContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}
		activeContracts = append(activeContracts, activeContract)
	}

	// maintain expiration
	cm.maintainExpiration()

	// validate the expire set
	for _, contract := range expiredContracts {
		if _, exists := cm.expiredContracts[contract.ID]; !exists {
			t.Fatalf("the expired contract is not in the expired contracts mapping list")
		}
		if _, exists := cm.activeContracts.Acquire(contract.ID); exists {
			t.Fatalf("the expired contract should not be in the active contracts list")
		}
	}

	for _, contract := range renewedContracts {
		if _, exists := cm.expiredContracts[contract.ID]; !exists {
			t.Fatalf("the renewed contract is not in the expired contracts mapping list")
		}
		if _, exists := cm.activeContracts.Acquire(contract.ID); exists {
			t.Fatalf("the renewed contract should not be in the active contracts list")
		}
	}

	for _, contract := range activeContracts {
		if _, exists := cm.expiredContracts[contract.ID]; exists {
			t.Fatalf("the expired contract is supposed not to be contained in the expiredContracts mapping list")
		}
		if _, exists := cm.activeContracts.Acquire(contract.ID); !exists {
			t.Fatalf("the active contract should be stored in the active contracts list")
		}
	}
}

func TestContractManager_RemoveDuplications(t *testing.T) {
	cm, err := createNewContractManager()
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	cm.blockHeight = 100

	// insert data into active contracts
	amount := 1000
	if testing.Short() {
		amount = 100
	}
	defer cm.activeContracts.Close()
	defer cm.activeContracts.EmptyDB()

	// start to generate data
	var enodeIDList []enode.ID
	var oldestContractSet = make(map[enode.ID]contractset.ContractHeader)
	for i := 0; i < amount; i++ {
		// generate old contracts
		enodeID := randomEnodeIDGenerator()
		enodeIDList = append(enodeIDList, enodeID)
		oldContract := randomDuplicateContractGenerator(200, enodeID)

		// insert old contract
		_, err := cm.activeContracts.InsertContract(oldContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}

		// append to oldestContractSet list
		oldestContractSet[enodeID] = oldContract
	}

	// generate / insert old contracts 2
	var olderContractSet = make(map[enode.ID]contractset.ContractHeader)
	for i := 0; i < amount; i++ {
		// generate old contracts
		oldContract := randomDuplicateContractGenerator(300, enodeIDList[i])

		// insert old contract
		_, err := cm.activeContracts.InsertContract(oldContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}

		// append to olderContractSet list
		olderContractSet[enodeIDList[i]] = oldContract
	}

	// generate newest contracts set
	var newestContractSet = make(map[enode.ID]contractset.ContractHeader)
	for i := 0; i < amount; i++ {
		newestContract := randomDuplicateContractGenerator(600, enodeIDList[i])

		// insert newest contract
		_, err := cm.activeContracts.InsertContract(newestContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}

		// append to newest contract set list
		newestContractSet[enodeIDList[i]] = newestContract
	}
	// remove duplications
	cm.removeDuplications()

	// validation for oldestContractSet, olderContractSet, newestContractSet
	// what to validate?
	// 		1. cm.hostToContract[contract.EnodeID] = contract.ID -> should be newestContract
	// 		2. oldestContractSet, olderContractSet should be placed under expiredContracts
	// 		3. oldestContractSet, olderContractSet should be deleted from the contractSet
	// 		4. the renewed from and renewed to order

	// validate the oldestContractSet
	for _, contract := range oldestContractSet {
		if _, exists := cm.expiredContracts[contract.ID]; !exists {
			t.Fatalf("the oldest contract is not in the expired contracts mapping list")
		}
		if _, exists := cm.activeContracts.Acquire(contract.ID); exists {
			t.Fatalf("the oldest contract should not be in the active contracts list")
		}
		if id, _ := cm.hostToContract[contract.EnodeID]; id == contract.ID {
			t.Fatalf("the oldest contract should not be in the host to contract mapping")
		}
	}

	// validate the olderContractSet
	for _, contract := range olderContractSet {
		if _, exists := cm.expiredContracts[contract.ID]; !exists {
			t.Fatalf("the older contract is not in the expired contracts mapping list")
		}
		if _, exists := cm.activeContracts.Acquire(contract.ID); exists {
			t.Fatalf("the older contract should not be in the active contracts list")
		}
		if id, _ := cm.hostToContract[contract.EnodeID]; id == contract.ID {
			t.Fatalf("the older contract should not be in the host to contract mapping")
		}
	}

	// validate the newestContractSet
	for _, contract := range newestContractSet {
		if _, exists := cm.expiredContracts[contract.ID]; exists {
			t.Fatalf("the newest contract should not be in the expired contracts mapping list")
		}
		if _, exists := cm.activeContracts.Acquire(contract.ID); !exists {
			t.Fatalf("the newest contract should be in the active contracts list")
		}
		if id, _ := cm.hostToContract[contract.EnodeID]; id != contract.ID {
			t.Fatalf("the newest contract should be in the host to contract mapping")
		}
	}

	// validate the renewFrom
	for enodeID, contract := range newestContractSet {
		older, exists := cm.renewedFrom[contract.ID]
		if !exists {
			t.Fatalf("the newest contract id should be in the renewFrom list")
		}

		if older != olderContractSet[enodeID].ID {
			t.Fatalf("the newest contract id should map to older contarct")
		}

		oldest, exists := cm.renewedFrom[olderContractSet[enodeID].ID]
		if !exists {
			t.Fatalf("the older contract id should be in the renewFrom list")
		}

		if oldest != oldestContractSet[enodeID].ID {
			t.Fatalf("the older contract id should be mapped to the oldest contract")
		}
	}
}

func TestContractManager_MaintainHostToContractIDMapping(t *testing.T) {

}

/*
 _____  _____  _______      __  _______ ______          ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|        |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__           | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|          |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____         | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|        |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func randomDuplicateContractGenerator(contractStartHeight uint64, id enode.ID) (ch contractset.ContractHeader) {
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: id,
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
		StartHeight:  contractStartHeight,
		DownloadCost: common.RandomBigInt(),
		UploadCost:   common.RandomBigInt(),
		TotalCost:    common.RandomBigInt(),
		StorageCost:  common.RandomBigInt(),
		GasFee:       common.RandomBigInt(),
		ContractFee:  common.RandomBigInt(),
	}

	return
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

func randomContractGenerator(contractEndHeight uint64) (ch contractset.ContractHeader) {
	// generate the private key
	ch = contractset.ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: randomEnodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			NewWindowStart:    contractEndHeight,
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
