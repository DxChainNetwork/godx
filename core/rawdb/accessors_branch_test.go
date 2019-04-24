package rawdb

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"math/big"
	"reflect"
	"testing"
)

// Create header chains and write into database, check if FindCommonAncestor could give correct outcome
//	1. Test header on the same branch 2. Test branch near to each other 3. test header on different branch
func TestFindCommonAncestor(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()

	// create headers and link each other to form a chain
	// header06
	// header05
	// header00 <-- header01 <-- header02 <-- header04
	//					^|
	//				header03

	header00 := &types.Header{Number: big.NewInt(0)}
	header01 := &types.Header{ParentHash: header00.Hash(), Number: big.NewInt(1)}
	header02 := &types.Header{ParentHash: header01.Hash(), Number: big.NewInt(2), Extra: []byte("this is a branch")}
	header03 := &types.Header{ParentHash: header01.Hash(), Number: big.NewInt(2), Extra: []byte("this is another branch")}
	header04 := &types.Header{ParentHash: header02.Hash(), Number: big.NewInt(3)}
	header05 := &types.Header{Number: big.NewInt(1), Extra: []byte("Ancestor of a chain")}
	header06 := &types.Header{Number: big.NewInt(3), Extra: []byte("Ancestor of a another chain")}

	// write nodes into database
	WriteHeader(db, header00)
	WriteHeader(db, header01)
	WriteHeader(db, header02)
	WriteHeader(db, header03)
	WriteHeader(db, header04)
	WriteHeader(db, header05)
	WriteHeader(db, header06)

	testHasCommonAncestor := []struct {
		h1 *types.Header
		h2 *types.Header
		h3 *types.Header
	}{
		{header02, header03, header01}, //different branch
		{header01, header04, header01}, //test the node near each other
		{header00, header04, header00}, // test the node far away
	}

	testNoCommonAncestor := []struct {
		h1 *types.Header
		h2 *types.Header
		h3 *types.Header
	}{
		{header00, header05, nil}, // test no common ancestor
		{header05, header00, nil}, // branch coverage test
		{header04, header06, nil}, // branch coverage test
		{header06, header04, nil}, // branch coverage test
	}

	// testcase of has common ancestor
	for _, itm := range testHasCommonAncestor {
		if header := FindCommonAncestor(db, itm.h1, itm.h2); header == nil || header.Hash() != itm.h3.Hash() {
			t.Errorf("Fail to find the correct ancestor")
		}
	}

	// testcase of no common ancestor
	for _, itm := range testNoCommonAncestor {
		if header := FindCommonAncestor(db, itm.h1, itm.h2); header != nil {
			t.Errorf("Nodes should not have the common ancestor")
		}
	}
}

// Test hash and readheader function
// !!!!!!!!!!Type of number does not handled, which expected not nil and less than 0!!!!!!!!!!
func TestHasHeader(t *testing.T) {
	db := ethdb.NewMemDatabase()
	var height int64 = 0
	// write header into database to check its status
	header := &types.Header{Number: big.NewInt(height), Extra: []byte("test header")}

	if entry := HasHeader(db, header.Hash(), uint64(height)); entry {
		t.Fatalf("Find non-exsisting data in the database")
	}
	WriteHeader(db, header)

	// test has the header
	if entry := HasHeader(db, header.Hash(), uint64(height)); !entry {
		t.Fatalf("Cannot find the header written into database")
	}

	// test if the header in the database
	if entry := ReadHeaderNumber(db, header.Hash()); *entry != uint64(height) {
		t.Fatalf("Number does not the same as when the data been wrote")
	}
}

// Test if the latest triprogress can be get
//	1. Test bound uint64 (0,18446744073709551615)
//	2. normal case
func TestTrieProgressStorage(t *testing.T) {
	db := ethdb.NewMemDatabase() // create database instance
	defer db.Close()
	testcase := []uint64{
		18446744073709551615, // maximum uint64
		0,                    // 0 case
		1,                    // 1 case
		50,                   // normal case
	}

	for _, i := range testcase {
		if WriteFastTrieProgress(db, i); i != ReadFastTrieProgress(db) {
			t.Errorf("Write value not equal the read value")
		}
	}
}

