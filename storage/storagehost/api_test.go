// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
	"path/filepath"
	"reflect"
	"testing"
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
		"windowSize": {
			map[string]string{"windowSize": "1b"},
			storage.HostIntConfig{WindowSize: uint64(mustParseTime("1b"))},
			nil,
		},
		"paymentAddress": {
			map[string]string{"paymentAddress": "0x1"},
			storage.HostIntConfig{},
			errors.New("invalid account"),
		},
		"deposit": {
			map[string]string{"deposit": "1wei"},
			storage.HostIntConfig{Deposit: mustParseCurrency("1wei")},
			nil,
		},
		"depositBudget": {
			map[string]string{"depositBudget": "100wei"},
			storage.HostIntConfig{DepositBudget: mustParseCurrency("100wei")},
			nil,
		},
		"maxDeposit": {
			map[string]string{"maxDeposit": "1000wei"},
			storage.HostIntConfig{MaxDeposit: mustParseCurrency("1000wei")},
			nil,
		},
		"baseRPCPrice": {
			map[string]string{"baseRPCPrice": "1wei"},
			storage.HostIntConfig{BaseRPCPrice: mustParseCurrency("1wei")},
			nil,
		},
		"contractPrice": {
			map[string]string{"contractPrice": "10wei"},
			storage.HostIntConfig{ContractPrice: mustParseCurrency("10wei")},
			nil,
		},
		"downloadBandwidthPrice": {
			map[string]string{"downloadBandwidthPrice": "1wei"},
			storage.HostIntConfig{DownloadBandwidthPrice: mustParseCurrency("1wei")},
			nil,
		},
		"sectorAccessPrice": {
			map[string]string{"sectorAccessPrice": "1wei"},
			storage.HostIntConfig{SectorAccessPrice: mustParseCurrency("1wei")},
			nil,
		},
		"storagePrice": {
			map[string]string{"storagePrice": "1wei"},
			storage.HostIntConfig{StoragePrice: mustParseCurrency("1wei")},
			nil,
		},
		"uploadBandwidthPrice": {
			map[string]string{"uploadBandwidthPrice": "1wei"},
			storage.HostIntConfig{UploadBandwidthPrice: mustParseCurrency("1wei")},
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
				"windowSize":             "12h",
				"deposit":                "1000wei",
				"depositBudget":          "100ether",
				"maxDeposit":             "10000wei",
				"baseRPCPrice":           "500000wei",
				"contractPrice":          "50microether",
				"downloadBandwidthPrice": "100000wei",
				"sectorAccessPrice":      "10000wei",
				"storagePrice":           "10000wei",
				"uploadBandwidthPrice":   "10000wei",
			},
			storage.HostIntConfig{
				AcceptingContracts:     true,
				MaxDownloadBatchSize:   uint64(mustParseStorage("10mb")),
				MaxDuration:            uint64(mustParseTime("1d")),
				MaxReviseBatchSize:     uint64(mustParseStorage("10mb")),
				WindowSize:             uint64(mustParseTime("12h")),
				Deposit:                mustParseCurrency("1000wei"),
				DepositBudget:          mustParseCurrency("100ether"),
				MaxDeposit:             mustParseCurrency("10000wei"),
				BaseRPCPrice:           mustParseCurrency("500000wei"),
				ContractPrice:          mustParseCurrency("50microether"),
				DownloadBandwidthPrice: mustParseCurrency("100000wei"),
				SectorAccessPrice:      mustParseCurrency("10000wei"),
				StoragePrice:           mustParseCurrency("10000wei"),
				UploadBandwidthPrice:   mustParseCurrency("10000wei"),
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
		// Check whether the config is stored
		hostSettingFile := filepath.Join(dir, HostSettingFile)
		persist := new(persistence)
		if err := common.LoadDxJSON(storageHostMeta, hostSettingFile, persist); err != nil {
			t.Fatalf("Test [%v] error: %v", key, err)
		}
		if !reflect.DeepEqual(persist.Config, test.expect) {
			t.Fatalf("Test %v config not expected.\nGot %vExpect %v", key,
				dumper.Sdump(persist.Config), dumper.Sdump(test.expect))
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

// mustParseSpeed parse the string to speed. If an error happens, panic.
func mustParseSpeed(str string) int64 {
	parsed, err := unit.ParseSpeed(str)
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
