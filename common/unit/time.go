// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package unit

import (
	"fmt"
	"strings"
)

// TimeUnit defines available units used for period and renew
var TimeUnit = []string{"h", "b", "d", "w", "m", "y"}

// ParseTime will parse the string version of period into uint64 type based on the
// unit provided. The supported units are blocks, hour, day, week, month, year
func ParseTime(str string) (parsed uint64, err error) {
	// format the string
	str = strings.Replace(str, " ", "", -1)
	str = strings.ToLower(str)
	unit := string(str[len(str)-1])

	switch {
	case unit == "h":
		// convert the time hour to number of blocks
		return ParseUint64(str, BlockPerHour, "h")
	case unit == "b":
		// return the number of blocks directly
		return ParseUint64(str, 1, "b")
	case unit == "d":
		// convert the time day to number of blocks
		return ParseUint64(str, BlocksPerDay, "d")

	case unit == "w":
		// convert the time week to number of blocks
		return ParseUint64(str, BlocksPerWeek, "w")

	case unit == "m":
		// convert the time month to number of blocks
		return ParseUint64(str, BlocksPerMonth, "m")

	case unit == "y":
		// convert the time year to number of blocks
		return ParseUint64(str, BlocksPerYear, "y")
	default:
		err = fmt.Errorf("valid unit must be provided: %v", TimeUnit)
		return
	}
}

// MustParseTime parse the string to duration. If an error happens, panic.
// WARNING: Do not use this function in production code. Use ParseTime instead.
func MustParseTime(str string) int64 {
	parsed, err := ParseTime(str)
	if err != nil {
		panic(err)
	}
	return parsed
}

// FormatTime is used to format the period and renewWindow field
// for displaying purpose
func FormatTime(time uint64) (formatted string) {
	switch {
	case time%BlocksPerYear == 0:
		formatted = fmt.Sprintf("%v Year(s)", time/BlocksPerYear)
		return
	case time%BlocksPerMonth == 0:
		formatted = fmt.Sprintf("%v Month(s)", time/BlocksPerMonth)
		return
	case time%BlocksPerWeek == 0:
		formatted = fmt.Sprintf("%v Week(s)", time/BlocksPerWeek)
		return
	case time%BlocksPerDay == 0:
		formatted = fmt.Sprintf("%v Day(s)", time/BlocksPerDay)
		return
	case time%BlockPerHour == 0:
		formatted = fmt.Sprintf("%v Hour(s)", time/BlockPerHour)
		return
	default:
		formatted = fmt.Sprintf("%v Minute(s)", float64(time)/float64(BlockPerMin))
		return
	}
}