// Test if the Hasbody method before and after record the body into database
func TestBodyHas(t *testing.T) {
	db := ethdb.NewMemDatabase() // create database instance
	defer db.Close()
	// create a body to as testcase
	body := &types.Body{Uncles: []*types.Header{{Number: big.NewInt(314)}, {Extra: []byte("test body")}}}
	var height uint64 = 100
	hash := common.BytesToHash([]byte("test hash"))
	if entry := HasBody(db, hash, height); entry {
		t.Fatalf("found non existing data")
	}

	WriteBody(db, hash, height, body)

	if entry := HasBody(db, hash, height); !entry {
		t.Fatalf("Stored body not found")
	}
}

// Warning: not a complete implementation of comparator, only use
// for checking the receipts of TestHasReceipts
func comparator(stru1 interface{}, stru2 interface{}) bool {
	// get the structure type for both
	val1 := reflect.ValueOf(stru1)
	val2 := reflect.ValueOf(stru2)
	if val1.Kind() != val2.Kind() {
		return false
	}
	// loop through the filed
	for i := 0; i < val1.NumField(); i++ {
		// if the kind of each field does not match each othere
		if val1.Field(i).Kind() != val2.Field(i).Kind() {
			return false
		}
		switch val1.Field(i).Kind() {
		// check if is slice, pointer in the slice cannot use reflector to compare
		case reflect.Slice:
			if val1.Field(i).IsNil() {
				if !val2.Field(i).IsNil() && val2.Field(i).Len() == 0 {
					continue
				}
				if !val2.Field(i).IsNil() {
					return false
				}
			} else if val1.Field(i).Len() == 0 {
				if val2.Field(i).IsNil() {
					continue
				}
				if val2.Field(i).Len() != 0 {
					return false
				}
			} else {
				if val1.Field(i).Len() != val2.Field(i).Len() {
					return false
				}
				switch val1.Field(i).Index(0).Kind() {
				// check if it is an pointer, recurssivly call comparator to check in depth
				case reflect.Ptr:
					for j := 0; j < val1.Field(i).Len(); j++ {
						if !comparator(val1.Field(i).Index(j).Elem(),
							val2.Field(i).Index(j).Elem()) {
							return false
						}
					}
				default:
					if !reflect.DeepEqual(val1.Field(i).Interface(), val2.Field(i).Interface()) {
						return false
					}
				}
			}
		}
	}
	return true
}

/**
Test the receipt with no block: where receipt are not write into the database
*/
func testReadReceiptNonBlock(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	// loop through the test transaction, they should not be found in the database
	for _, tx := range txs {
		if receipt, _, _, _ := ReadReceipt(db, tx.Hash()); receipt != nil {
			t.Errorf("found non existing block")
		}
	}
}

/**
@ param: txs: transactions, require transaction length > receipt length to pass the test
@ param receipts: receipts, require receipt < transaction length to pass the test
*/
func testReadInequivalent(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {
	// create database instance
	db := ethdb.NewMemDatabase()
	defer db.Close()
	// create block instance, which record both the transactions and the receipts
	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil, receipts)

	//record every things into the database
	WriteBlock(db, block)
	WriteTxLookupEntries(db, block)
	//WriteReceipts(db, block.Hash(), block.NumberU64(), receipts)

	// loop through and try to find the receipt in the database, every thing should be well recorded
	for i := len(receipts); i < len(txs); i++ {
		if receipt, _, _, _ := ReadReceipt(db, txs[i].Hash()); receipt != nil {
			t.Errorf("find non existing receipts")
		}
	}
}

