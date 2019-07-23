// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package unit

import (
	"fmt"
	"strings"
)

// DataSizeUnit defines available units used for specifying expected storage size, expected upload size, and expected download size
var DataSizeUnit = []string{"kb", "mb", "gb", "tb", "kib", "mib", "gib", "tib"}

var dataSizeMultiplier = map[string]uint64{
	"kb":  1e3,
	"mb":  1e6,
	"gb":  1e9,
	"tb":  1e12,
	"kib": 1 << 10,
	"mib": 1 << 20,
	"gib": 1 << 30,
	"tib": 1 << 40,
}

// ParseStorage will convert the string with the unit into uint64 in the unit of byte
func ParseStorage(str string) (parsed uint64, err error) {
	// string format
	str = strings.Replace(str, " ", "", -1)
	str = strings.ToLower(str)

	// convert the data size into bytes
	for unit, multiplier := range dataSizeMultiplier {
		if strings.HasSuffix(str, unit) {
			return ParseUint64(str, multiplier, unit)
		}
	}

	if strings.HasSuffix(str, "b") {
		return ParseUint64(str, 1, "b")
	}

	err = fmt.Errorf("data provided does not have valid unit: %s. valid units are: %v",
		str, DataSizeUnit)
	return
}

// FormatStorage is used to format the data for console display purpose
func FormatStorage(dataSize uint64, storage bool) (formatted string) {
	additionalInfo := ""
	if !storage {
		additionalInfo = "/block"
	}

	switch {
	case dataSize%dataSizeMultiplier["tib"] == 0:
		formatted = fmt.Sprintf("%v TiB%s", dataSize/dataSizeMultiplier["tib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["gib"] == 0:
		formatted = fmt.Sprintf("%v GiB%s", dataSize/dataSizeMultiplier["gib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["mib"] == 0:
		formatted = fmt.Sprintf("%v MiB%s", dataSize/dataSizeMultiplier["mib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["kib"] == 0:
		formatted = fmt.Sprintf("%v KiB%s", dataSize/dataSizeMultiplier["kib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["tb"] == 0:
		formatted = fmt.Sprintf("%v TB%s", dataSize/dataSizeMultiplier["tb"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["gb"] == 0:
		formatted = fmt.Sprintf("%v GB%s", dataSize/dataSizeMultiplier["gb"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["mb"] == 0:
		formatted = fmt.Sprintf("%v MB%s", dataSize/dataSizeMultiplier["mb"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["kb"] == 0:
		formatted = fmt.Sprintf("%v KB%s", dataSize/dataSizeMultiplier["kb"], additionalInfo)
		return
	default:
		formatted = fmt.Sprintf("%v B%s", dataSize, additionalInfo)
		return
	}
}
