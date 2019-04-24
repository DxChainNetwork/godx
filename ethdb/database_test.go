package ethdb

import (
	"bytes"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"testing"
)

/**
Factory method: create a database instance
*/
func createdb(pairs []struct {
	key   string
	value string
}) (*LDBDatabase, error) {
	db, _ := leveldb.OpenFile(workdb, nil)
	if pairs != nil {
		for _, it := range pairs {
			_ = db.Put([]byte(it.key), []byte(it.value), nil)
		}
	}
	_ = db.Close()
	return NewLDBDatabase(workdb, 16, 16)
}

/**
clear the data and remove the database dir
*/
func cleardb(t *testing.T, db *LDBDatabase) {
	dir := db.Path()
	db.Close()
	_ = os.RemoveAll(dir)
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("UnAuthorize to delete database")
	}
}

/**
check if the content in the database the same as the expected
*/
func check(t *testing.T, db *LDBDatabase, ans map[string]string) {
	db.Close()
	ldb, _ := leveldb.OpenFile(workdb, nil)
	itr := ldb.NewIterator(nil, nil)
	var itmCounter int
	for itr.Next() {
		if val, ok := ans[string(itr.Key()[:])]; ok {
			if string(itr.Value()[:]) != val {
				t.Errorf("value does not consistent with the expected")
			}
		} else {
			t.Errorf("cannot find expected key in the databse")
		}
		itmCounter++
	}

	if len(ans) != itmCounter {
		t.Errorf("number of content does not consistent")
	}
}

/**
Test constructor :
(Unhandle overflows int.(input error))
*/
func TestCreatedb(t *testing.T) {

	var tmpdbs = []struct {
		cache   int
		handles int
	}{
		{0, 0},     // test 0
		{-23, -91}, // test negatice
		{14, 10},   // test smaller number on both
		{16, 19},   // test normal
		{16, 16},   // test conner on 16
		{19, 14},   // test diff cache and handles
		{14, 19},   // test diff cache and handles
	}

	for _, tt := range tmpdbs {
		db, err := NewLDBDatabase(emptydb, tt.cache, tt.handles)
		if err != nil {
			t.Errorf("Handled excpetion, but not in specification")
		}
		if _, err := os.Stat(emptydb); err != nil {
			t.Errorf("Creation fail")
		}
		cleardb(t, db)
	}
}

/**
Test the Path function: test if databse can be created in a defined path
*/
func TestPath(t *testing.T) {
	db, err := createdb(nil)
	if err != nil {
		t.Errorf("database cannot be created or accessed")
	}
	if db.Path() != workdb {
		t.Error("database cannot be created using a specified dir", db.Path())
	}
	cleardb(t, db)
}

/**
Test the putter function
1. if remove the duplicate
2. if put to empty db
3. normal case
*/
func TestPut(t *testing.T) {
	db, err := createdb(nil)

	if err != nil {
		t.Errorf("database cannot be created or accessed")
	}

	for _, it := range testdata {
		err := db.Put([]byte(it.key), []byte(it.value))
		if err != nil {
			t.Errorf(err.Error())
		}
	}

	expected := map[string]string{"key01": "val100", "key02": "val200", "key03": "val03", "key04": "val04", "key05": "val05"}
	check(t, db, expected)
	cleardb(t, db)
}

/**
Test 'Hash' operation.
1. existing normal & duplicate element
2. non-existing element
3. calling has from a empty database

*/
func TestHas(t *testing.T) {
	var nondata = []string{"key", "key00", "key001", "!@#$%^&"}
	db, err := createdb(nil)

	for _, it := range nondata {
		if ok, err := db.Has([]byte(it)); err != nil {
			t.Errorf(err.Error())
		} else if ok {
			t.Errorf("data should not exist in database")
		}
	}

	db.Close() // db must be close before run the new database

	db, err = createdb(testdata)
	if err != nil {
		t.Errorf("cannot create database")
	}
	for _, it := range testdata {
		if ok, err := db.Has([]byte(it.key)); !ok || err != nil {
			t.Errorf("can not check existing element using 'Has'")
		}
	}

	for _, it := range nondata {
		if ok, err := db.Has([]byte(it)); err != nil {
			t.Errorf(err.Error())
		} else if ok {
			t.Errorf("data should not exist in database")
		}
	}
	cleardb(t, db)
}

