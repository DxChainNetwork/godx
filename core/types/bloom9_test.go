package types

import (
	"bytes"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"math/big"
	"testing"
)

// TestBytesToBloom_Bytes test conversion from bytes to bloom, and bloom to bytes
func TestBytesToBloom_Bytes(t *testing.T) {
	tests := [][]byte{
		{},
		{0x01},
		{0xff, 0xff, 0xee},
		bytes.Repeat([]byte{0xff}, 256),
	}
	for i, test := range tests {
		// Convert bytes to bloom
		res := BytesToBloom(test)
		if !bytes.Equal(res[:BloomByteLength-len(test)], bytes.Repeat([]byte{0}, BloomByteLength-len(test))) {
			t.Errorf("The first part of bloom is not all 0: %s", res[:BloomByteLength-len(test)])
		}
		if !bytes.Equal(res[BloomByteLength-len(test):], test) {
			t.Errorf("The last part of bloom does not equal to input data. Want: %x. Got %x", test,
				res[BloomByteLength-len(test):])
		}
		// Convert back from bloom to bytes
		b := res.Bytes()
		CheckEquality(t, string(i), "Bytes_prefix", b[:BloomByteLength-len(test)],
			bytes.Repeat([]byte{0}, BloomByteLength-len(test)))
		CheckEquality(t, string(i), "Bytes_suffix", b[BloomByteLength-len(test):],
			test[:])
	}
}

// When input a byte of length larger than 256, supposed to panic.
func TestBytesToBloomPanic(t *testing.T) {
	data := bytes.Repeat([]byte{0x01}, 257)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code does not panic")
		}
	}()
	BytesToBloom(data)
}

func TestBloom_SetBytes(t *testing.T) {
	tests := [][]byte{
		{},
		{0x01},
		{0xff, 0xff, 0xee},
		bytes.Repeat([]byte{0x01}, 256),
	}
	for _, test := range tests {
		res := BytesToBloom(test)
		if !bytes.Equal(res[:BloomByteLength-len(test)], bytes.Repeat([]byte{0}, BloomByteLength-len(test))) {
			t.Errorf("The first part of bloom is not all 0: %s", res[:BloomByteLength-len(test)])
		}
		if !bytes.Equal(res[BloomByteLength-len(test):], test) {
			t.Errorf("The last part of bloom does not equal to input data. Got: %x. Want: %x",
				res[BloomByteLength-len(test):], test)
		}
	}
}

// TestBloom9 will check result from Bloom9.
// 1. The result should have less or equal to 3 bits of 1.
// 2. The result for multiple input should be different.
func TestBloom9(t *testing.T) {
	tests := [][]byte{
		{0x01, 0x02, 0x03},
		{0x21, 0x12, 0xff},
		{0x21, 0x83, 0xa1},
	}
	found := make(map[string]interface{})
	for _, test := range tests {
		res := Bloom9(test)
		if _, ok := found[string(res.Bytes())]; ok {
			t.Errorf("Bloom result found in previous results. %x", res)
		}
		found[string(res.Bytes())] = struct{}{}
		// Count the number of bit 1s
		var count int
		cur := new(big.Int).Set(res)
		for cur.Cmp(big.NewInt(0)) > 0 {
			and := big.NewInt(0)
			and = and.And(cur, big.NewInt(1))
			if and.Cmp(big.NewInt(0)) != 0 {
				count++
			}
			cur = cur.Rsh(cur, 1)
		}
		cur.Add(cur, big.NewInt(1))
		if count > 3 {
			t.Errorf("Bloom 9 should not have '1' bits more than number 3. Input %x Got %x", test, res)
		}
	}
}

