// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
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

func TestContainsDigitOnly(t *testing.T) {
	tables := []struct {
		input     string
		allDigits bool
	}{
		{"1231232131232131231231321313", true},
		{"fjsdlkfjalkfjlkahdfklhfiwehflsdjfkjf", false},
		{"21rfewf2rewf4r4", false},
		{"fdafasdf", false},
	}

	for _, table := range tables {
		result := containsDigitOnly(table.input)
		if result != table.allDigits {
			t.Errorf("the string %s is expected allcontaindigit %t, got %t",
				table.input, table.allDigits, result)
		}
	}
}

func TestParseFund(t *testing.T) {
	tables := []struct {
		fund   string
		result common.BigInt
	}{
		{"100 wei", common.NewBigInt(100)},
		{"100ether", common.NewBigInt(100).MultUint64(currencyIndexMap["ether"])},
		{"100 MILLIETHER", common.NewBigInt(100).MultUint64(currencyIndexMap["milliether"])},
		{"100  MICROether", common.NewBigInt(100).MultUint64(currencyIndexMap["microether"])},
		{"99876KWEI", common.NewBigInt(99876).MultUint64(currencyIndexMap["kwei"])},
	}

	for _, table := range tables {

		// parse the fund
		parsed, err := parseFund(table.fund)
		if err != nil {
			t.Fatalf("failed to parse the fund %s: %s", table.fund, err.Error())
		}

		// parsed result verification
		if !parsed.IsEqual(table.result) {
			t.Errorf("the parsed value does not match with the expected result: expected %+v, got %+v",
				table.result, parsed)
		}
	}
}

func TestParseFundFail(t *testing.T) {
	var failedExpected = []string{
		"100dxc",     // does not match with any unit
		"100fwei",    // error, even the suffix is dx
		"120cdwei",   // error, even the suffix is dx
		"a1200ether", // error, even the suffix is dx
		"120gether",  // error, does not match with any unit
	}

	for _, failedCase := range failedExpected {
		if _, err := parseFund(failedCase); err == nil {
			t.Errorf("error is expected with the input %s", failedCase)
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

func TestParsePeriodRenew(t *testing.T) {
	var tables = []struct {
		period string
		parsed uint64
		err    bool
	}{
		{"100 d", 100 * storage.BlocksPerDay, false},
		{"182 h", 182 * storage.BlockPerHour, false},
		{"179 W", 179 * storage.BlocksPerWeek, false},
		{"3000 M", 3000 * storage.BlocksPerMonth, false},
		{"10 y", 10 * storage.BlocksPerYear, false},
		{"10000 b", 10000, false},
		{"10000 J", 0, true},
		{"100u0 d", 0, true},
	}

	for _, table := range tables {
		result, err := parsePeriodAndRenew(table.period)
		if err != nil && table.err {
			continue
		}

		if err != nil {
			t.Fatalf("error is not expected, got error: %s", err.Error())
		}

		if result != table.parsed {
			t.Errorf("input %+v, epxected parsed value %+v, got %+v",
				table.period, table.parsed, result)
		}
	}
}

func TestParseExpectedStorage(t *testing.T) {
	var tables = []struct {
		dataSize string
		parsed   uint64
		err      bool
	}{
		{"256b", 256, false},
		{"1000 b", 1000, false},
		{"431 KB", 431 * 1e3, false},
		{"486 mB", 486 * 1e6, false},
		{"1025 gb", 1025 * 1e9, false},
		{"3 tB", 3 * 1e12, false},
		{"431 mib", 431 * 1 << 20, false},
		{"572 tib", 572 * 1 << 40, false},
		{"572 tibb", 0, true},
	}

	for _, table := range tables {
		result, err := parseExpectedStorage(table.dataSize)
		if err != nil && table.err {
			continue
		} else if err != nil {
			t.Fatalf("error parsing the expected storage: %s", err.Error())
		}

		if result != table.parsed {
			t.Errorf("error parsing: expected parsed storage size %+v, got %+v",
				table.parsed, result)
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

func TestParseEnableIPViolation(t *testing.T) {
	var tables = []struct {
		enable string
		parsed bool
		err    bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"1", false, true},
	}

	for _, table := range tables {
		result, err := parseEnableIPViolation(table.enable)
		if err != nil && table.err {
			continue
		} else if err != nil {
			t.Fatalf("error parsing the ip violation enable: %s", err.Error())
		}

		if result != table.parsed {
			t.Errorf("error parsing: expected parsed ip violation enable %t, got %t", table.parsed,
				result)
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

	var value, unit interface{}

	rand.Seed(time.Now().UnixNano())

	for key, _ := range selectedKeys {
		switch {
		case key == "fund":
			value = common.RandomBigInt()
			unit = currencyUnit[rand.Intn(len(currencyUnit))]
			break
		case key == "period" || key == "renew":
			value = rand.Uint64()
			unit = timeUnit[rand.Intn(len(timeUnit))]
			break
		case key == "storage" || key == "upload" || key == "download":
			value = rand.Uint64()
			unit = dataSizeUnit[rand.Intn(len(dataSizeUnit))]
			break
		case key == "redundancy":
			value = rand.Float64()
			unit = ""
			break
		case key == "violation":
			value = rand.Intn(2) == 0
			unit = ""
			break
		case key == "uploadspeed" || key == "downloadspeed" || key == "hosts":
			value = rand.Int63()
			unit = ""
			break
		default:
			err = fmt.Errorf("the key received is not valid: %s", key)
			return
		}

		settings[key] = fmt.Sprintf("%v%v", value, unit)
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
