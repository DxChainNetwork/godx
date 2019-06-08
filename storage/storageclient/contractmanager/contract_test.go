package contractmanager

import (
	"crypto/ecdsa"
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"github.com/Pallinder/go-randomdata"
	"math/big"
	"testing"
	"time"
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
		amount = 10
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

	// call maintain expiration to update the activeContract field
	cm.maintainExpiration()

	// call maintainHostToContractIDMapping to update the mapping
	cm.maintainHostToContractIDMapping()

	// lastly, do validation, the expiredContracts should not be in the contract id mapping
	for _, contract := range expiredContracts {
		if _, exists := cm.hostToContract[contract.EnodeID]; exists {
			t.Fatalf("the expired contract is not supposed to be contained in the hostToContract mapping")
		}
	}

	// the activeContracts should still be in the contract id mapping
	for _, contract := range activeContracts {
		if _, exists := cm.hostToContract[contract.EnodeID]; !exists {
			t.Fatalf("the active contract should be contained in the hostToContract mapping")
		}
	}
}

func TestContractManager_removeHostWithDuplicateNetworkAddress(t *testing.T) {
	// create and initialize new contractManager
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

	// generate earlier IP changed contract and hosts
	var ipList []string
	var contracts []contractset.ContractHeader
	for i := 0; i < amount; i++ {
		enodeID := randomEnodeIDGenerator()
		ip := randomdata.IpV4Address()

		// insert storage host first
		if err := insertHost(cm, ip, enodeID, time.Now()); err != nil {
			t.Fatalf("failed to insert storage host: %s", err.Error())
		}

		// insert the contract for the storage host
		contract := randomContractWithEnodeID(enodeID)
		if _, err := cm.activeContracts.InsertContract(contract, randomRootsGenerator(10)); err != nil {
			t.Fatalf("failed to insert the contract: %s", err.Error())
		}

		// append the ip and contract to the list
		ipList = append(ipList, ip)
		contracts = append(contracts, contract)
	}

	// generate filtered contracts
	var filteredContracts []contractset.ContractHeader
	for i := 0; i < amount; i++ {
		enodeID := randomEnodeIDGenerator()

		// insert storage host first
		if err := insertHost(cm, ipList[i], enodeID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("failed to insert filtered storage host: %s", err.Error())
		}

		// insert the storage contract for the storage hsot
		contract := randomContractWithEnodeID(enodeID)
		if _, err := cm.activeContracts.InsertContract(contract, randomRootsGenerator(10)); err != nil {
			t.Fatalf("failed to insert filtered contract: %s", err.Error())
		}

		filteredContracts = append(filteredContracts, contract)
	}

	// before enable ip violation check, no contract should be canceled
	cm.removeHostWithDuplicateNetworkAddress()
	if canceled := checkCanceled(cm, contracts); canceled {
		t.Fatalf("unfiltered contract should not be canceled")
	}

	if canceled := checkCanceled(cm, filteredContracts); canceled {
		t.Fatalf("filtered contract should not be canceled because the IP check is not enabled")
	}

	// enable the ip violation check
	cm.hostManager.SetIPViolationCheck(true)
	cm.removeHostWithDuplicateNetworkAddress()
	if canceled := checkCanceled(cm, contracts); canceled {
		t.Fatalf("unfiltered contract should not be canceled")
	}

	if canceled := checkCanceled(cm, filteredContracts); !canceled {
		t.Fatalf("filtered contract should be canceled")
	}
}

func TestContractManager_markNewlyFormedContractStats(t *testing.T) {
	// create and initialize new contractManager
	cm, err := createNewContractManager()
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	//insert data into active contracts
	amount := 1000
	if testing.Short() {
		amount = 10
	}
	defer cm.activeContracts.Close()
	defer cm.activeContracts.EmptyDB()

	// create and insert the canceled contract
	var canceledContractIDs []storage.ContractID
	for i := 0; i < amount; i++ {
		canceledContract := randomCanceledContractGenerator()
		_, err := cm.activeContracts.InsertContract(canceledContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}

		canceledContractIDs = append(canceledContractIDs, canceledContract.ID)
	}

	// mark all as newly formed contract status
	for _, id := range canceledContractIDs {
		if err := cm.markNewlyFormedContractStats(id); err != nil {
			t.Fatalf("failed to mark the newly formed contract status: %s", err.Error())
		}
	}

	// contract status validation
	for _, id := range canceledContractIDs {
		meta, exists := cm.activeContracts.RetrieveContractMetaData(id)
		if !exists {
			t.Fatalf("the contract id provided is not in the active contract list")
		}

		if meta.Status.Canceled || !meta.Status.UploadAbility || !meta.Status.RenewAbility {
			t.Fatalf("the contract status has not been changed to newly formed contract status")
		}

	}
}

