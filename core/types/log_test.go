package types

import (
	"encoding/json"
	"errors"
	"github.com/DxChainNetwork/godx/rlp"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
)

// TestLog_EncodeRLP test the encode function for Log.
// This test function will not call EncodeRLP directly, but rather call rlp.EncodeToByte
// cause Log implements rlp.Encoder.
func TestLog_EncodeRLP(t *testing.T) {
	test, ok := logTestData["ok"]
	if !ok {
		t.Errorf("Cannot get the test data")
	}
	expected := common.FromHex("f87a94ecf8f87f810ecf450940c9f60066b4a7a501d6a7f842a0ddf252a" +
		"d1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa000000000000000000000000" +
		"080b2c9d7cbbf30a1b0fc8983c647d754c6525615a0000000000000000000000000000000000000000" +
		"000000001a055690d9db80000")

	val, err := rlp.EncodeToBytes(test.want)
	CheckError(t, "_", err, nil)
	CheckEquality(t, "_", "rlp", val, expected)
}

// TestLog_DecodeRLP test the DecodeRLP function for Log structure.
// This test function will not call DecodeRLP directly, but rather call rlp.DecodeBytes
// cause Log implements rlp.Decoder.
func TestLog_DecodeRLP(t *testing.T) {
	input := common.FromHex("f87a94ecf8f87f810ecf450940c9f60066b4a7a501d6a7f842a0ddf252ad1b" +
		"e2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa000000000000000000000000080" +
		"b2c9d7cbbf30a1b0fc8983c647d754c6525615a0000000000000000000000000000000000000000000" +
		"000001a055690d9db80000")
	var res Log
	err := rlp.DecodeBytes(input, &res)
	if err != nil {
		t.Errorf("Cannot decode byte " + err.Error())
	}
	target, ok := logTestData["ok"]
	if !ok {
		t.Fatal("Cannot get test data for Log")
	}
	CheckEquality(t, "_", "address", res.Address, target.want.Address)

	CheckEquality(t, "_", "Topics", res.Topics, target.want.Topics)

	CheckEquality(t, "_", "data", res.Topics, target.want.Topics)
}

// TestLog_EncodeRLP test the encode function for LogForStorage.
// This test function will not call EncodeRLP directly, but rather call rlp.EncodeToByte
// cause LogForStorage implements rlp.Encoder.
func TestLogForStorage_EncodeRLP(t *testing.T) {
	test, ok := logTestData["ok"]
	if !ok {
		t.Errorf("Cannot get the test data")
	}
	expected := common.FromHex("f8c294ecf8f87f810ecf450940c9f60066b4a7a501d6a7f842a0ddf252a" +
		"d1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa000000000000000000000000" +
		"080b2c9d7cbbf30a1b0fc8983c647d754c6525615a0000000000000000000000000000000000000000" +
		"000000001a055690d9db80000831ecfa4a03b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e" +
		"030251acb87f6487e03a0656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681" +
		"05602")
	val, err := rlp.EncodeToBytes((*LogForStorage)(test.want))
	CheckError(t, "_", err, nil)
	CheckEquality(t, "_", "rlp", val, expected)
}

// TestLogForStorage_DecodeRLP test the DecodeRLP for LogForStorage.
// This test function will not call DecodeRLP directly, but rather call rlp.DecodeBytes
// cause LogForStorage implements rlp.Decoder.
func TestLogForStorage_DecodeRLP(t *testing.T) {
	input := common.FromHex("f8c294ecf8f87f810ecf450940c9f60066b4a7a501d6a7f842a0ddf252a" +
		"d1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa000000000000000000000000" +
		"080b2c9d7cbbf30a1b0fc8983c647d754c6525615a0000000000000000000000000000000000000000" +
		"000000001a055690d9db80000831ecfa4a03b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e" +
		"030251acb87f6487e03a0656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681" +
		"05602")
	var res LogForStorage
	err := rlp.DecodeBytes(input, &res)
	if err != nil {
		t.Errorf("Cannot decode byte " + err.Error())
	}
	target, ok := logTestData["ok"]
	if !ok {
		t.Errorf("Cannot get test data for Log")
	}
	CheckEquality(t, "_", "LogForStorage", res, LogForStorage(*target.want))
}