/**
@ param: txs: transactions, require transaction length == receipt length to pass the test
@ param receipts: receipts, require receipt == transaction length to pass the test
Test read the receipt in general case.
*/
func testReadReceiptGeneralCase(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	// create block instance, which record both the transactions and the receipts
	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil, receipts)

	// write every thing into the database
	WriteBlock(db, block)
	WriteTxLookupEntries(db, block)
	// check Has function when receipt not recorded
	if HasReceipts(db, block.Hash(), block.NumberU64()) {
		t.Errorf("non existing receipts found")
	}
	WriteReceipts(db, block.Hash(), block.NumberU64(), receipts)

	// loop through the transaction, they should be well recorded
	for i, tx := range txs {
		blockhash, _, index := ReadTxLookupEntry(db, tx.Hash())
		if blockhash != block.Hash() {
			t.Errorf("cannot find blockchain according to a transaction")
		}
		if txs[index].Hash() != txs[i].Hash() {
			t.Errorf("transaction hash does not match")
		}
	}

	if !HasReceipts(db, block.Hash(), block.NumberU64()) {
		t.Errorf("Cannot find receipts just added")
	}

	// loop through the transactions and find the corresponding receipt
	for i, tx := range txs {
		ret_receipt, _, _, _ := ReadReceipt(db, tx.Hash())
		if ret_receipt == nil {
			t.Errorf("cannot find receipt")
		} else if !comparator(*receipts[i], *ret_receipt) {
			t.Errorf("ReadReceipt cannot read the things store by the WriteReceipts")
		} else if tx.Hash() != ret_receipt.TxHash {
			t.Errorf("Transaction receipt does not match receipt's transaction hash")
		}
	}
}

/**
Contain two test case:
1. General case
2. Transaction with a block with only the header
*/
func testReadTransaction(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {
	testReadTransactionGeneral(t, txs, receipts)
	testReadTransactionEmptybody(t, txs, receipts)
}

/**
Helper function of the testReadTransaction, which test if reader work in general case
*/
func testReadTransactionGeneral(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	// init a block with body and head
	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil, receipts)
	// write the data into the database
	WriteBlock(db, block)
	WriteTxLookupEntries(db, block)
	WriteReceipts(db, block.Hash(), block.NumberU64(), receipts)
	// loop around the transactions
	for i, tx := range txs {
		if ret_tx, blockhash, blocknum, txindex := ReadTransaction(db, tx.Hash()); ret_tx.Hash() != tx.Hash() {
			t.Fatalf("fail to read the transaction")
		} else if blockhash != block.Hash() {
			t.Fatalf("fail to read the transaction")
		} else if blocknum != block.NumberU64() {
			t.Fatalf("fail to read the transaction")
		} else if txindex != uint64(i) {
			t.Fatalf("fail to read the transaction")
		}
	}
}

/**
Helper function of testReadTransaction, which test if reader work as expected when encounter an empty body block
*/
func testReadTransactionEmptybody(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	// init a block without body
	nonbodyblock := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(314)})
	// write data to db
	WriteBlock(db, nonbodyblock)
	WriteTxLookupEntries(db, nonbodyblock)
	WriteReceipts(db, nonbodyblock.Hash(), nonbodyblock.NumberU64(), receipts)
	// loop through the transactions, they should not be found when the block body is empty
	for _, tx := range txs {
		if ret_tx, blockhash, blocknum, txindex := ReadTransaction(db, tx.Hash()); ret_tx != nil {
			t.Fatalf("found non existing transaction")
		} else if blockhash != (common.Hash{}) {
			t.Fatalf("found non existing transaction")
		} else if blocknum != 0 {
			t.Fatalf("found non existing transaction")
		} else if txindex != 0 {
			t.Fatalf("found non existing transaction")
		}
	}
}

/**
Test if only write block the transaction and receipt would be recorded
*/
func testReceiptsRecord(t *testing.T, txs []*types.Transaction, receipts []*types.Receipt) {

	db := ethdb.NewMemDatabase()
	defer db.Close()

	// generate a new block, with transaction and receipts as parameter
	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil, receipts)

	// test if only write block into the database if the transaction and receipts would be recorded
	WriteBlock(db, block)

	if HasReceipts(db, block.Hash(), block.NumberU64()) {
		t.Errorf("receipt should not be recorded")
	}

	for _, tx := range txs {
		if ret_tx, _, _, _ := ReadTransaction(db, tx.Hash()); ret_tx != nil {
			t.Errorf("transaction should not be recorded")
		}
	}

	for _, tx := range txs {
		if receipt, _, _, _ := ReadReceipt(db, tx.Hash()); receipt != nil {
			t.Errorf("receipt should not be recorded")
		}
	}

}

