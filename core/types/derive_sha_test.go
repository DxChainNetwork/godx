package types

import (
	"bytes"
	"github.com/DxChainNetwork/godx/common"
	"testing"
)

// Create a DerivableList implementing DerivableList
type testList [][]byte

func (tl testList) Len() int {
	return len(tl)
}

func (tl testList) GetRlp(i int) []byte {
	return tl[i]
}

// TestDeriveSha test the result of function DeriveSha
func TestDeriveSha(t *testing.T) {
	list := testList{
		{1, 2, 3, 4, 5, 6},
		{6, 5, 4, 3, 2, 1},
		{1, 1, 1, 1, 1, 1},
		{3, 3, 3, 3, 3, 3},
	}
	res := DeriveSha(list)
	expected := []byte{
		0x52, 0x7a, 0x68, 0x9a, 0xa5, 0x1c, 0x42, 0x5d, 0x22, 0x9f, 0xeb, 0x46, 0xa1, 0xab, 0xc8,
		0x55, 0x72, 0x12, 0x08, 0xf4, 0x56, 0xa8, 0x1f, 0x5a, 0x63, 0x5d, 0x52, 0x3f, 0xf9, 0x3d,
		0xa2, 0x16,
	}
	if !bytes.Equal(res.Bytes(), expected) {
		t.Errorf("DeriveSha Got an unexpected root hash. Got %x Want %x.", res, expected)
	}
}

// TestDeriveSha_Transactions test DeriveSha for Transactions
func TestDeriveSha_Transactions(t *testing.T) {
	length := 10
	txs := make(Transactions, 0, length)
	for i := 0; i != length; i++ {
		txs = append(txs, simpleNewTransactionByNonce(uint64(i)))
	}
	res := DeriveSha(txs)
	expected := common.FromHex("28dd2d133be3d1611630d16f28a96c5473b82d92bdbbd8e8b9339e3a4e852931")
	if !bytes.Equal(expected, res.Bytes()) {
		t.Errorf("DeriveSha for Transactions got an unexpected root hash.\nGot %x\nWant %x", res, expected)
	}
}
