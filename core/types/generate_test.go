package types

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"math/big"
	"testing"
)

func TestMakeRandomBlock(t *testing.T) {
	b, rs := MakeRandomBlock(10, common.Hash{}, big.NewInt(1))

	CheckEquality(t, "transactions", "length", len(b.transactions), 10)
	CheckEquality(t, "receipts", "length", len(rs), 10)
	preTxIndex := 0
	// receipt.TxHash should equal to transactions in the same order
	// receipt.log should have correct transaction index
	// receipt.log should increment in log index
	for i, r := range rs {
		CheckEquality(t, fmt.Sprintf("receipt[%d]", i), "TxHash", r.TxHash, b.transactions[i].Hash())
		for j, log := range r.Logs {
			CheckEquality(t, fmt.Sprintf("receipt[%d].Logs[%d]", i, j), "TxIndex", log.TxIndex, uint(i))
			CheckEquality(t, fmt.Sprintf("receipt[%d].Logs[%d]", i, j), "index", log.Index, uint(preTxIndex))
			preTxIndex++
		}
	}
}