// _______ _________          _______  _______   _________ _______  _______ _________ _______
// (  ____ \\__   __/|\     /|(  ____ \(  ____ )  \__   __/(  ____ \(  ____ \\__   __/(  ____ \
// | (    \/   ) (   | )   ( || (    \/| (    )|     ) (   | (    \/| (    \/   ) (   | (    \/
// | (__       | |   | (___) || (__    | (____)|     | |   | (__    | (_____    | |   | (_____
// |  __)      | |   |  ___  ||  __)   |     __)     | |   |  __)   (_____  )   | |   (_____  )
// | (         | |   | (   ) || (      | (\ (        | |   | (            ) |   | |         ) |
// | (____/\   | |   | )   ( || (____/\| ) \ \__     | |   | (____/\/\____) |   | |   /\____) |
// (_______/   )_(   |/     \|(_______/|/   \__/     )_(   (_______/\_______)   )_(   \_______)
//
// TestUnmarshalLog checks whether the Log structure could be correctly marshaled
func TestUnmarshalLog(t *testing.T) {
	for name, test := range logTestData {
		var log *Log
		err := json.Unmarshal([]byte(test.input), &log)
		CheckError(t, name, err, test.wantError, test.wantError2)
		if test.wantError == nil && err == nil {
			if !reflect.DeepEqual(log, test.want) {
				t.Errorf("test %q:\nGot %sWant %s", name, dumper.Sdump(log), dumper.Sdump(test.want))
			}
		}
	}
}

var logTestData = map[string]struct {
	input      string
	want       *Log
	wantError  error
	wantError2 error
}{
	"ok": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		want: &Log{
			Address:     common.HexToAddress("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   common.HexToHash("0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056"),
			BlockNumber: 2019236,
			Data:        hexutil.MustDecode("0x000000000000000000000000000000000000000000000001a055690d9db80000"),
			Index:       2,
			TxIndex:     3,
			TxHash:      common.HexToHash("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"),
			Topics: []common.Hash{
				common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
				common.HexToHash("0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"),
			},
		},
	},
	"empty data": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		want: &Log{
			Address:     common.HexToAddress("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   common.HexToHash("0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056"),
			BlockNumber: 2019236,
			Data:        []byte{},
			Index:       2,
			TxIndex:     3,
			TxHash:      common.HexToHash("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"),
			Topics: []common.Hash{
				common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
				common.HexToHash("0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"),
			},
		},
	},
	"missing block fields (pending logs)": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","data":"0x","logIndex":"0x0","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		want: &Log{
			Address:     common.HexToAddress("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   common.Hash{},
			BlockNumber: 0,
			Data:        []byte{},
			Index:       0,
			TxIndex:     3,
			TxHash:      common.HexToHash("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"),
			Topics: []common.Hash{
				common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
			},
		},
	},
	"Removed: true": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3","removed":true}`,
		want: &Log{
			Address:     common.HexToAddress("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   common.HexToHash("0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056"),
			BlockNumber: 2019236,
			Data:        []byte{},
			Index:       2,
			TxIndex:     3,
			TxHash:      common.HexToHash("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"),
			Topics: []common.Hash{
				common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
			},
			Removed: true,
		},
	},
	"json error": {
		input:      `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x65634545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		wantError:  errors.New("json: cannot unmarshal hex string of odd length into Go value of type common.Hash"),
		wantError2: errors.New("json: cannot unmarshal hex string of odd length into Go struct field Log.blockHash of type common.Hash"),
	},
	"missing data": {
		input:     `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615","0x000000000000000000000000f9dff387dcb5cc4cca5b91adb07a95f54e9f1bb6"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		wantError: errors.New("missing required field 'data' for Log"),
	},
	"missing address": {
		input:     `{"blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		wantError: errors.New("missing required field 'address' for Log"),
	},
	"missing Topics": {
		input:     `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		wantError: errors.New("missing required field 'topics' for Log"),
	},
	"missing TxHash": {
		input:     `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionIndex":"0x3"}`,
		wantError: errors.New("missing required field 'transactionHash' for Log"),
	},
	"missing TxIndex": {
		input:     `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"}`,
		wantError: errors.New("missing required field 'transactionIndex' for Log"),
	},
	"missing Index": {
		input:     `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		wantError: errors.New("missing required field 'logIndex' for Log"),
	},
}
