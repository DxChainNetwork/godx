package merkle

import (
	"hash"

	"github.com/pkg/errors"
)

//CachedTree will store the leaf's data data instead of hash
type CachedTree struct {
	subTreeCachedHeight      uint64
	confirmStorageProofIndex uint64
	Tree
}

// NewCachedTree return a cachedTree
func NewCachedTree(h hash.Hash, height uint64) *CachedTree {
	return &CachedTree{
		subTreeCachedHeight: height,

		Tree: Tree{
			hash: h,

			usedAsCached: true,
		},
	}
}

// SetIndex must be called on an empty tree.
func (ct *CachedTree) SetIndex(i uint64) error {
	if ct.top != nil {
		return errors.New("must be called on an empty tree")
	}
	ct.confirmStorageProofIndex = i
	return ct.Tree.SetStorageProofIndex(i / (1 << ct.subTreeCachedHeight))
}

// Prove construct a storage proof result set for
// the cached tree that has established the storage proof index
func (ct *CachedTree) Prove(cachedTreeProofList [][]byte) (merkleRoot []byte, proofList [][]byte, storageProofIndex uint64, numLeaves uint64) {

	cachedSubtree := uint64(1) << ct.subTreeCachedHeight
	numLeaves = cachedSubtree * ct.leafIndex

	//get storage proof from merkle tree
	merkleRoot, merkleTreeProofList, _, _ := ct.Tree.ProofList()
	if len(merkleTreeProofList) < 1 {
		return merkleRoot, nil, ct.confirmStorageProofIndex, numLeaves
	}

	//we already have the data from storageProofIndex
	proofList = append(cachedTreeProofList, merkleTreeProofList[1:]...)
	return merkleRoot, proofList, ct.confirmStorageProofIndex, numLeaves
}
