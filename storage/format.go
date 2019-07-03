package storage

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
)

// FormatBool format a bool value to string
func FormatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
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
	case speed%speedMultiplier["tbps"] == 0:
		formatted = fmt.Sprintf("%v Tbps", speed/speedMultiplier["tbps"])
		return
	case speed%speedMultiplier["gbps"] == 0:
		formatted = fmt.Sprintf("%v Gbps", speed/speedMultiplier["gbps"])
		return
	case speed%speedMultiplier["mbps"] == 0:
		formatted = fmt.Sprintf("%v Mbps", speed/speedMultiplier["mbps"])
		return
	case speed%speedMultiplier["kbps"] == 0:
		formatted = fmt.Sprintf("%v Kbps", speed/speedMultiplier["kbps"])
		return
	default:
		formatted = fmt.Sprintf("%v bps", speed)
		return
	}
}

// FormatFund is used to format the rentPayment.Fund field for displaying purpose
func FormatFund(fund common.BigInt) (formatted string) {
	switch {
	case fund.DivNoRemaining(currencyIndexMap["ether"]):
		formatted = fmt.Sprintf("%v ether", fund.DivUint64(currencyIndexMap["ether"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["milliether"]):
		formatted = fmt.Sprintf("%v milliether", fund.DivUint64(currencyIndexMap["milliether"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["microether"]):
		formatted = fmt.Sprintf("%v microether", fund.DivUint64(currencyIndexMap["microether"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["gwei"]):
		formatted = fmt.Sprintf("%v Gwei", fund.DivUint64(currencyIndexMap["gwei"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["mwei"]):
		formatted = fmt.Sprintf("%v Mwei", fund.DivUint64(currencyIndexMap["mwei"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["kwei"]):
		formatted = fmt.Sprintf("%v Kwei", fund.DivUint64(currencyIndexMap["kwei"]))
		return
	default:
		formatted = fmt.Sprintf("%v wei", fund)
		return
	}
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
