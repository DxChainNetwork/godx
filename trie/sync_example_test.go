package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
)

func ExampleSync() {
	// Create a random trie to copy
	srcDb, srcTrie, _ := makeTestTrie()

	// Create a destination trie and sync with the scheduler
	diskdb := ethdb.NewMemDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	queue := append([]common.Hash{}, sched.Missing(1)...)
	for len(queue) > 0 {
		results := make([]SyncResult, len(queue))
		for i, hash := range queue {
			data, err := srcDb.Node(hash)
			if err != nil {
				fmt.Printf("failed to retrieve node data for %x: %v\n", hash, err)
			}
			results[i] = SyncResult{hash, data}
		}
		if _, index, err := sched.Process(results); err != nil {
			fmt.Printf("failed to process result #%d: %v\n", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			fmt.Printf("failed to commit data #%d: %v\n", index, err)
		}
		queue = append(queue[:0], sched.Missing(1)...)
	}
	// Output:
}
