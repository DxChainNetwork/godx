// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
)

func TestHostPrivateAPI_SetConfig(t *testing.T) {
	tests := map[string]struct {
		config map[string]string
		expect storage.HostIntConfig
		err    error
	}{
		"acceptingContracts": {
			map[string]string{"acceptingContracts": "true"},
			storage.HostIntConfig{AcceptingContracts: true},
			nil,
		},
		"maxDownloadBatchSize": {
			map[string]string{"maxDownloadBatchSize": "1kb"},
			storage.HostIntConfig{MaxDownloadBatchSize: 1e3},
			nil,
		},
		"maxDuration": {
			map[string]string{"maxDuration": "1b"},
			storage.HostIntConfig{MaxDuration: uint64(mustParseTime("1b"))},
			nil,
		},
		"maxReviseBatchSize": {
			map[string]string{"maxReviseBatchSize": "1kb"},
			storage.HostIntConfig{MaxReviseBatchSize: uint64(mustParseStorage("1kb"))},
			nil,
		},
		"paymentAddress": {
			map[string]string{"paymentAddress": "0x1"},
			storage.HostIntConfig{},
			errors.New("invalid account"),
		},
		"deposit": {
			map[string]string{"deposit": "1camel"},
			storage.HostIntConfig{Deposit: mustParseCurrency("1camel")},
			nil,
		},
		"depositBudget": {
			map[string]string{"depositBudget": "100camel"},
			storage.HostIntConfig{DepositBudget: mustParseCurrency("100camel")},
			nil,
		},
		"maxDeposit": {
			map[string]string{"maxDeposit": "1000camel"},
			storage.HostIntConfig{MaxDeposit: mustParseCurrency("1000camel")},
			nil,
		},
		"baseRPCPrice": {
			map[string]string{"baseRPCPrice": "1camel"},
			storage.HostIntConfig{BaseRPCPrice: mustParseCurrency("1camel")},
			nil,
		},
		"contractPrice": {
			map[string]string{"contractPrice": "10camel"},
			storage.HostIntConfig{ContractPrice: mustParseCurrency("10camel")},
			nil,
		},
		"downloadBandwidthPrice": {
			map[string]string{"downloadBandwidthPrice": "1camel"},
			storage.HostIntConfig{DownloadBandwidthPrice: mustParseCurrency("1camel")},
			nil,
		},
		"sectorAccessPrice": {
			map[string]string{"sectorAccessPrice": "1camel"},
			storage.HostIntConfig{SectorAccessPrice: mustParseCurrency("1camel")},
			nil,
		},
		"storagePrice": {
			map[string]string{"storagePrice": "1camel"},
			storage.HostIntConfig{StoragePrice: mustParseCurrency("1camel")},
			nil,
		},
		"uploadBandwidthPrice": {
			map[string]string{"uploadBandwidthPrice": "1camel"},
			storage.HostIntConfig{UploadBandwidthPrice: mustParseCurrency("1camel")},
			nil,
		},
		"currency parse error": {
			map[string]string{"baseRPCPrice": "1234", "acceptingContracts": "true"},
			storage.HostIntConfig{},
			errors.New("currency error"),
		},
		"storage parse error": {
			map[string]string{"maxDownloadBatchSize": "1234", "acceptingContracts": "true"},
			storage.HostIntConfig{},
			errors.New("storage error"),
		},
		"duration parse error": {
			map[string]string{"windowSize": "1234", "acceptingContracts": "true"},
			storage.HostIntConfig{},
			errors.New("duration error"),
		},
		"unknown keyword": {
			map[string]string{"unknown keyword": "1234", "acceptingContracts": "true"},
			storage.HostIntConfig{},
			errors.New("unknown keyword"),
		},
		"overall test": {
			map[string]string{
				"acceptingContracts":     "true",
				"maxDownloadBatchSize":   "10mb",
				"maxDuration":            "1d",
				"maxReviseBatchSize":     "10mb",
				"deposit":                "1000camel",
				"depositBudget":          "100dx",
				"maxDeposit":             "10000camel",
				"baseRPCPrice":           "500000camel",
				"contractPrice":          "50gcamel",
				"downloadBandwidthPrice": "100000camel",
				"sectorAccessPrice":      "10000camel",
				"storagePrice":           "10000camel",
				"uploadBandwidthPrice":   "10000camel",
			},
			storage.HostIntConfig{
				AcceptingContracts:     true,
				MaxDownloadBatchSize:   uint64(mustParseStorage("10mb")),
				MaxDuration:            uint64(mustParseTime("1d")),
				MaxReviseBatchSize:     uint64(mustParseStorage("10mb")),
				Deposit:                mustParseCurrency("1000camel"),
				DepositBudget:          mustParseCurrency("100dx"),
				MaxDeposit:             mustParseCurrency("10000camel"),
				BaseRPCPrice:           mustParseCurrency("500000camel"),
				ContractPrice:          mustParseCurrency("50gcamel"),
				DownloadBandwidthPrice: mustParseCurrency("100000camel"),
				SectorAccessPrice:      mustParseCurrency("10000camel"),
				StoragePrice:           mustParseCurrency("10000camel"),
				UploadBandwidthPrice:   mustParseCurrency("10000camel"),
			},
			nil,
		},
	}
	dir := tempDir(t.Name())
	for key, test := range tests {
		// Create a new storage host api and apply the test config
		h := NewHostPrivateAPI(&StorageHost{persistDir: dir})
		_, err := h.SetConfig(test.config)
		// errors should be as expected
		if (err == nil) != (test.err == nil) {
			t.Fatalf("Test %v not expected error. Expect %v, Got %v", key, test.err, err)
		}
		// After set, the config should be test.config
		if !reflect.DeepEqual(h.storageHost.config, test.expect) {
			t.Fatalf("Test %v config not expected.\nGot %vExpect %v", key,
				dumper.Sdump(h.storageHost.config), dumper.Sdump(test.expect))
		}
		if err != nil {
			continue
		}
		// Check whdx the config is stored
		hostSettingFile := filepath.Join(dir, HostSettingFile)
		persist := new(persistence)
		if err := common.LoadDxJSON(storageHostMeta, hostSettingFile, persist); err != nil {
			t.Fatalf("Test [%v] error: %v", key, err)
		}
		if !reflect.DeepEqual(persist.Config, test.expect) {
			t.Fatalf("Test %v config not expected.\nGot %vExpect %v", key,
				dumper.Sdump(persist.Config), dumper.Sdump(test.expect))
		}
		if err = os.Remove(hostSettingFile); err != nil {
			t.Fatal(err)
		}
	}
}

// mustParseCurrency parse the string to currency. If an error happens, panic.
func mustParseCurrency(str string) common.BigInt {
	parsed, err := unit.ParseCurrency(str)
	if err != nil {
		panic(err)
	}
	return parsed
}

// mustParseTime parse the string to duration. If an error happens, panic.
func mustParseTime(str string) uint64 {
	parsed, err := unit.ParseTime(str)
	if err != nil {
		panic(err)
	}
	return parsed
}

// mustParseStorage parse the string to storage. If an error happens, panic.
func mustParseStorage(str string) uint64 {
	parsed, err := unit.ParseStorage(str)
	if err != nil {
		panic(err)
	}
	return parsed
}
