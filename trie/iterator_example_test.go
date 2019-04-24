package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
)

func ExampleNodeIterator() {
	trie := newTrieWithData(testData)
	trie.Commit(nil)

	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		fmt.Printf("Got key value pair: %s -> %s\n", it.Key, it.Value)
	}
	// Output:
	// Got key value pair: doge -> coin
	// Got key value pair: dog -> puppy
	// Got key value pair: do -> verb
	// Got key value pair: ether -> wookiedoo
	// Got key value pair: horse -> stallion
	// Got key value pair: shaman -> horse
	// Got key value pair: somethingveryoddindeedthis is -> myothernodedata
}

func ExampleDifferenceIterator() {
	// Create two iterators it1 and it2. it2 is the updated version of it1.
	trie1 := newTrieWithData(testData)
	hash, _ := trie1.Commit(nil)
	it1 := trie1.NodeIterator([]byte{})
	trie2, _ := New(hash, trie1.db)
	trie2.Update([]byte("doom"), []byte("catastrophe"))
	it2 := trie2.NodeIterator([]byte{})

	// Create the difference iterator based on it1 and it2.
	it, cnt := NewDifferenceIterator(it1, it2)
	dit := NewIterator(it)
	for dit.Next() {
		fmt.Printf("Retrieved record: %s -> %s\n", string(dit.Key), string(dit.Value))
	}
	fmt.Printf("In total %d nodes from trie1 and trie2.\n", *cnt)
	// Output:
	// Retrieved record: doom -> catastrophe
	// In total 31 nodes from trie1 and trie2.
}

func ExampleUnionIterator() {
	// Create two iterators it1 and it2. it2 is the updated version of it1.
	trie1 := newTrieWithData(testData)
	hash, _ := trie1.Commit(nil)
	it1 := trie1.NodeIterator([]byte{})
	trie2, _ := New(hash, trie1.db)
	trie2.Update([]byte("doom"), []byte("catastrophe"))
	it2 := trie2.NodeIterator([]byte{})

	// Create the union iterator based on it1 and it2.
	it, cnt := NewUnionIterator([]NodeIterator{it1, it2})
	uit := NewIterator(it)
	for uit.Next() {
		fmt.Printf("Retrieved record: %s -> %s\n", string(uit.Key), string(uit.Value))
	}
	fmt.Printf("In total %d nodes from trie1 and trie2.\n", *cnt)
	// Output:
	// Retrieved record: doge -> coin
	// Retrieved record: dog -> puppy
	// Retrieved record: doom -> catastrophe
	// Retrieved record: do -> verb
	// Retrieved record: ether -> wookiedoo
	// Retrieved record: horse -> stallion
	// Retrieved record: shaman -> horse
	// Retrieved record: somethingveryoddindeedthis is -> myothernodedata
	// In total 37 nodes from trie1 and trie2.
}

func ExampleIteratorProof() {
	trie := newTrieWithData(testData)
	hash, err := trie.Commit(nil)
	pd := ethdb.NewMemDatabase()
	// Create the proofs with iterator
	it := NewIterator(trie.NodeIterator(nil))
	if !it.Next() {
		fmt.Println("iterator Next return error:", it.Err)
	}
	key := it.Key
	proofs := it.Prove()
	for _, proof := range proofs {
		err := pd.Put(crypto.Keccak256(proof), proof)
		if err != nil {
			fmt.Println("ethdb Put error:", err.Error())
		}
	}
	val, _, err := VerifyProof(hash, []byte(key), pd)
	if err != nil {
		fmt.Println("VerifyProof return error:", err.Error())
	}
	fmt.Printf("Proof for key [%s] finished, return value [%s]\n", key, val)
	// Output:
	// Proof for key [doge] finished, return value [coin]
}
