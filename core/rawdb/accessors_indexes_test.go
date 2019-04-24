package rawdb

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"math/big"
	"testing"
)

/**

 ____   _____   _____  _____  _____  _   _            _        _______  ______   _____  _______  _____             _____  ______
 / __ \ |  __ \ |_   _|/ ____||_   _|| \ | |    /\    | |      |__   __||  ____| / ____||__   __|/ ____|    /\     / ____||  ____|
| |  | || |__) |  | | | |  __   | |  |  \| |   /  \   | |         | |   | |__   | (___     | |  | |        /  \   | (___  | |__
| |  | ||  _  /   | | | | |_ |  | |  | . ` |  / /\ \  | |         | |   |  __|   \___ \    | |  | |       / /\ \   \___ \ |  __|
| |__| || | \ \  _| |_| |__| | _| |_ | |\  | / ____ \ | |____     | |   | |____  ____) |   | |  | |____  / ____ \  ____) || |____
 \____/ |_|  \_\|_____|\_____||_____||_| \_|/_/    \_\|______|    |_|   |______||_____/    |_|   \_____|/_/    \_\|_____/ |______|


*/
// Tests that positional lookup metadata can be stored and retrieved.
func TestLookupStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	tx2 := types.NewTransaction(2, common.BytesToAddress([]byte{0x22}), big.NewInt(222), 2222, big.NewInt(22222), []byte{0x22, 0x22, 0x22})
	tx3 := types.NewTransaction(3, common.BytesToAddress([]byte{0x33}), big.NewInt(333), 3333, big.NewInt(33333), []byte{0x33, 0x33, 0x33})
	txs := []*types.Transaction{tx1, tx2, tx3}

	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil, nil)

	// Check that no transactions entries are in a pristine database
	for i, tx := range txs {
		if txn, _, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: non existent transaction returned: %v", i, tx.Hash(), txn)
		}
	}
	// Insert all the transactions into the database, and verify contents
	WriteBlock(db, block)
	WriteTxLookupEntries(db, block)

	for i, tx := range txs {
		if txn, hash, number, index := ReadTransaction(db, tx.Hash()); txn == nil {
			t.Fatalf("tx #%d [%x]: transaction not found", i, tx.Hash())
		} else {
			if hash != block.Hash() || number != block.NumberU64() || index != uint64(i) {
				t.Fatalf("tx #%d [%x]: positional metadata mismatch: have %x/%d/%d, want %x/%v/%v", i, tx.Hash(), hash, number, index, block.Hash(), block.NumberU64(), i)
			}
			if tx.Hash() != txn.Hash() {
				t.Fatalf("tx #%d [%x]: transaction mismatch: have %v, want %v", i, tx.Hash(), txn, tx)
			}
		}
	}
	// Delete the transactions and check purge
	for i, tx := range txs {
		DeleteTxLookupEntry(db, tx.Hash())
		if txn, _, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: deleted transaction returned: %v", i, tx.Hash(), txn)
		}
	}
}
