package rpc

import (
	"encoding/json"
	"testing"

	"github.com/DxChainNetwork/godx/common/math"
)

var bn BlockNumber = -3

/*
	Test the function UnmarshalJSON to check if the function can correctly parse the given byte slice
	to get BlockNumber

	Input Criteria:
		1. if the byte slice contains a string number, then 0x must contained in the string's prefix

	Cases:
		earliest
		latest
		pending
		"earliest"
		regular numbers
*/
func TestUnmarshalJson(t *testing.T) {
	tables := []struct {
		dataInput []byte
		blockNum  BlockNumber
		fail      bool
	}{
		{[]byte("earliest"), EarliestBlockNumber, false},
		{[]byte("latest"), LatestBlockNumber, false},
		{[]byte("pending"), PendingBlockNumber, false},
		{[]byte(`"earliest"`), EarliestBlockNumber, false},
		{[]byte("1"), BlockNumber(1), true},
		{[]byte("0x1"), BlockNumber(1), false},
		{[]byte("0x17569"), BlockNumber(17569), false},
		{[]byte(`"0x"`), BlockNumber(0), false},
		{[]byte(`""`), BlockNumber(0), true},
		{[]byte(``), BlockNumber(0), true},
	}

	for _, table := range tables {
		err := (&bn).UnmarshalJSON(table.dataInput)
		// if error and the function is not expected to fail
		if err != nil && !table.fail {
			t.Errorf("Error: %s", err)
		}

		// if error and the function is expected to fail
		if err != nil && table.fail {
			return
		}

		if bn != table.blockNum {
			t.Fatalf("input %s, got blockNum: %d, expected blockNum %d",
				table.dataInput, bn, table.blockNum)
		}
	}
}

/*


  ____  _____  _____ _____ _____ _   _          _        _______ ______  _____ _______ _____           _____ ______  _____
 / __ \|  __ \|_   _/ ____|_   _| \ | |   /\   | |      |__   __|  ____|/ ____|__   __/ ____|   /\    / ____|  ____|/ ____|
| |  | | |__) | | || |  __  | | |  \| |  /  \  | |         | |  | |__  | (___    | | | |       /  \  | (___ | |__  | (___
| |  | |  _  /  | || | |_ | | | | . ` | / /\ \ | |         | |  |  __|  \___ \   | | | |      / /\ \  \___ \|  __|  \___ \
| |__| | | \ \ _| || |__| |_| |_| |\  |/ ____ \| |____     | |  | |____ ____) |  | | | |____ / ____ \ ____) | |____ ____) |
 \____/|_|  \_\_____\_____|_____|_| \_/_/    \_\______|    |_|  |______|_____/   |_|  \_____/_/    \_\_____/|______|_____/



*/

func TestBlockNumberJSONUnmarshal(t *testing.T) {
	tests := []struct {
		input    string
		mustFail bool
		expected BlockNumber
	}{
		0:  {`"0x"`, true, BlockNumber(0)},
		1:  {`"0x0"`, false, BlockNumber(0)},
		2:  {`"0X1"`, false, BlockNumber(1)},
		3:  {`"0x00"`, true, BlockNumber(0)},
		4:  {`"0x01"`, true, BlockNumber(0)},
		5:  {`"0x1"`, false, BlockNumber(1)},
		6:  {`"0x12"`, false, BlockNumber(18)},
		7:  {`"0x7fffffffffffffff"`, false, BlockNumber(math.MaxInt64)},
		8:  {`"0x8000000000000000"`, true, BlockNumber(0)},
		9:  {"0", true, BlockNumber(0)},
		10: {`"ff"`, true, BlockNumber(0)},
		11: {`"pending"`, false, PendingBlockNumber},
		12: {`"latest"`, false, LatestBlockNumber},
		13: {`"earliest"`, false, EarliestBlockNumber},
		14: {`someString`, true, BlockNumber(0)},
		15: {`""`, true, BlockNumber(0)},
		16: {``, true, BlockNumber(0)},
	}

	for i, test := range tests {
		var num BlockNumber
		err := json.Unmarshal([]byte(test.input), &num)
		if test.mustFail && err == nil {
			t.Errorf("Test %d should fail", i)
			continue
		}
		if !test.mustFail && err != nil {
			t.Errorf("Test %d should pass but got err: %v", i, err)
			continue
		}
		if num != test.expected {
			t.Errorf("Test %d got unexpected value, want %d, got %d", i, test.expected, num)
		}
	}
}
