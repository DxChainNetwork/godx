package merkle

import "hash"

type subtree struct {
	next   *subtree
	height int
	sum    []byte
}

type Tree struct {
	top               *subtree
	hash              hash.Hash
	nodeIndex         uint64
	storageProofIndex uint64
	storageProofList  [][]byte
	usedAsProof       bool
	usedAsCached      bool
}

func New(h hash.Hash) *Tree {
	return &Tree{
		hash: h,
	}
}