/**
testing process function, form up the data in a large scope, but seperate database for each testcase:
1. testRead receipt with no block
2. testRead receipt when transaction number and receipt number does not match
3. testRead receipt in general case
4. testRead transaction (with helper to include the special casese): testReadTransaction
*/
func TestTransactionRelated(t *testing.T) {

	// create transactions
	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	tx2 := types.NewTransaction(2, common.BytesToAddress([]byte{0x22}), big.NewInt(222), 2222, big.NewInt(22222), []byte{0x22, 0x22, 0x22})
	tx3 := types.NewTransaction(3, common.BytesToAddress([]byte{0x33}), big.NewInt(333), 3333, big.NewInt(33333), []byte{0x33, 0x33, 0x33})
	txs := []*types.Transaction{tx1, tx2, tx3}

	// create receipts
	receipt1 := &types.Receipt{
		Status:            types.ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*types.Log{
			{Address: common.BytesToAddress([]byte{0x11})},
			{Address: common.BytesToAddress([]byte{0x01, 0x11})},
		},
		TxHash:          tx1.Hash(),
		ContractAddress: common.BytesToAddress([]byte{0x01, 0x11, 0x11}),
		GasUsed:         111111,
	}

	receipt2 := &types.Receipt{
		PostState:         common.Hash{2}.Bytes(),
		CumulativeGasUsed: 2,
		Logs: []*types.Log{
			{Address: common.BytesToAddress([]byte{0x22})},
			{Address: common.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          tx2.Hash(),
		ContractAddress: common.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         222222,
	}

	receipt3 := &types.Receipt{
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 2,
		Logs: []*types.Log{
			{Address: common.BytesToAddress([]byte{0x22})},
			{Address: common.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          tx3.Hash(),
		ContractAddress: common.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         222222,
	}

	// general receipts, which match the transaction exactly
	receipts := []*types.Receipt{receipt1, receipt2, receipt3}
	// missing receipts, whihc miss match the transaction
	missreceipts := []*types.Receipt{receipt1, receipt2}

	testReadReceiptNonBlock(t, txs, receipts)
	testReadInequivalent(t, txs, missreceipts)
	testReadReceiptGeneralCase(t, txs, receipts)
	testReadTransaction(t, txs, receipts)
	testReceiptsRecord(t, txs, receipts)
}

/*
	Test bloom storage: Test read the bloom bits before and after the recording
*/
func TestBloom(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()

	// construct testcases
	bloombits := []struct {
		bit     uint
		section uint64
		head    common.Hash
		bits    []byte
	}{
		{0, 0, common.BytesToHash([]byte("test hash1")), nil},                                                       // nil bits
		{18446744073709551615, 18446744073709551615, common.BytesToHash([]byte("test hash2")), []byte("test bits")}, // maximum bloombits
		{1000, 1000, common.BytesToHash([]byte("test hash3")), []byte("test bits2")},                                // normal case
	}

	// loop and check before the bloom recorded in to the database, after checking, record the bloom into the database
	for _, bloom := range bloombits {
		if _, err := ReadBloomBits(db, bloom.bit, bloom.section, bloom.head); err == nil {
			t.Errorf("find non existing bloombit in the database")
		}
		WriteBloomBits(db, bloom.bit, bloom.section, bloom.head, bloom.bits)
	}

	// loop again, this time, the bloom should be well recorded in the database
	for _, bloom := range bloombits {
		if bits, err := ReadBloomBits(db, bloom.bit, bloom.section, bloom.head); err != nil {
			t.Errorf(err.Error())
		} else if common.BytesToHash(bits) != common.BytesToHash(bloom.bits) {
			t.Errorf("data stored in db does not match the expected")
		}
	}
}
