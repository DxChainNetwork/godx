package trie

import (
	"bytes"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
	"io/ioutil"
	"os"
	"testing"
)

var testShortNode = &shortNode{Key: []byte{10, 9, 16}, Val: valueNode("passed")}
var testFullNode = &fullNode{Children: [17]node{
	nil, nil, nil, &shortNode{Key: []byte{9, 9, 15}, Val: valueNode("test")}, nil, nil, nil, nil,
	nil, nil, nil, nil, nil, nil, nil, nil, valueNode("tested"),
}}
var testValueNode = valueNode("value")
var byteData = [][]byte{
	{},
	{12, 32, 12, 14},
	{123, 32, 12, 51, 8},
}

func TestDatabase_NewDatabaseWithCache(t *testing.T) {
	rawDb := ethdb.NewMemDatabase()
	cache := 1024
	db := NewDatabaseWithCache(rawDb, cache)
	if db.diskdb != rawDb {
		t.Errorf("Created database does not point to wanted ethdb. wanted: %p, got: %p", rawDb, db.diskdb)
	}
	if db.cleans.Capacity() != 512*1024*cache {
		t.Errorf("DB clean cache capacity does not match. Wanted: %d, Got: %d.", 512*1024*cache, db.cleans.Capacity())
	}
}

func TestDatabase_NewDatabase(t *testing.T) {
	db := NewDatabase(ethdb.NewMemDatabase())
	if db.cleans != nil {
		t.Errorf("NewDatabase does not return a database with 0 clean cache. Got %d.", db.cleans.Capacity())
	}
}

func TestDatabase_DiskDB(t *testing.T) {
	dirname, err := ioutil.TempDir(os.TempDir(), "ethdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	leveldb, err := ethdb.NewLDBDatabase(dirname, 0, 0)
	if err != nil {
		panic("failed to create a leveldb: " + err.Error())
	}
	tests := []ethdb.Database{
		leveldb,
		ethdb.NewMemDatabase(),
		ethdb.NewTable(ethdb.NewMemDatabase(), "test-"),
	}
	for _, test := range tests {
		db := NewDatabase(test)
		if db.DiskDB() != test {
			t.Errorf("DiskDB does not return the input database. Want: <%p>, got: <%p>", test, db.DiskDB())
		}
	}
}

func TestDatabase_Insert(t *testing.T) {
	rlpStr, _ := rlp.EncodeToBytes(testFullNode)
	hash := crypto.Keccak256(rlpStr)
	db := NewDatabase(ethdb.NewMemDatabase())
	tests := []node{
		testShortNode,
		testFullNode,
		&shortNode{Key: []byte{1, 2, 3}, Val: hashNode(hash)},
		testValueNode,
	}
	for i, test := range tests {
		rlpStr, _ = rlp.EncodeToBytes(test)
		hash = crypto.Keccak256(crypto.Keccak256(rlpStr))
		db.insert(common.BytesToHash(hash), rlpStr, test)
		if len(db.dirties) != i+2 {
			t.Errorf("Size of db dirties does not match. Want: %d, Got: %d.", i+2, len(db.dirties))
		}
	}
}

func TestDatabase_InsertBlob(t *testing.T) {
	db := NewDatabase(ethdb.NewMemDatabase())
	tests := [][]byte{
		{},
		{12, 32, 20, 19},
		{12, 31, 20, 192, 129},
	}
	for _, test := range tests {
		db.InsertBlob(crypto.Keccak256Hash(test), test)
	}
	for _, test := range tests {
		if dbData, ok := db.dirties[crypto.Keccak256Hash(test)].node.(rawNode); !ok || !bytes.Equal(dbData, test) {
			t.Errorf("Inserted blob does not match the original. Wanted %x, got %x", test, dbData)
		}
	}
}

func TestDatabase_NodeDirty(t *testing.T) {
	db := NewDatabase(ethdb.NewMemDatabase())
	dirties := []node{testShortNode, testFullNode, testValueNode}

	for _, dirty := range dirties {
		rlpStr, _ := rlp.EncodeToBytes(dirty)
		hash := crypto.Keccak256Hash(rlpStr)
		db.insert(hash, rlpStr, dirty)
	}

	for _, dirty := range dirties {
		hash := makeHash(dirty)
		if node := db.dirties[hash]; node == nil {
			t.Errorf("Cannot find hash %x in db.dirties", hash)
		}
	}
}

func TestDatabase_NodeCleans(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)
	for _, clean := range byteData {
		err := db.cleans.Set(string(crypto.Keccak256(clean)), clean)
		if err != nil {
			t.Errorf("Cannot set cleans value: %x", clean)
		}
	}

	for _, clean := range byteData {
		val, err := db.cleans.Get(string(crypto.Keccak256(clean)))
		if err != nil {
			t.Errorf("Cannot get cleans value: %x", clean)
		}
		if !bytes.Equal(val, clean) {
			t.Errorf("Cleans stores wrong data. Wanted %x, got %x", clean, val)
		}
	}
}