/**
Test Get operation
1. existing key
2. nonexisting key
3. empty db
*/
func TestGet(t *testing.T) {
	var nondata = []string{"key", "key00", "key001", "!@#$%^&"}
	db, _ := createdb(nil)

	for _, it := range nondata {
		if val, err := db.Get([]byte(it)); err == nil {
			t.Errorf("data should not exist in database")
		} else if val != nil {
			t.Errorf("data should not exist in database")
		}
	}
	db.Close()

	db, err := createdb(testdata)
	if err != nil {
		t.Errorf("cannot create database")
	}

	var expectKey = []string{
		"key01", "key02", "key03", "key04", "key05",
	}

	ans := make(map[string]string)
	for _, it := range expectKey {
		if val, err := db.Get([]byte(it)); err == nil {
			ans[it] = string(val[:])
		}
	}
	check(t, db, ans)
	cleardb(t, db)
}

/**
Test delete:
1. delete element in empty database
2. normal case
3. delete twice
*/
func TestDelete(t *testing.T) {
	db, _ := createdb(nil)
	var nondata = []string{"key", "key00", "key001", "!@#$%^&"}
	// delete non-existing elemnt would appear any error
	for _, it := range nondata {
		if err := db.Delete([]byte(it)); err != nil {
			t.Errorf(err.Error())
		}
	}
	db.Close()

	var data = []struct {
		key   string
		value string
	}{
		{"key01", "val01"},
		{"key02", "val02"},
		{"key03", "val03"},
		{"key04", "val04"},
		{"key05", "val05"},
		{"key01", "val100"},
		{"key02", "val200"},
	}

	db, _ = createdb(data)
	var delKey = []string{"key01", "key02", "key002", "val01"}
	for _, it := range delKey {
		if err := db.Delete([]byte(it)); err != nil {
			t.Errorf(err.Error())
		}
	}
	var expected = map[string]string{"key03": "val03", "key04": "val04", "key05": "val05"}
	check(t, db, expected)
	cleardb(t, db)
}

/**
Test iterator:
1. test get iterator from the empty database
2. test get iterator and loop through the database
*/
func TestIterator(t *testing.T) {
	// testing the empty database
	db, _ := createdb(nil)
	itr := db.NewIterator()
	for itr.Next() {
		t.Errorf("database should contain nothing")
	}

	db.Close()
	// testing the database with value

	db, _ = createdb(testdata)
	itr = db.NewIterator()
	var expected = make(map[string]string)
	for itr.Next() {
		expected[string(itr.Key())] = string(itr.Value())
	}

	check(t, db, expected)
	cleardb(t, db)
}

/**
Test prefix Iterator:
1. test prefix with key
2. test prefix with pre
3. test empty string
4. test nil
*/
func TestPrefixItr(t *testing.T) {
	// testing the database with value
	var data = []struct {
		key   string
		value string
	}{
		{"key01", "val01"},
		{"key02", "val02"},
		{"key03", "val03"},
		{"key04", "val04"},
		{"key01", "val100"},
		{"key02", "val200"},
		{"prekey01", "val000"},
		{"prekey02", "val111"},
		{"prekey03", "val222"},
		{"pr", "val???"},
	}
	db, _ := createdb(data)
	itrkey := db.NewIteratorWithPrefix([]byte("key"))
	itrpre := db.NewIteratorWithPrefix([]byte("pre"))
	itremp := db.NewIteratorWithPrefix([]byte(""))
	itrnoexist := db.NewIteratorWithPrefix([]byte("w"))

	anskey := map[string]string{
		"key01": "val100",
		"key02": "val200",
		"key03": "val03",
		"key04": "val04",
	}

	anspre := map[string]string{
		"prekey01": "val000",
		"prekey02": "val111",
		"prekey03": "val222",
	}

	ansemp := map[string]string{
		"key01":    "val100",
		"key02":    "val200",
		"key03":    "val03",
		"key04":    "val04",
		"prekey01": "val000",
		"prekey02": "val111",
		"prekey03": "val222",
		"pr":       "val???",
	}

	ansnoexist := map[string]string{}

	itrs := []iterator.Iterator{itrkey, itrpre, itremp, itrnoexist}
	ans := []map[string]string{anskey, anspre, ansemp, ansnoexist}

	for i, _ := range itrs {
		counter := 0
		for itrs[i].Next() {
			if val, ok := ans[i][string(itrs[i].Key())]; ok {
				if val != string(itrs[i].Value()) {
					t.Errorf("value does not as consistent as the expected")
				}
			}
			counter++
		}
		if counter != len(ans[i]) {
			t.Errorf("number of element is not the same as the expected")
		}
	}
	cleardb(t, db)
}

/**
Test close and then get instance
*/
func TestInstance(t *testing.T) {
	db, _ := createdb(nil)
	db.Close()
	instdb := db.LDB()
	if instdb == nil {
		t.Errorf("cannot get levelDB instance")
	}
	cleardb(t, db)
}