func TestContractManager_checkContractStatus(t *testing.T) {
	// create new contract manager
	cm, err := createNewContractManager()
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	cm.blockHeight = 100

	// insert data into active contracts
	amount := 1000
	if testing.Short() {
		amount = 1
	}
	defer cm.activeContracts.Close()
	defer cm.activeContracts.EmptyDB()

	// base line for the storage host evaluation
	baseline := common.NewBigIntFloat64(10)

	// insert contract, host does not exist
	var hostNotExistsContract []storage.ContractMetaData
	if hostNotExistsContract, err = insertHostDoesNotExistContract(cm, amount); err != nil {
		t.Fatalf("failed to insert host does not exist contract: %s", err.Error())
	}

	for _, contract := range hostNotExistsContract {
		meta, _ := cm.activeContracts.RetrieveContractMetaData(contract.ID)
		stats := cm.checkContractStatus(meta, baseline)
		if stats.UploadAbility || stats.RenewAbility || stats.Canceled {
			t.Fatalf("host not exist contract was still able to upload or renew contract")
		}
	}

	//insert contract, storage host evaluation lower than the base line
	var lowEvalContracts []storage.ContractMetaData

	if lowEvalContracts, err = insertLowEvalContract(cm, amount); err != nil {
		t.Fatalf("failed to insert contract with lower host evaluation: %s", err.Error())
	}

	for _, contract := range lowEvalContracts {
		stats := cm.checkContractStatus(contract, baseline)
		if stats.UploadAbility || stats.RenewAbility || stats.Canceled {
			t.Fatalf("lower evaluation storage host contract was still able to upload or renew contract")
		}
	}

	// insert contract, storage host evaluation higher than the base line
	var highEvalContracts []storage.ContractMetaData
	if highEvalContracts, err = insertHighEvalContract(cm, amount); err != nil {
		t.Fatalf("failed to insert contract with high host evaluation: %s", err.Error())
	}

	for _, contract := range highEvalContracts {
		meta, _ := cm.activeContracts.RetrieveContractMetaData(contract.ID)
		stats := cm.checkContractStatus(meta, baseline)
		if stats.UploadAbility || !stats.RenewAbility || stats.Canceled {
			t.Fatalf("contract with high host evaluation should be aable to renew the contract")
		}
	}
}

func TestContractManager_MaintainContractStatus(t *testing.T) {
	// create new contract manager
	cm, err := createNewContractManager()
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	cm.blockHeight = 100

	// insert data into active contracts
	amount := 1000
	if testing.Short() {
		amount = 1
	}
	defer cm.activeContracts.Close()
	defer cm.activeContracts.EmptyDB()

	// insert contract, host does not exist
	var hostNotExistsContract []storage.ContractMetaData
	if hostNotExistsContract, err = insertHostDoesNotExistContract(cm, amount); err != nil {
		t.Fatalf("failed to insert host does not exist contract: %s", err.Error())
	}

	//insert contract, storage host evaluation lower than the base line
	var lowEvalContracts []storage.ContractMetaData

	if lowEvalContracts, err = insertLowEvalContract(cm, amount); err != nil {
		t.Fatalf("failed to insert contract with lower host evaluation: %s", err.Error())
	}

	// insert contract, storage host evaluation higher than the base line
	var highEvalContracts []storage.ContractMetaData
	if highEvalContracts, err = insertHighEvalContract(cm, amount); err != nil {
		t.Fatalf("failed to insert contract with high host evaluation: %s", err.Error())
	}

	// maintain contract status
	if err := cm.maintainContractStatus(); err != nil {
		t.Fatalf("failed to maintain the contract status")
	}

	// validation for hostNotExistsContract, lowEvalContracts, highEvalContracts

	for _, contract := range hostNotExistsContract {
		retrieved, exists := cm.activeContracts.RetrieveContractMetaData(contract.ID)
		if !exists {
			t.Fatalf("failed to retrieve the hostNotExistsContract")
		}
		if retrieved.Status.UploadAbility || retrieved.Status.RenewAbility || retrieved.Status.Canceled {
			t.Fatalf("the hostNotExistsContract should not be able to upload nor renew, and it should not be cancled")
		}
	}

	for _, contract := range lowEvalContracts {
		retrieved, exists := cm.activeContracts.RetrieveContractMetaData(contract.ID)
		if !exists {
			t.Fatalf("failed to retrieve the lowEvalContracts")
		}
		if retrieved.Status.UploadAbility || retrieved.Status.RenewAbility || retrieved.Status.Canceled {
			t.Fatalf("the lowEvalContracts should not be able to upload nor renew, and it should not be cancled")
		}

	}

	for _, contract := range highEvalContracts {
		retrieved, exists := cm.activeContracts.RetrieveContractMetaData(contract.ID)
		if !exists {
			t.Fatalf("failed to retrieve the highEvalContracts")
		}
		if retrieved.Status.UploadAbility || !retrieved.Status.RenewAbility || retrieved.Status.Canceled {
			t.Fatalf("the highEvalContracts should still be able to renew")
		}
	}

}

