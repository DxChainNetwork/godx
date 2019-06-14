package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAddBatchNormal(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	sm := newTestStorageManager(t, "", newDisrupter())
	// Create one folder
	path := randomFolderPath(t, "")
	size := uint64(1 << 25)
	if err := sm.AddStorageFolder(path, size); err != nil {
		t.Fatal(err)
	}
	// insert three sectors
	type expect struct {
		root  common.Hash
		count int
		data  []byte
	}
	sectors := size / storage.SectorSize
	expects := make([]expect, 0, sectors)
	var lock sync.Mutex
	for i := 0; i != int(sectors); i++ {
		// Add 8 sectors
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatal(err)
		}
		expects = append(expects, expect{
			data:  data,
			root:  root,
			count: 1,
		})
	}
	// Randomly select 3 as a batch each time
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	for numBatch := 0; numBatch != 10; numBatch++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch := make([]common.Hash, 0, 3)
			selectedIndex := rand.Perm(int(sectors))[:3]
			lock.Lock()
			for _, i := range selectedIndex {
				batch = append(batch, expects[i].root)
			}
			lock.Unlock()
			if err := sm.AddSectorBatch(batch); err != nil {
				errChan <- err
			}
			lock.Lock()
			for _, i := range selectedIndex {
				prev := expects[i]
				prev.count++
				expects[i] = prev
			}
			lock.Unlock()
			fmt.Println("finished")
		}()
	}
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()
	select {
	case <-waitChan:
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatalf("time out")
	}
	for _, expect := range expects {
		if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkFoldersHasExpectedSectors(sm, int(sectors)); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, time.Second)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}