func TestDatabase_NodeDiskDb(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)
	for _, dbData := range byteData {
		err := db.diskdb.Put(crypto.Keccak256(dbData), dbData)
		if err != nil {
			t.Errorf("Cannot set cleans value: %x", dbData)
		}
	}

	for _, dbData := range byteData {
		val, err := db.diskdb.Get(crypto.Keccak256(dbData))
		if err != nil {
			t.Errorf("Cannot get cleans value: %x", dbData)
		}
		if !bytes.Equal(val, dbData) {
			t.Errorf("Cleans stores wrong data. Wanted %x, got %x", dbData, val)
		}
	}
}

func TestDatabase_NodeIntegration(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)

	rlpStr, _ := rlp.EncodeToBytes(testFullNode)
	hash := crypto.Keccak256Hash(rlpStr)

	db.insert(hash, rlpStr, testFullNode)
	err := db.cleans.Set(string(hash.Bytes()), []byte{1, 2, 3})
	if err != nil {
		t.Errorf("Cannot set clean value %s", err.Error())
	}
	err = db.diskdb.Put(hash.Bytes(), []byte{3, 2, 1})
	if err != nil {
		t.Errorf("Cannot set db value %s", err.Error())
	}

	data, err := db.Node(hash)
	if !bytes.Equal(data, []byte{1, 2, 3}) {
		t.Errorf("Node does not give correct data. Wanted %x, got %x", []byte{1, 2, 3}, data)
	}
}

func TestDatabase_Nodes(t *testing.T) {
	db := NewDatabase(ethdb.NewMemDatabase())
	dirties := []node{testShortNode, testFullNode, testValueNode}
	inserted := make(map[common.Hash]node)

	for _, dirty := range dirties {
		rlpStr, _ := rlp.EncodeToBytes(dirty)
		hash := crypto.Keccak256Hash(rlpStr)
		db.insert(hash, rlpStr, dirty)
		inserted[hash] = dirty
	}

	nodes := db.Nodes()
	if len(nodes) < len(inserted) {
		t.Errorf("Node return node number smaller than inserted. Wanted %d, got %d", len(inserted), len(nodes))
	}
	for _, node := range nodes {
		if retrieved := inserted[node]; retrieved == nil {
			t.Errorf("Node does not return a valid hash: %x", node)
		}
	}
}

func TestDatabase_Reference(t *testing.T) {
	db := NewDatabase(ethdb.NewMemDatabase())
	trie, _ := New(common.Hash{}, db)
	for _, entry := range testData {
		trie.Update([]byte(entry.k), []byte(entry.v))
	}
	hash, err := trie.Commit(nil)
	if err != nil {
		panic("Cannot commit the trie")
	}

	insertedData := []byte{1, 2, 3, 4, 5, 6, 7}
	dataHash := crypto.Keccak256Hash(insertedData)
	db.InsertBlob(dataHash, insertedData)

	origParPar, origParChildLen := db.dirties[hash].parents, len(db.dirties[hash].children)
	origChildPar, origChildChildLen := db.dirties[dataHash].parents, len(db.dirties[dataHash].children)

	db.Reference(dataHash, hash)

	newParent, newChild := db.dirties[hash], db.dirties[dataHash]
	if origParPar != newParent.parents || origChildChildLen != len(newChild.children) {
		t.Errorf("Reference effected unrelated field: parent.parents: %d -> %d, len(child.Children): %d -> %d", origParPar, newParent.parents, origChildChildLen, len(newChild.children))
	}
	if origParChildLen != len(newParent.children)-1 {
		t.Errorf("Reference does not increment the length of parent.children. Wanted %d, got %d", origParChildLen+1, len(newParent.children))
	}
	if origChildPar != newChild.parents-1 {
		t.Errorf("Reference does not increment the child.parents. Wanted %d, got %d", origChildPar+1, newChild.parents)
	}
}