/**
Test the operation when write in batch
*/
func TestBatch(t *testing.T) {
	db, _ := createdb([]struct {
		key   string
		value string
	}{{"default", "default"}})
	batch := db.NewBatch()

	expected := map[string]string{
		"key01": "val100",
		"key02": "val200",
		"key03": "val03",
	}

	for _, it := range testdata {
		if err := batch.Put([]byte(it.key), []byte(it.value)); err != nil {
			t.Errorf("cause error while writing in batch")
		}
	}

	var deldata = []string{"key05", "key04", "default"}
	for _, it := range deldata {
		if err := batch.Delete([]byte(it)); err != nil {
			t.Errorf("delete fail")
		}
	}

	if err := batch.Write(); err != nil {
		t.Errorf("write first batch fail")
	}

	data := []struct {
		key   string
		value string
	}{
		{"dirty01", "dirtyval1"},
		{"dirty02", "dirtyval2"},
	}

	for _, it := range data {
		if err := batch.Put([]byte(it.key), []byte(it.value)); err != nil {
			t.Errorf("cause error while writing in batch")
		}
	}

	batch.Reset()
	if batch.ValueSize() != 0 {
		t.Errorf("batch size should reset to 0")
	}

	if err := batch.Put([]byte("newkey"), []byte("newval")); err != nil {
		t.Errorf("cannot put into batch")
	}

	if err := batch.Write(); err != nil {
		t.Errorf("write second batch fail")
	}

	expected["newkey"] = "newval"
	check(t, db, expected)
	cleardb(t, db)
}

func TestMeter(t *testing.T) {
	db, err := createdb(nil)
	if err != nil {
		t.Errorf("cannot create database")
	}
	db.Meter("prefix")
	cleardb(t, db)
}

/**
/////////////////////////////////////Original test cases///////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////////////////////
*/

func newTestLDB() (*LDBDatabase, func()) {
	dirname, err := ioutil.TempDir(os.TempDir(), "ethdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := NewLDBDatabase(dirname, 0, 0)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirname)
	}
}

var test_values = []string{"", "a", "1251", "\x00123\x00"}

func TestLDB_PutGet(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	testPutGet(db, t)
}

func TestMemoryDB_PutGet(t *testing.T) {
	testPutGet(NewMemDatabase(), t)
}

func testPutGet(db Database, t *testing.T) {
	t.Parallel()

	for _, k := range test_values {
		err := db.Put([]byte(k), nil)
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for _, k := range test_values {
		data, err := db.Get([]byte(k))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if len(data) != 0 {
			t.Fatalf("get returned wrong result, got %q expected nil", string(data))
		}
	}

	_, err := db.Get([]byte("non-exist-key"))
	if err == nil {
		t.Fatalf("expect to return a not found error")
	}

	for _, v := range test_values {
		err := db.Put([]byte(v), []byte(v))
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for _, v := range test_values {
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte(v)) {
			t.Fatalf("get returned wrong result, got %q expected %q", string(data), v)
		}
	}

	for _, v := range test_values {
		err := db.Put([]byte(v), []byte("?"))
		if err != nil {
			t.Fatalf("put override failed: %v", err)
		}
	}

	for _, v := range test_values {
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte("?")) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	for _, v := range test_values {
		orig, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		orig[0] = byte(0xff)
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte("?")) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	for _, v := range test_values {
		err := db.Delete([]byte(v))
		if err != nil {
			t.Fatalf("delete %q failed: %v", v, err)
		}
	}

	for _, v := range test_values {
		_, err := db.Get([]byte(v))
		if err == nil {
			t.Fatalf("got deleted value %q", v)
		}
	}
}

func TestLDB_ParallelPutGet(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	testParallelPutGet(db, t)
}

func TestMemoryDB_ParallelPutGet(t *testing.T) {
	testParallelPutGet(NewMemDatabase(), t)
}

func testParallelPutGet(db Database, t *testing.T) {
	const n = 8
	var pending sync.WaitGroup

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			err := db.Put([]byte(key), []byte("v"+key))
			if err != nil {
				panic("put failed: " + err.Error())
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			data, err := db.Get([]byte(key))
			if err != nil {
				panic("get failed: " + err.Error())
			}
			if !bytes.Equal(data, []byte("v"+key)) {
				panic(fmt.Sprintf("get failed, got %q expected %q", []byte(data), []byte("v"+key)))
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			err := db.Delete([]byte(key))
			if err != nil {
				panic("delete failed: " + err.Error())
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			_, err := db.Get([]byte(key))
			if err == nil {
				panic("get succeeded")
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()
}
