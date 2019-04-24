package ethdb

import (
	"testing"
)

/**
Local constructor for dbTable, create and return instance
*/
func createTable(t *testing.T, dat []struct {
	key   string
	value string
}, prefix string) (Database, *LDBDatabase) {
	prefixdat := make([]struct {
		key   string
		value string
	}, len(dat))
	copy(prefixdat, dat)
	if prefixdat != nil {
		for i, _ := range prefixdat {
			prefixdat[i].key = prefix + prefixdat[i].key
		}
	}

	db, err := createdb(prefixdat)
	if err != nil {
		t.Errorf("database cannot be created")
	}
	DB := NewTable(db, prefix)
	return DB, db
}

/**
Local constructor for memTable, create and return instance
*/
func creatememTable(t *testing.T, size int, dat []struct {
	key   string
	value string
}, prefix string) Database {
	prefixdat := make([]struct {
		key   string
		value string
	}, len(dat))
	copy(prefixdat, dat)
	if prefixdat != nil {
		for i, _ := range prefixdat {
			prefixdat[i].key = prefix + prefixdat[i].key
		}
	}
	mem := creatememdb(t, size, prefixdat)
	DB := NewTable(mem, prefix)
	return DB
}

/**
Check if the table the same as the expectation
*/
func checkTable(t *testing.T, db Database, expected []struct {
	key   string
	value string
}) {
	for _, it := range expected {
		// check if the database contain the key
		if has, err := db.Has([]byte(it.key)); err != nil {
			t.Errorf("cannot call function 'has'")
		} else if !has {
			t.Errorf("table does not contain the expected element")
		}

		// check if the value the same corresponding the key
		if val, err := db.Get([]byte(it.key)); err != nil {
			t.Errorf("error caused by calling the 'Get' function")
		} else if string(val) != it.value {
			t.Errorf("the value are not the same as the expected")
		}
	}
}

/**
test put element into two type of table: memtype and dbtype with prefix
*/
func TestPutTable(t *testing.T) {
	db, ldb := createTable(t, nil, "#T_")
	mem := creatememTable(t, 0, nil, "#M_")
	for _, it := range testdata {
		if err := db.Put([]byte(it.key), []byte(it.value)); err != nil {
			t.Errorf("put element to table fail")
		}
		if err := mem.Put([]byte(it.key), []byte(it.value)); err != nil {
			t.Errorf("put element to table fail")
		}
	}

	expected := []struct {
		key   string
		value string
	}{
		{"key03", "val03"},
		{"key04", "val04"},
		{"key05", "val05"},
		{"key01", "val100"},
		{"key02", "val200"},
	}

	checkTable(t, db, expected)
	checkTable(t, mem, expected)

	cleardb(t, ldb)
	db.Close()
	mem.Close()
}

/**
Test delete table: try to delete existing and non-existing key
*/
func TestDeleteTable(t *testing.T) {
	db, ldb := createTable(t, testdata, "#T_")
	mem := creatememTable(t, 0, testdata, "#M_")

	non_exist := []string{"key001", "key002", "key003", "!@#$%^&&*(()"}
	for _, it := range non_exist {
		if err := db.Delete([]byte(it)); err != nil {
			t.Errorf("delet non-existing element cause error ")
		}
		if err := mem.Delete([]byte(it)); err != nil {
			t.Errorf("delet non-existing element cause error ")
		}
	}

	expected := []struct {
		key   string
		value string
	}{
		{"key03", "val03"},
		{"key04", "val04"},
		{"key05", "val05"},
		{"key01", "val100"},
		{"key02", "val200"},
	}

	checkTable(t, db, expected)
	checkTable(t, mem, expected)
	cleardb(t, ldb)
	db.Close()
	mem.Close()
}

/**
Test writing batch:
1. delete normal, non-existing value using batch
2. put into batch and then reset batch
3. using bath add new element
*/
func TestTableBatch(t *testing.T) {
	db, ldb := createTable(t, testdata, "#T_")
	mem := creatememTable(t, 0, testdata, "#M_")

	deldata := []string{"key05", "key04", "default"}
	dirtydata := []struct {
		key   string
		value string
	}{
		{"dirty01", "dirtyval1"},
		{"dirty02", "dirtyval2"},
	}

	// Delete data and write to database
	dbbatch := NewTableBatch(ldb, "#T_")
	//dbbatch := db.NewBatch()
	membatch := mem.NewBatch()
	//membatch := NewTableBatch(mem, "#M_")
	for _, it := range deldata {
		if err := dbbatch.Delete([]byte(it)); err != nil {
			t.Errorf("delete fail")
		}
		if err := membatch.Delete([]byte(it)); err != nil {
			t.Errorf("delete fail")
		}
	}

	if err := dbbatch.Write(); err != nil {
		t.Errorf("write first batch fail")
	}

	if err := membatch.Write(); err != nil {
		t.Errorf("write first batch fail")
	}

	// add data and reset
	for _, it := range dirtydata {
		if err := dbbatch.Put([]byte(it.key), []byte(it.value)); err != nil {
			t.Errorf("cause error while writing in batch")
		}
		if err := membatch.Put([]byte(it.key), []byte(it.value)); err != nil {
			t.Errorf("cause error while writing in batch")
		}
	}

	dbbatch.Reset()
	membatch.Reset()

	if dbbatch.ValueSize() != 0 {
		t.Errorf("batch size should reset to 0")
	}
	if membatch.ValueSize() != 0 {
		t.Errorf("batch size should reset to 0")
	}

	// add a new element and write into batch
	if err := dbbatch.Put([]byte("newkey"), []byte("newval")); err != nil {
		t.Errorf("cannot put into batch")
	}

	if err := dbbatch.Write(); err != nil {
		t.Errorf("write second batch fail")
	}

	if err := membatch.Put([]byte("newkey"), []byte("newval")); err != nil {
		t.Errorf("cannot put into batch")
	}

	if err := membatch.Write(); err != nil {
		t.Errorf("write second batch fail")
	}

	expected := []struct {
		key   string
		value string
	}{
		{"key01", "val100"},
		{"key02", "val200"},
		{"key03", "val03"},
		{"newkey", "newval"},
	}

	checkTable(t, db, expected)
	checkTable(t, mem, expected)
	cleardb(t, ldb)
}
