package newstoragemanager

import (
	"bytes"
	"testing"
)

// TestDatabase_getSectorSalt test database.getSectorSalt
func TestDatabase_getSectorSalt(t *testing.T) {
	db, err := openDB(tempDir(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	salt, err := db.getSectorSalt()
	if err != nil {
		t.Fatal(err)
	}
	salt2, err := db.getSectorSalt()
	if err != nil {
		t.Fatal(err)
	}
	if salt != salt2 {
		t.Errorf("salt not equal. Prev %x, Later %x", salt, salt2)
	}
	saltFromDB, err := db.lvl.Get([]byte(sectorSaltKey), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(salt[:], saltFromDB) {
		t.Errorf("salt from db not equal. Got %x, Expect %x", saltFromDB, salt)
	}
}
