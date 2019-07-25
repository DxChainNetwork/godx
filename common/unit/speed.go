// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package unit

import (
	"fmt"
	"strconv"
	"strings"
)

// SpeedUnit defines available units used for specifying upload and download speed
var SpeedUnit = []string{"bps", "kbps", "mbps", "gbps", "tbps"}

var SpeedMultiplier = map[string]int64{
	"bps":  1,
	"kbps": 1e3,
	"mbps": 1e6,
	"gbps": 1e9,
	"tbps": 1e12,
}

// ParseSpeed will parse the string into maxUploadSpeed or maxDownloadSpeed which will be used
// to limit the speed while uploading/downloading data
func ParseSpeed(str string) (parsed int64, err error) {
	// remove whitespace and convert all to lower case
	str = formatString(str)

	// loop through the SpeedMultiplier
	for unit := range SpeedMultiplier {
		// unit bps is ignored due to each unit contains bps as suffix
		if unit == "bps" {
			continue
		}
		if strings.HasSuffix(str, unit) {
			return speedConvert(str, unit)
		}
	}

	// check the bps unit
	if strings.HasSuffix(str, "bps") {
		return speedConvert(str, "bps")
	}

	// there is no qualified unit
	err = fmt.Errorf("failed to parse the upload/download speed, unit must be included. Here is a list of available units: %s", SpeedUnit)
	return
}

// MustParseSpeed parse the string to speed. If an error happens, panic.
// WARNING: Do not use this function in production code. Use ParseSpeed instead.
func MustParseSpeed(str string) int64 {
	parsed, err := ParseSpeed(str)
	if err != nil {
		panic(err)
	}
	return parsed
}

// FormatSpeed is used to format max upload and download field for displaying
// purpose
func FormatSpeed(speed int64) (formatted string) {
	// if the speed is 0, means unlimited
	if speed == 0 {
		formatted = fmt.Sprintf("Unlimited")
		return
	}

	switch {
	case speed%SpeedMultiplier["tbps"] == 0:
		formatted = fmt.Sprintf("%v Tbps", speed/SpeedMultiplier["tbps"])
		return
	case speed%SpeedMultiplier["gbps"] == 0:
		formatted = fmt.Sprintf("%v Gbps", speed/SpeedMultiplier["gbps"])
		return
	case speed%SpeedMultiplier["mbps"] == 0:
		formatted = fmt.Sprintf("%v Mbps", speed/SpeedMultiplier["mbps"])
		return
	case speed%SpeedMultiplier["kbps"] == 0:
		formatted = fmt.Sprintf("%v Kbps", speed/SpeedMultiplier["kbps"])
		return
	default:
		formatted = fmt.Sprintf("%v bps", speed)
		return
	}
}

// speedConvert will convert the upload and download speed to bps, in form of int64
func speedConvert(speed string, unit string) (parsed int64, err error) {
	// remove the suffix
	speed = strings.TrimSuffix(speed, unit)

	// parse the string into int64 format
	if parsed, err = strconv.ParseInt(speed, 10, 64); err != nil {
		err = fmt.Errorf("failed to parse the speed provided, unit must be included. Here is a list of available units: %s", SpeedUnit)
		return
	}

	// convert the the result into bps format
	parsed *= SpeedMultiplier[unit]

	return
}
