package storagemanager

import (
	"fmt"
	"testing"
)

//const name  = iota
func TestNew(t *testing.T) {
	//os.RemoveAll("./testdata/")

	sm, err := New("./testdata/")
	if err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	if err := sm.AddStorageFolder("./testdata/folders123", SectorSize*64); err != nil {
		fmt.Println(err.Error())
	}

	if err := sm.syncConfig(); err != nil {
		fmt.Println(err.Error())
	}

	//spew.Dump(sm.folders)
}

func TestRand(t *testing.T) {

	fmt.Println(SectorSize)
	buildSetting(TST)
	fmt.Println(SectorSize)

}
