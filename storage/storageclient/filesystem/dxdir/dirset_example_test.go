package dxdir

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"path/filepath"
	"reflect"
)

var exampleDirSetDir = tempDir("example")

// ExampleDirSet shows an example usage of DirSet
func ExampleDirSet() {
	// initialize
	ds, err := NewDirSet(exampleDirSetDir, newExampleWal())
	path := randomDxPath(2)
	entry, err := ds.NewDxDir(path)
	if err != nil {
		fmt.Println(err)
	}
	// create a random metadata and update
	newMeta := randomMetadata()
	// note the DxPath field is not updated
	newMeta.DxPath = path
	err = entry.UpdateMetadata(*newMeta)
	if err != nil {
		fmt.Println(err)
	}
	// Close the entry
	err = entry.Close()
	if err != nil {
		fmt.Println(err)
	}
	// Reopen the entry
	newEntry, err := ds.Open(path)
	if err != nil {
		fmt.Println(err)
	}
	if !reflect.DeepEqual(*newEntry.metadata, *newMeta) {
		fmt.Printf("After open, metadata not equal: \n\tExpect %+v\n\tGot %+v", newMeta, newEntry.metadata)
	}
	// output:
}

// newExampleWal create a new wal for the example
func newExampleWal() *writeaheadlog.Wal {
	wal, txns, err := writeaheadlog.New(filepath.Join(exampleDirSetDir, "example.wal"))
	if err != nil {
		fmt.Println(err)
	}
	for _, txn := range txns {
		err := txn.Release()
		if err != nil {
			panic(err)
		}
	}
	return wal
}
