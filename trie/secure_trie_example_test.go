package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
)

// SecureTrie has almost the same behaviour as Trie
func ExampleSecureTrie() {
	triedb := NewDatabase(ethdb.NewMemDatabase())
	trie, _ := NewSecure(common.Hash{}, triedb, 0)
	for _, entry := range testData {
		trie.Update([]byte(entry.k), []byte(entry.v))
	}

	// Insert
	trie.Update([]byte("does"), []byte("good"))

	// Get
	val := trie.Get([]byte("does"))
	fmt.Printf("Retrieved key value pair: %s -> %s\n", "does", string(val))

	// Delete
	trie.Delete([]byte("does"))
	if val := trie.Get([]byte("does")); val == nil {
		fmt.Printf("Successfully deleted the key \"does\"\n")
	}

	// Commit
	hash, err := trie.Commit(nil)
	if err != nil {
		fmt.Println("Commit return an error:", err.Error())
		return
	}

	// Create a new trie same as the original trie
	newTrie, err := NewSecure(hash, triedb, 0)
	if err != nil {
		fmt.Println(err.Error())
	}
	val = newTrie.Get([]byte("dog"))
	fmt.Printf("Retrieved from new trie: %s -> %s", "dog", val)
	// Output:
	// Retrieved key value pair: does -> good
	// Successfully deleted the key "does"
	// Retrieved from new trie: dog -> puppy
}
