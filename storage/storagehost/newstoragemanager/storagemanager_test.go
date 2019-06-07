package newstoragemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "storagemanager", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v", path))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return path
}

// newTestDatabase create the database for testing
func newTestDatabase(t *testing.T, extra string) (db *database) {
	var dbPath string
	if len(extra) != 0 {
		dbPath = tempDir(t.Name(), extra, databaseFileName)
	} else {
		dbPath = tempDir(t.Name(), databaseFileName)
	}

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
