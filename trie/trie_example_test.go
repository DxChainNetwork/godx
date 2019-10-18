package trie

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
)

var testData = []struct{ k, v string }{
	{"do", "verb"},
	{"ether", "wookiedoo"},
	{"horse", "stallion"},
	{"shaman", "horse"},
	{"doge", "coin"},
	{"dog", "puppy"},
	{"somethingveryoddindeedthis is", "myothernodedata"},
}

// newTrieWithData returns a trie not committed with input data.
func newTrieWithData(data []struct{ k, v string }) *Trie {
	trie := newEmpty()
	if data == nil || len(data) == 0 {
		// If the data is nil or have length 0, return an empty trie.
		return trie
	}
	for _, entry := range data {
		trie.Update([]byte(entry.k), []byte(entry.v))
	}
	return trie
}

func ExampleTrie_InsertGet() {
	trie := newTrieWithData(testData)
	// Insert a new entry
	trie.Update([]byte("does"), []byte("good"))
	// Retrieve the value
	val := trie.Get([]byte("does"))

	fmt.Printf("Inserted value: %s -> %s\n", "does", string(val))
	// Output:
	// Inserted value: does -> good
}

func ExampleTrie_UpdateGet() {
	trie := newTrieWithData(testData)
	// Update an existing entry
	trie.Update([]byte("dog"), []byte("valueChanged"))
	// Get the value.
	newVal := trie.Get([]byte("dog"))

	fmt.Printf("Updated val: %s -> %s\n", "dog", newVal)
	// Output:
	// Updated val: dog -> valueChanged
}

func ExampleTrie_Delete() {
	trie := newTrieWithData(testData)

	// Delete an existing entry
	trie.Delete([]byte("dog"))

	// Try to find the deleted entry, must return an error
	val, err := trie.TryGet([]byte("dog"))
	if err != nil {
		fmt.Println("TryGet returns an error:", err.Error())
		return
	}
	fmt.Printf("After deletion, have key value pair: %s -> [%s].\n", "dog", val)
	// Output:
	// After deletion, have key value pair: dog -> [].
}

// This ExampleTrie_Commit function covers some underlying logic of the Commit operation.
func ExampleTrie_Commit() {
	// Create a trie1
	trie1 := newTrieWithData(testData)

	// Commit trie1. The root node after commit is a copied cached node of the original root node.
	// But the root's hash is exactly the hash of committed node.
	hash, err := trie1.Commit(nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// Create a new trie with committed data. Note here the root node in trie2 is not the root from trie1.
	// The root of trie2 is a new node with hash.
	trie2, err := New(hash, trie1.db)
	if err != nil {
		fmt.Println("New returns an error:,", err.Error())
		return
	}
	for _, entry := range testData {
		val := trie2.Get([]byte(entry.k))
		fmt.Printf("The new trie got data: %s -> %s\n", entry.k, string(val))
	}
	fmt.Println()

	// Change an entry in the new trie should not affect the original trie1.
	// After node updated, the updated node will have a new flag with a nil hash.
	trie2.Update([]byte("doom"), []byte("catastrophe"))

	// Commit the trie2. After commit, the changed node in the last update which has nil hash will be
	// flushed to disk with a new hash.
	newHash, _ := trie2.Commit(nil)
	if newHash != hash {
		fmt.Println("The new trie have a different hash from the original.")
	}

	val := trie1.Get([]byte("doom"))
	fmt.Printf("After change trie2, trie1 have key value: %s -> [%s].\n", "doom", val)
	// Output:
	// The new trie got data: do -> verb
	// The new trie got data: ether -> wookiedoo
	// The new trie got data: horse -> stallion
	// The new trie got data: shaman -> horse
	// The new trie got data: doge -> coin
	// The new trie got data: dog -> puppy
	// The new trie got data: somethingveryoddindeedthis is -> myothernodedata
	//
	// The new trie have a different hash from the original.
	// After change trie2, trie1 have key value: doom -> [].
}

// Trie is a content based storage. Two tries using the same database will not effect each other
// as long as you get the root correct.
func ExampleTrie_ContentBasedStorage() {
	key := []byte("my key")
	value1 := []byte("value 1")
	value2 := []byte("value 2")
	newValue2 := []byte("value 3")

	db := NewDatabase(ethdb.NewMemDatabase())
	t1, _ := New(common.Hash{}, db)
	t1.TryUpdate(key, value1)
	root1, _ := t1.Commit(nil)

	t2, _ := New(common.Hash{}, db)
	t2.TryUpdate(key, value2)
	root2, _ := t2.Commit(nil)

	recoveredT1, _ := New(root1, db)
	recoveredV1, _ := recoveredT1.TryGet(key)
	fmt.Printf("recovered [value 1], got [%v]\n", string(recoveredV1))

	recoveredT2, _ := New(root2, db)
	recoveredV2, _ := recoveredT2.TryGet(key)
	fmt.Printf("recovered [value 2], got [%v]\n", string(recoveredV2))

	// Updating t2 shall not effect t1
	fmt.Println("\nupdating trie 2")

	t2.TryUpdate(key, newValue2)
	newRoot2, _ := t2.Commit(nil)

	recoveredT1, _ = New(root1, db)
	recoveredV1, _ = recoveredT1.TryGet(key)
	fmt.Printf("recovered [value 1], got [%v]\n", string(recoveredV1))

	recoveredT2, _ = New(newRoot2, db)
	recoveredV2, _ = recoveredT2.TryGet(key)
	fmt.Printf("recovered [value 2], got [%v]\n", string(recoveredV2))
	// Output:
	// recovered [value 1], got [value 1]
	// recovered [value 2], got [value 2]
	//
	// updating trie 2
	// recovered [value 1], got [value 1]
	// recovered [value 2], got [value 3]
}
