package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/ethdb"
)

func ExampleProof() {
	trie := newTrieWithData(testData)
	hash, err := trie.Commit(nil)
	if err != nil {
		fmt.Println("trie commit returns an error:", err.Error())
		return
	}
	pd := ethdb.NewMemDatabase()

	// Put the proof for key "doge" into pd
	err = trie.Prove([]byte("doge"), 0, pd)
	if err != nil {
		fmt.Println("Prove returns an error:", err.Error())
		return
	}

	// Verify the doge proof
	value, nodes, err := VerifyProof(hash, []byte("doge"), pd)
	if err != nil {
		fmt.Println("VerifyProof returns an error:", err.Error())
		return
	}
	fmt.Printf("Traversed %d nodes, verified proof for key %s -> [%s]\n", nodes, "doge", string(value))
	// Output:
	// Traversed 4 nodes, verified proof for key doge -> [coin]
}

func ExampleMissProof() {
	trie := newTrieWithData(testData)
	hash, err := trie.Commit(nil)
	if err != nil {
		fmt.Println("trie commit returns an error:", err.Error())
		return
	}
	pd := ethdb.NewMemDatabase()

	// Put the proof for key "doge" into pd
	err = trie.Prove([]byte("doge"), 0, pd)
	if err != nil {
		fmt.Println("The Prove returns an error:", err.Error())
		return
	}
	// Verify a key not in db. Note if you find some other keys, the value of the key
	// still can be obtained, since the other key-value pair does not need to be hashed
	// and is already in pd.
	value, nodes, err := VerifyProof(hash, []byte("somethingveryoddindeedthis is"), pd)
	if err != nil {
		fmt.Println("VerifyProof returns an error:", err.Error())
		return
	}
	fmt.Printf("Traversed %d nodes, verified proof for key %s -> [%s]\n", nodes, "doge", string(value))
	// Output:
	// VerifyProof returns an error: proof node 1 (hash adbfedafc33dc930b9a9f0c6345320b4d57016638332d2fa08fbfbdd01449c45) missing
}

func ExampleProofLevel() {
	trie := newTrieWithData(testData)
	hash, err := trie.Commit(nil)
	if err != nil {
		fmt.Println("trie commit returns an error:", err.Error())
		return
	}
	pd := ethdb.NewMemDatabase()

	// Put the proof for key "doge" into pd. It will not put the last node into pd
	err = trie.Prove([]byte("doge"), 1, pd)
	if err != nil {
		fmt.Println("Prove returns an error:", err.Error())
		return
	}

	// Since not a complete set of proof nodes are put into pd, an error is expected.
	value, nodes, err := VerifyProof(hash, []byte("doge"), pd)
	if err != nil {
		fmt.Println("VerifyProof returns an error", err.Error())
		return
	}
	fmt.Printf("Traversed %d nodes, verified proof for key %s -> [%s]\n", nodes, "doge", string(value))
	// Output:
	// VerifyProof returns an error proof node 0 (hash 09c889feaafd53779755259beaa0ff41c32512c8cac45152af46fae7ebdef210) missing
}
