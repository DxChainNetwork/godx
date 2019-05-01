package storagehost

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"math/big"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"
)

// Try the debug setter function
func TestHostDeBugAPI_HostSetter(t *testing.T) {
	// clear the saved data for testing
	remmoveFolders("./testdata/", t)
	defer remmoveFolders("./testdata/", t)

	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// create an debug api
	api := NewHostDebugAPI(host)

	// try using the setter do some modification
	api.SetRevisionNumber(5000)
	api.SetBroadCast(true)

	// directly load the persistence from file, in order to check if the setting have been save to
	// file at mean time
	persist := &persistence{}
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join(host.persistDir, HostSettingFile), persist); err != nil {
		t.Errorf(err.Error())
	}

	// if the persistence loaded from file does not equal to the expected
	if !persist.BroadCast || persist.RevisionNumber != 5000 {
		t.Errorf(err.Error())
	}

	// close the host
	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}
}

// testing the load of setting as an object
func TestHostDeBugAPI_LoadSetting(t *testing.T) {
	// clear the saved data for testing
	remmoveFolders("./testdata/", t)
	defer remmoveFolders("./testdata/", t)

	// try to do the first data json renew, use the default value
	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// create an debug api
	api := NewHostDebugAPI(host)

	internalSetting := StorageHostIntSetting{
		AcceptingContracts:   rand.Float32() < 0.5,
		MaxDownloadBatchSize: uint64(rand.Intn(RANDRANGE)),
		Deposit:              *big.NewInt(int64(rand.Intn(RANDRANGE))),
		MinBaseRPCPrice:      *big.NewInt(int64(rand.Intn(RANDRANGE))),
	}

	financial := HostFinancialMetrics{
		ContractCount:        uint64(rand.Intn(RANDRANGE)),
		ContractCompensation: *big.NewInt(int64(rand.Intn(RANDRANGE))),
		LockedStorageDeposit: *big.NewInt(int64(rand.Intn(RANDRANGE))),
		LostRevenue:          *big.NewInt(int64(rand.Intn(RANDRANGE))),
	}

	api.LoadInternalSetting(internalSetting)
	api.LoadFinancialMetrics(financial)

	// extract the internal setting from the host
	hostintSetting := host.InternalSetting()

	// extract the financial metrics from host
	hostfinancial := host.FinancialMetrics()

	if !reflect.DeepEqual(internalSetting, hostintSetting) {
		t.Errorf("the input string of internal setting fail loaded to host")
	}

	if !reflect.DeepEqual(financial, hostfinancial) {
		t.Errorf("the input string of financial metrix fail loaded to host")
	}

	// close the host
	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestHostDeBugAPI_LoadSettingStream(t *testing.T) {
	// clear the saved data for testing
	remmoveFolders("./testdata/", t)
	defer remmoveFolders("./testdata/", t)

	// try to do the first data json renew, use the default value
	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// create an debug api
	api := NewHostDebugAPI(host)

	// string for loading
	dataInternal := "{" +
		"\"acceptingcontracts\" : true, " +
		"\"maxduration\":20, " +
		"\"deposit\" :520, " +
		"\"depositbudget\": 5200, " +
		"\"minuploadbandwidthprice\": 1314520" +
		"}"

	// string for loading
	dataFinancial := "{" +
		"\"contractcount\" : 1000, " +
		"\"contractcompensation\":2000, " +
		"\"lockedstoragedeposit\" :5200, " +
		"\"lostrevenue\": 7000, " +
		"\"loststoragedeposit\": 8000" +
		"}"

	api.LoadInternalSettingStream(dataInternal)
	api.LoadFinancialMetricsStream(dataFinancial)

	// extract the internal setting from the host
	internalSetting := host.InternalSetting()

	// extract the financial metrics from host
	financial := host.FinancialMetrics()

	if !isEqualsInternalSetting(internalSetting) {
		t.Errorf("the input string of internal setting fail loaded to host")
	}

	if !isEqualsFinancialMetirx(financial) {
		t.Errorf("the input string of financial metrix fail loaded to host")
	}

	// directly load the persistence from file, in order to check if the setting have been save to
	// file at mean time
	persist := &persistence{}
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join(host.persistDir, HostSettingFile), persist); err != nil {
		t.Errorf(err.Error())
	}

	if !isEqualsInternalSetting(internalSetting) {
		t.Errorf("the input string of internal setting fail loaded to host")
	}

	if !isEqualsFinancialMetirx(financial) {
		t.Errorf("the input string of financial metrix fail loaded to host")
	}

	// close the host
	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}
}

// Currently hard code the test case
func isEqualsInternalSetting(internalSetting StorageHostIntSetting) bool {
	// make sure these parts are equal
	if !internalSetting.AcceptingContracts ||
		internalSetting.MaxDuration != 20 ||
		!reflect.DeepEqual(internalSetting.Deposit, *big.NewInt(520)) ||
		!reflect.DeepEqual(internalSetting.DepositBudget, *big.NewInt(5200)) ||
		!reflect.DeepEqual(internalSetting.MinUploadBandwidthPrice, *big.NewInt(1314520)) {
		fmt.Println(internalSetting.AcceptingContracts)
		return false
	}
	return true
}

// Currently hard code the test case
func isEqualsFinancialMetirx(financial HostFinancialMetrics) bool {
	// make sure these parts are equal
	if financial.ContractCount != 1000 ||
		!reflect.DeepEqual(financial.ContractCompensation, *big.NewInt(2000)) ||
		!reflect.DeepEqual(financial.LockedStorageDeposit, *big.NewInt(5200)) ||
		!reflect.DeepEqual(financial.LostRevenue, *big.NewInt(7000)) ||
		!reflect.DeepEqual(financial.LostStorageDeposit, *big.NewInt(8000)) {
		return false
	}
	return true
}
