// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
	"math/rand"
	"testing"
	"time"
)

func TestStorageClient_ParseClientSetting(t *testing.T) {

	for i := 0; i < 100; i++ {
		// randomly generate some settings
		settings, err := randomSettings()
		if err != nil {
			t.Fatalf("failed to get random settings: %s", err.Error())
		}

		// randomly generate client settings
		prevSetting := randomClientSettingsGenerator()

		// parse the setting, and start the validation
		clientSetting, err := parseClientSetting(settings, prevSetting)
		if err != nil {
			t.Fatalf("failed to parse the client setting: %s", err.Error())
		}

		for _, key := range keys {
			_, exists := settings[key]
			if !exists {
				// meaning the setting should be same as previous setting
				match, err := clientSettingValidation(key, clientSetting, prevSetting)
				if err != nil {
					t.Fatalf("validation failed: %s", err.Error())
				}

				if !match {
					t.Errorf("with the key: %s, two values are not same", key)
				}
			}
		}
	}
}

func TestParseStorageHosts(t *testing.T) {
	var tables = []struct {
		hosts  string
		parsed uint64
	}{
		{"1231231", 1231231},
		{"34123431324", 34123431324},
		{"023123131", 23123131},
	}

	for _, table := range tables {
		result, err := parseStorageHosts(table.hosts)
		if err != nil {
			t.Fatalf("failed to parse the storage hosts: %s", err.Error())
		}

		if result != table.parsed {
			t.Errorf("by using %s as input, expected parsed value %v, got %v",
				table.hosts, table.parsed, result)
		}
	}
}

func TestParseExpectedUpload(t *testing.T) {
	var tables = []struct {
		dataSize string
		parsed   uint64
	}{
		{"256b", 256 / storage.BlocksPerMonth},
		{"1000 b", 1000 / storage.BlocksPerMonth},
		{"431 KB", 431 * 1e3 / storage.BlocksPerMonth},
		{"486 mB", 486 * 1e6 / storage.BlocksPerMonth},
		{"1025 gb", 1025 * 1e9 / storage.BlocksPerMonth},
		{"3 tB", 3 * 1e12 / storage.BlocksPerMonth},
		{"431 mib", 431 * 1 << 20 / storage.BlocksPerMonth},
		{"572 tib", 572 * 1 << 40 / storage.BlocksPerMonth},
	}

	for _, table := range tables {
		result, err := parseExpectedUpload(table.dataSize)
		if err != nil {
			t.Fatalf("error parsing the expected upload: %s", err.Error())
		}

		if result != table.parsed {
			t.Errorf("error parsing: expected parsed upload size %+v, got %+v",
				table.parsed, result)
		}
	}
}

func TestParseExpectedRedundancy(t *testing.T) {
	var tables = []struct {
		redundancy string
		parsed     float64
		err        bool
	}{
		{"3.5", 3.5, false},
		{"4.0", 4.0, false},
		{"abcdefg", 0, true},
	}

	for _, table := range tables {
		result, err := parseExpectedRedundancy(table.redundancy)
		if err != nil && table.err {
			continue
		} else if err != nil {
			t.Fatalf("error parsing the expected redundancy: %s", err.Error())
		}

		if result != table.parsed {
			t.Errorf("error parsing: expected parsed redundancy %+v, got %+v",
				table.parsed, result)
		}
	}
}

func randomSettings() (settings map[string]string, err error) {
	var keys map[string]string

	if keys, err = randomKeys(); err != nil {
		return
	}
	return randomValue(keys)

}

func randomKeys() (selectedKeys map[string]string, err error) {
	selectedKeys = make(map[string]string)
	rand.Seed(time.Now().UnixNano())

	// get random amount of keys
	randomAmount := rand.Intn(len(keys))
	for i := 0; i < randomAmount; i++ {
		// randomly selected a key from the keys list
		randomIndex := rand.Intn(len(keys))
		if randomIndex < 0 || randomIndex >= len(keys) {
			err = fmt.Errorf("wrong random index: %v", randomIndex)
			return
		}
		selectedKey := keys[randomIndex]

		// check if the key already existed
		if _, exists := selectedKeys[selectedKey]; exists {
			i--
			continue
		}

		// if not, assign empty value to it
		selectedKeys[selectedKey] = ""
	}
	return
}

func randomValue(selectedKeys map[string]string) (settings map[string]string, err error) {
	settings = make(map[string]string)

	var value, granularity interface{}

	rand.Seed(time.Now().UnixNano())

	for key, _ := range selectedKeys {
		switch {
		case key == "fund":
			value = common.RandomBigInt()
			granularity = unit.CurrencyUnit[rand.Intn(len(unit.CurrencyUnit))]
			break
		case key == "period" || key == "renew":
			value = rand.Uint64()
			granularity = unit.TimeUnit[rand.Intn(len(unit.TimeUnit))]
			break
		case key == "storage" || key == "upload" || key == "download":
			value = rand.Uint64()
			granularity = unit.DataSizeUnit[rand.Intn(len(unit.DataSizeUnit))]
			break
		case key == "redundancy":
			value = rand.Float64()
			granularity = ""
			break
		case key == "violation":
			value = rand.Intn(2) == 0
			granularity = ""
			break
		case key == "hosts":
			value = rand.Int63()
			granularity = ""
			break
		case key == "uploadspeed" || key == "downloadspeed":
			value = rand.Int63()
			granularity = unit.SpeedUnit[rand.Intn(len(unit.SpeedUnit))]
			break
		default:
			err = fmt.Errorf("the key received is not valid: %s", key)
			return
		}

		settings[key] = fmt.Sprintf("%v%v", value, granularity)
	}
	return
}

func clientSettingValidation(key string, prevSetting storage.ClientSetting, currentSetting storage.ClientSetting) (valid bool, err error) {
	switch key {
	case "fund":
		valid = currentSetting.RentPayment.Fund.IsEqual(prevSetting.RentPayment.Fund)
		return
	case "hosts":
		valid = currentSetting.RentPayment.StorageHosts == prevSetting.RentPayment.StorageHosts
		return
	case "period":
		valid = currentSetting.RentPayment.Period == prevSetting.RentPayment.Period
		return
	case "renew":
		valid = currentSetting.RentPayment.RenewWindow == prevSetting.RentPayment.RenewWindow
		return
	case "storage":
		valid = currentSetting.RentPayment.ExpectedStorage == prevSetting.RentPayment.ExpectedStorage
		return
	case "upload":
		valid = currentSetting.RentPayment.ExpectedUpload == prevSetting.RentPayment.ExpectedUpload
		return
	case "download":
		valid = currentSetting.RentPayment.ExpectedDownload == prevSetting.RentPayment.ExpectedDownload
		return
	case "redundancy":
		valid = currentSetting.RentPayment.ExpectedRedundancy == prevSetting.RentPayment.ExpectedRedundancy
		return
	case "violation":
		valid = currentSetting.EnableIPViolation == prevSetting.EnableIPViolation
		return
	case "uploadspeed":
		valid = currentSetting.MaxUploadSpeed == prevSetting.MaxUploadSpeed
		return
	case "downloadspeed":
		valid = currentSetting.MaxDownloadSpeed == prevSetting.MaxDownloadSpeed
		return
	default:
		err = fmt.Errorf("the provided key is invalid: %s", key)
		return
	}
}