/*
 _____  _____  _______      __  _______ ______          ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|        |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__           | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|          |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____         | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|        |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func insertHostDoesNotExistContract(cm *ContractManager, amount int) (hostNotExistsContract []storage.ContractMetaData, err error) {
	var meta storage.ContractMetaData
	for i := 0; i < amount; i++ {
		contract := randomContractGenerator(cm.blockHeight / 2)
		meta, err = cm.activeContracts.InsertContract(contract, randomRootsGenerator(10))
		if err != nil {
			return
		}

		hostNotExistsContract = append(hostNotExistsContract, meta)
	}
	return
}

func insertHighEvalContract(cm *ContractManager, amount int) (highEvalContracts []storage.ContractMetaData, err error) {
	var meta storage.ContractMetaData
	for i := 0; i < amount; i++ {
		enodeID := randomEnodeIDGenerator()
		contract := randomContractWithEnodeID(enodeID)

		// insert storage host first
		if err = insertHostHighEval(cm, enodeID); err != nil {
			return
		}

		// then, insert the contract
		meta, err = cm.activeContracts.InsertContract(contract, randomRootsGenerator(10))
		if err != nil {
			return
		}

		// add it to the return value
		highEvalContracts = append(highEvalContracts, meta)
	}

	return
}

func insertLowEvalContract(cm *ContractManager, amount int) (lowEvalContracts []storage.ContractMetaData, err error) {
	for i := 0; i < amount; i++ {
		enodeID := randomEnodeIDGenerator()
		contract := randomContractWithEnodeID(enodeID)

		// insert the storage host first
		if err = insertHostLowEval(cm, enodeID); err != nil {
			return
		}

		// then, insert the contract
		var meta storage.ContractMetaData
		meta, err = cm.activeContracts.InsertContract(contract, randomRootsGenerator(10))
		if err != nil {
			return
		}

		// add it to the return value
		lowEvalContracts = append(lowEvalContracts, meta)
	}

	return
}

func checkCanceled(cm *ContractManager, contracts []contractset.ContractHeader) (canceled bool) {
	for _, contract := range contracts {
		meta, _ := cm.activeContracts.RetrieveContractMetaData(contract.ID)
		if meta.Status.Canceled && !meta.Status.RenewAbility && !meta.Status.UploadAbility {
			return true
		}
	}
	return false
}

func randomContractWithEnodeID(id enode.ID) (ch contractset.ContractHeader) {
	ch = randomContractGenerator(100)
	ch.EnodeID = id
	return
}

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

func insertHost(cm *ContractManager, ip string, id enode.ID, ipChange time.Time) (err error) {
	testDebug := storagehostmanager.NewPublicStorageClientDebugAPI(cm.hostManager)
	return testDebug.InsertHostInfoIPTime(1, id, ip, ipChange)
}

func insertHostHighEval(cm *ContractManager, id enode.ID) (err error) {
	testDebug := storagehostmanager.NewPublicStorageClientDebugAPI(cm.hostManager)
	return testDebug.InsertHostInfoHighEval(id)
}

func insertHostLowEval(cm *ContractManager, id enode.ID) (err error) {
	testDebug := storagehostmanager.NewPublicStorageClientDebugAPI(cm.hostManager)
	return testDebug.InsertHostInfoLowEval(id)
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
		Status: storage.ContractStatus{
			UploadAbility: true,
			RenewAbility:  true,
			Canceled:      false,
		},
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
