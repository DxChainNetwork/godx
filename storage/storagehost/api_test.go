package storagehost

import (
	"math/big"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// Try the debug setter function
func TestHostDeBugAPI_HostSetter(t *testing.T) {
	// clear the saved data for testing
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	host, err := New("./testdata/")
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
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	// try to do the first data json renew, use the default value
	host, err := New("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// create an debug api
	api := NewHostDebugAPI(host)

	internalSetting := storage.HostIntConfig{
		AcceptingContracts:   rand.Float32() < 0.5,
		MaxDownloadBatchSize: uint64(rand.Intn(RANDRANGE)),
		Deposit:              *big.NewInt(int64(rand.Intn(RANDRANGE))),
		MinBaseRPCPrice:      *big.NewInt(int64(rand.Intn(RANDRANGE))),
	}

	financial := HostFinancialMetrics{
		ContractCount:        uint64(rand.Intn(RANDRANGE)),
		ContractCompensation: common.NewBigInt(RANDRANGE),
		LockedStorageDeposit: common.NewBigInt(RANDRANGE),
		LostRevenue:          common.NewBigInt(RANDRANGE),
	}

	api.LoadIntConfig(internalSetting)
	api.LoadFinancialMetrics(financial)

	// extract the internal setting from the host
	hostintSetting := host.InternalConfig()

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
