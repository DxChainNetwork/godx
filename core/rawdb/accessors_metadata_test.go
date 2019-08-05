package rawdb

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/params"
	"math/big"
	"reflect"
	"testing"
)

// Test the version storage
func TestVersionstorage(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()

	// try to read if there is any version recorded, expected nil version
	if version := ReadDatabaseVersion(db); version != nil {
		t.Fatalf("Found non existing version number in the database")
	}

	// loop to write the version and read it back, test if can be read out, and can be override
	expected := []uint64{100, 18446744073709551615, 0, 1}
	for _, vers := range expected {
		WriteDatabaseVersion(db, vers)
		if version := ReadDatabaseVersion(db); *version != vers {
			t.Fatalf("cannot overwrite the version, " +
				"or read version does not match the write version")
		}
	}
}

/**
Warning: blocknumber smaller than 0 does not handled in this section
Test the chain config storage
*/
func TestReadChainConfig(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()

	var testhash = common.BytesToHash([]byte("testHash"))
	// create a testchain config to test the chain config storage
	var TestChainConfig = &params.ChainConfig{
		ChainID:             big.NewInt(-100),
		HomesteadBlock:      big.NewInt(-1111),
		DAOForkBlock:        big.NewInt(-22222),
		DAOForkSupport:      true,
		EIP150Block:         big.NewInt(1844674407370955161),
		EIP150Hash:          common.HexToHash("testChainConfig"),
		EIP155Block:         big.NewInt(1844674407370955161),
		EIP158Block:         big.NewInt(1844674407370955161),
		ByzantiumBlock:      big.NewInt(1844674407370955161),
		ConstantinopleBlock: nil,
		Ethash:              new(params.EthashConfig),
	}

	// try to read before add into database, expected a nil value
	if config := ReadChainConfig(db, testhash); config != nil {
		t.Errorf("found non existing config in database")
	}
	// write into the chain config with nil value
	WriteChainConfig(db, testhash, nil)
	// read the chain config, where recorded as nil
	if config := ReadChainConfig(db, testhash); config != nil {
		t.Errorf("found non existing config in database")
	}
	// write the testcase into the db
	WriteChainConfig(db, testhash, TestChainConfig)
	// check again, this time the chain config should be recorded
	if config := ReadChainConfig(db, testhash); config == nil {
		t.Errorf("cannot find any config in database")
	} else if !reflect.DeepEqual(config, TestChainConfig) {
		t.Errorf("config write into database does not match when read it back")
	}
}

/**
Test the preimages storage
1. Test read from empty db, 2. Test write things to the db
3. Test if the value corresponding to a key can be override to the database
*/
func TestPreimagesStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	// create preimages instance
	preimages := map[common.Hash][]byte{
		common.BytesToHash([]byte("img001")): []byte("test img 001"),
		common.BytesToHash([]byte("img002")): []byte("test img 002"),
		common.BytesToHash([]byte("img003")): []byte("test img 003"),
		common.BytesToHash([]byte("img004")): []byte("test img 004"),
		common.BytesToHash([]byte("img005")): []byte("test img 005"),
	}
	// try to read the empty db, expected nil data
	for key := range preimages {
		if data := ReadPreimage(db, key); data != nil {
			t.Errorf("find non existing data in the db")
		}
	}
	// write the images into the db
	WritePreimages(db, preimages)
	// read each imges according to the key
	for key, val := range preimages {
		if data := ReadPreimage(db, key); common.BytesToHash(val) != common.BytesToHash(data) {
			t.Errorf("read data does not match before writing it to db")
		}
	}
	// test if a value can be override corresponding to a key
	repeatimgs := map[common.Hash][]byte{
		common.BytesToHash([]byte("img001")): []byte("test img 100"),
		common.BytesToHash([]byte("img002")): []byte("test img 200"),
	}
	// expected data
	expected := map[common.Hash][]byte{
		common.BytesToHash([]byte("img001")): []byte("test img 100"),
		common.BytesToHash([]byte("img002")): []byte("test img 200"),
		common.BytesToHash([]byte("img003")): []byte("test img 003"),
		common.BytesToHash([]byte("img004")): []byte("test img 004"),
		common.BytesToHash([]byte("img005")): []byte("test img 005"),
	}

	WritePreimages(db, repeatimgs)
	// test if the images in the database match the expected value
	for key, val := range expected {
		if data := ReadPreimage(db, key); common.BytesToHash(val) != common.BytesToHash(data) {
			t.Errorf("read data does not match before writing it to db")
		}
	}

}