// TestBloomLoopUp test the lookup function for bloom.
func TestBloomLookup(t *testing.T) {
	b := BytesToBloom([]byte{
		0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x00, 0x80, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	positive := [][]byte{
		{0x01, 0x02, 0x03},
		{0x21, 0x12, 0xff},
		{0x21, 0x83, 0xa1},
	}
	negative := [][]byte{
		{0x11, 0x22, 0x33},
		{0x55, 0x66, 0x99},
	}
	for _, data := range positive {
		if !BloomLookup(b, new(big.Int).SetBytes(data)) {
			t.Errorf("Bloom cannot find data %x", data)
		}
	}
	for _, data := range negative {
		if BloomLookup(b, new(big.Int).SetBytes(data)) {
			t.Errorf("Bloom can fin data not inserted %x", data)
		}
	}
}

// TestBloom_Bytes shows the basic usage of Bloom type for bytes.
// 1. Add some string called positive
// 2. Try to find the positive string, shall return true.
// 3. Try to find the not inserted string, shall return false.
func TestBloom_TestBytes(t *testing.T) {
	positive := []string{
		"dxchain",
		"dxchaingogogogogo",
		"jackyishandsome",
		"wudi",
		"wu di shi duo me, duo me ji mo",
	}
	negative := []string{
		"manxiangisweak",
		"not just weak today, weak every day",
	}
	var bloom Bloom
	for _, data := range positive {
		bloom.Add(new(big.Int).SetBytes([]byte(data)))
	}

	for _, data := range positive {
		if !bloom.TestBytes([]byte(data)) {
			t.Error("expected", data, "to test true")
		}
	}
	for _, data := range negative {
		if bloom.TestBytes([]byte(data)) {
			t.Error("expected", data, "to test false")
		}
	}
}

// TestBloom_Int shows the basic usage of Bloom type for int.
// The process is identical to TestBloom_Bytes
func TestBloom_TestInt(t *testing.T) {
	positive := []*big.Int{
		big.NewInt(0),
		big.NewInt(10),
		big.NewInt(129038571293815),
	}
	negative := []*big.Int{
		big.NewInt(15),
		big.NewInt(32),
		big.NewInt(2388092891994897),
	}
	var bloom Bloom
	for _, data := range positive {
		bloom.Add(data)
	}

	for _, data := range positive {
		if !bloom.Test(data) {
			t.Error("expected", data, "to test true")
		}
	}
	for _, data := range negative {
		if bloom.Test(data) {
			t.Error("expected", data, "to test false")
		}
	}
}

func TestCreateBloom(t *testing.T) {
	// Create bl based on random data
	data := MakeReceipts(20, common.Address{})
	bl := CreateBloom(data)
	//Check whether log address and topics are in bloom
	for _, r := range data {
		for _, l := range r.Logs {
			if !BloomLookup(bl, l.Address) {
				t.Errorf("LogsBloom does not contain address %x", l.Address)
			}
			for _, topic := range l.Topics {
				if !BloomLookup(bl, topic) {
					t.Errorf("Logbloom does not contain topic %x", topic)
				}
			}
		}
	}
}

// TestLogsBloom test LogsBloom
// 1. generate random data
// 2. Create the bloom based on this random data
// 3. Check whether the bloom contain these keys
func TestLogsBloom(t *testing.T) {
	// Create bl based on random data
	data := MakeRandomLogs(20, uint64(0), common.Hash{}, common.Hash{}, uint(0), uint(0))
	bl := Bloom{}
	bl.SetBytes(LogsBloom(data).Bytes())

	for _, log := range data {
		//fmt.Printf("%x\n", log.Address)
		if !BloomLookup(bl, log.Address) {
			t.Errorf("LogsBloom does not contain address %x", log.Address)
		}
		for _, topic := range log.Topics {
			//fmt.Printf("%x\n", topic.Bytes())
			if !BloomLookup(bl, topic) {
				t.Errorf("Logbloom does not contain topic %x", topic)
			}
		}
	}

}

type myByte string

func (mb myByte) Bytes() []byte {
	return []byte(string(mb))
}

// TestBloom_Bytes shows the basic usage of Bloom type for bytes.
// 1. Add some string called positive
// 2. Try to find the positive string, shall return true.
// 3. Try to find the not inserted string, shall return false.
func ExampleBloom() {
	positive := []myByte{
		" ",
		"dxchaingogogogogo",
		"123123123213123123123123123123",
	}
	negative := []myByte{
		"31231231231",
	}

	var bloom Bloom
	for _, data := range positive {
		bloom.Add(new(big.Int).SetBytes([]byte(data)))
	}

	for _, data := range positive {
		if BloomLookup(bloom, data) {
			fmt.Println("Inserted value", data, "found in bloom")
		}
	}
	for _, data := range negative {
		if !BloomLookup(bloom, data) {
			fmt.Println("Value haven't inserted", data, "not found in bloom")
		}
	}
	// Output:
	// Inserted value   found in bloom
	// Inserted value dxchaingogogogogo found in bloom
	// Inserted value 123123123213123123123123123123 found in bloom
	// Value haven't inserted 31231231231 not found in bloom

}
