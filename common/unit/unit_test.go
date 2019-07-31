// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package unit

import (
	"github.com/DxChainNetwork/godx/common"
	"testing"
)

func TestParseFund(t *testing.T) {
	tables := []struct {
		fund   string
		result common.BigInt
	}{
		{"100 camel", common.NewBigInt(100)},
		{"100camel", common.NewBigInt(100).MultUint64(CurrencyIndexMap["camel"])},
		{"99876GCAMEL", common.NewBigInt(99876).MultUint64(CurrencyIndexMap["gcamel"])},
	}

	for _, table := range tables {

		// parse the fund
		parsed, err := ParseCurrency(table.fund)
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
		"100ether",   // does not match with any unit
		"100fdx",     // error, even the suffix is dx
		"120cdcamel", // error, even the suffix is dx
		"a1200camel", // error, even the suffix is dx
		"120gdxc",    // error, does not match with any unit
	}

	for _, failedCase := range failedExpected {
		if _, err := ParseCurrency(failedCase); err == nil {
			t.Errorf("error is expected with the input %s", failedCase)
		}

	}
}

func TestParseTime(t *testing.T) {
	var tables = []struct {
		period string
		parsed uint64
		err    bool
	}{
		{"100 d", 100 * BlocksPerDay, false},
		{"182 h", 182 * BlockPerHour, false},
		{"179 W", 179 * BlocksPerWeek, false},
		{"3000 M", 3000 * BlocksPerMonth, false},
		{"10 y", 10 * BlocksPerYear, false},
		{"10000 b", 10000, false},
		{"10000 J", 0, true},
		{"100u0 d", 0, true},
	}

	for _, table := range tables {
		result, err := ParseTime(table.period)
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

func TestParseStorage(t *testing.T) {
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
		result, err := ParseStorage(table.dataSize)
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

func TestParseBool(t *testing.T) {
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
		result, err := ParseBool(table.enable)
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