func TestDatabase_Dereference(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)
	tests := []*struct {
		k common.Hash
		v []byte
	}{
		{v: []byte{1, 2, 3, 4, 5}},
		{v: []byte{3, 2, 1}},
		{v: []byte{9, 8, 7, 6, 5, 4, 3, 2, 1}},
	}
	for _, test := range tests {
		hash := crypto.Keccak256Hash(test.v)
		test.k = hash
		db.InsertBlob(hash, test.v)
	}
	// Construct a reference chain a->b->c
	db.Reference(tests[0].k, tests[1].k)
	db.Reference(tests[1].k, tests[2].k)

	// Dereference c will dereference a and b
	db.Dereference(tests[2].k)

	for _, test := range tests {
		if db.dirties[test.k] != nil {
			t.Errorf("Node %x should have been deleted from dirties.", test.k)
		}
	}
}

func TestDatabase_Cap(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)
	tests := []*struct {
		k common.Hash
		v []byte
	}{
		{v: []byte{1, 2, 3, 4, 5}},
		{v: []byte{3, 2, 1}},
		{v: []byte{9, 8, 7, 6, 5, 4, 3, 2, 1}},
	}
	for _, test := range tests {
		hash := crypto.Keccak256Hash(test.v)
		test.k = hash
		db.InsertBlob(hash, test.v)
	}
	sizes := []common.StorageSize{200, 100, 50, 0}
	for _, size := range sizes {
		err := db.Cap(size)
		if err != nil {
			panic("Cannot cap the db:" + err.Error())
		}
		dbSize, _ := db.Size()
		if dbSize > size {
			t.Errorf("After calling Cap, db dirty size is larger than expected. Want: <%s, Got: %s.", size, dbSize)
		}
	}
	// After calling Cap(0), all inserted entries should not be in db.dirties.
	for _, test := range tests {
		if db.dirties[test.k] != nil {
			t.Errorf("After calling Cap, entry still in db: %x -> %x", test.k, test.v)
		}
	}
}

func TestDatabase_Commit(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)
	trie, err := New(common.Hash{}, db)
	if err != nil {
		panic("Cannot create a trie")
	}
	for _, data := range testData {
		trie.Update([]byte(data.k), []byte(data.v))
	}
	hash, err := trie.Commit(nil)
	if err != nil {
		panic("Cannot commit trie")
	}
	// Commit db. All entries should have been flushed to diskdb
	if err = db.Commit(hash, false); err != nil {
		panic("Cannot commit db")
	}
	if db.dirties[hash] != nil {
		t.Errorf("After db commit, the node still hasn't been flushed to db %x", hash)
	}
}

func TestDatabase_Size(t *testing.T) {
	db := NewDatabaseWithCache(ethdb.NewMemDatabase(), 1024)
	tests := []*struct {
		k common.Hash
		v []byte
	}{
		{v: []byte{1, 2, 3, 4, 5}},
		{v: []byte{3, 2, 1}},
		{v: []byte{9, 8, 7, 6, 5, 4, 3, 2, 1}},
	}
	size, preimageSize := db.Size()
	for _, test := range tests {
		hash := crypto.Keccak256Hash(test.v)
		test.k = hash
		db.InsertBlob(hash, test.v)
		newSize, newPreimageSize := db.Size()
		if newSize <= size {
			t.Errorf("After insertion, the size decreases as unexpected")
		}
		if preimageSize != newPreimageSize {
			t.Errorf("Insertion changed the size of preimage: %s -> %s", preimageSize, newPreimageSize)
		}
		size, preimageSize = newSize, newPreimageSize
	}
	for _, test := range tests {
		db.insertPreimage(test.k, test.v)
		newSize, newPreimageSize := db.Size()
		if newSize != size {
			t.Errorf("After inserting preimage, the size remains the same")
		}
		if newPreimageSize <= preimageSize {
			t.Errorf("After inserting preimage, the preimage size does not grow larger")
		}
		size, preimageSize = newSize, newPreimageSize
	}
	for _, test := range tests {
		err := db.Commit(test.k, false)
		if err != nil {
			panic(fmt.Sprintf("Cannot commit db for node %x %x", test.k, test.v))
		}
		newSize, newPreimageSize := db.Size()
		if newSize > size {
			t.Errorf("After committing, Size for dirty grows: %s -> %s", size, newSize)
		}
		size, preimageSize = newSize, newPreimageSize
	}
	if size != common.StorageSize(0) || preimageSize != common.StorageSize(0) {
		t.Errorf("After cap, db Size does not return 0. Expected 0.00 B/0.00 B, having %s/%s", size, preimageSize)
	}
}

func makeHash(i interface{}) common.Hash {
	rlpStr, _ := rlp.EncodeToBytes(i)
	hash := crypto.Keccak256Hash(rlpStr)
	return hash
}
