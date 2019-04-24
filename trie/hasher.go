package trie

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"golang.org/x/crypto/sha3"
	"hash"
	"sync"
)

// hasher is the structure used to hash the node.
type hasher struct {
	tmp        sliceBuffer
	sha        keccakState
	cachegen   uint16
	cachelimit uint16
	onleaf     LeafCallback
}

// kecaakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type keccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

type sliceBuffer []byte

// Write function for sliceBuffer write the data to its data structure
func (b *sliceBuffer) Write(data []byte) (n int, err error) {
	*b = append(*b, data...)
	return len(data), nil
}

// Reset function reset the sliceBuffer
func (b *sliceBuffer) Reset() {
	*b = (*b)[:0]
}

// hashers live in a global db
var hasherPool = sync.Pool{
	New: func() interface{} {
		return &hasher{
			tmp: make(sliceBuffer, 0, 550), // cap is as large as a full fullNode
			sha: sha3.NewLegacyKeccak256().(keccakState),
		}
	},
}

// newHasher grab an arbitrary Hasher from hasherPool.
func newHasher(cachegen, cachelimit uint16, onleaf LeafCallback) *hasher {
	h := hasherPool.Get().(*hasher)
	h.cachegen, h.cachelimit, h.onleaf = cachegen, cachelimit, onleaf
	return h
}

// returnHasherToPool returns a hasher to hasherPool
func returnHasherToPool(h *hasher) {
	hasherPool.Put(h)
}

// hash collapses a node down into a hash node, also returning a copy of the
// original node initialized with the computed hash to replace the original one.
// hash function do the following:
// 1. For node that is no need to unload (clean and previously hashed), do nothing
// 		and return.
// 2. For node needed to be cached, hash all its children with recursive call to hash.
// 		The call returns collapsed node and cached node.
// 3. Call store function to insert the hashed node to db, and use h.onleaf function to
//		n.Val or n.Children.
// 4. Update flag (hash and dirty) of cached node.
// 5. Return hashed node, cached node, nil.
//
// Another perspective: Divide the function to two threads:
// 	one deal with collapsed and cached nodes,
// 	one deal with cached nodes.

// For the one dealing with collapsed nodes:
//		1. Judge whether the original node could be unloaded directly.
// 		2. Make a copy of the original nodes, if short node, encode the key to compact for the new node.
// 		3. Link collapsed descendents from recursive call to hash function to the new node.
// 		4. Call store function to store the collapsed node to database, returning a hashNode
//			(or itself if rlp length < 32)
// 		5. Return the hashNode.

// For the one dealing with cached nodes:
// 		1. Judge whether the original node could be unloaded directly.
//		2. Make a copy of the original node.
//		3. Link cached descendents from recursive call to hash function to the new node.
//		4. Mark the hash from hashedNode as cached node's flag
//		5. Return the cached node.
//
// Thus, to conclude, the hash function will return two nodes:
//		1. hashed node: could be
//			a. hashNode: 	The node has already been inserted into database
//			b. shortNode: 	RLP length must be less than 32. Key encoded to compact. Descendents
//								are also returned hashed node from hash function.
//			c. fullNode: 	RLP length must be less than 32. Descendents are also returned hashed
//								node from hash function.
// 			d. valueNode:	RLP length must be less than 32. Has been already inserted to database
//
//		2. cachedNode:	Fully expanded node. The difference from the original node is that
//							its and its descendents' hash are not nil.
func (h *hasher) hash(n node, db *Database, force bool) (node, node, error) {
	// If we're not storing the node, just hashing, use available cached data
	if hash, dirty := n.cache(); hash != nil {
		if db == nil {
			return hash, n, nil
		}
		if n.canUnload(h.cachegen, h.cachelimit) {
			// Unload the node from cache. All of its subnodes will have a lower or equal
			// cache generation number.
			cacheUnloadCounter.Inc(1)
			return hash, hash, nil
		}
		if !dirty {
			return hash, n, nil
		}
	}
	// Trie not processed yet or needs storage, walk the children
	collapsed, cached, err := h.hashChildren(n, db)
	if err != nil {
		return hashNode{}, n, err
	}
	hashed, err := h.store(collapsed, db, force)
	if err != nil {
		return hashNode{}, n, err
	}
	// Cache the hash of the node for later reuse and remove
	// the dirty flag in commit mode. It's fine to assign these values directly
	// without copying the node first because hashChildren copies it.
	cachedHash, _ := hashed.(hashNode)
	switch cn := cached.(type) {
	case *shortNode:
		cn.flags.hash = cachedHash
		if db != nil {
			cn.flags.dirty = false
		}
	case *fullNode:
		cn.flags.hash = cachedHash
		if db != nil {
			cn.flags.dirty = false
		}
	}
	return hashed, cached, nil
}

// hashChildren replaces the children of a node with their hashes if the encoded
// size of the child is larger than a hash, returning the collapsed node as well
// as a replacement for the original node with the child hashes cached in.
// This function is only called by h.hash and do some operation on the input node.
// It does the following:
// 1. Based on node type, make two copies of the original node. One for collapsed,
// 		one for cached. The collapsed has compact key and cached has hex key.
// 2. Hash the children or value of the node, returning two node: collapsed and
// 		cached. Link the collapsed one to the collapsed (might be hashNode), link
// 		the cached one to the cached (not hashed).
func (h *hasher) hashChildren(original node, db *Database) (node, node, error) {
	var err error

	switch n := original.(type) {
	case *shortNode:
		// Hash the short node's child, caching the newly hashed subtree
		collapsed, cached := n.copy(), n.copy()
		collapsed.Key = hexToCompact(n.Key)
		cached.Key = common.CopyBytes(n.Key)

		if _, ok := n.Val.(valueNode); !ok {
			collapsed.Val, cached.Val, err = h.hash(n.Val, db, false)
			if err != nil {
				return original, original, err
			}
		}
		return collapsed, cached, nil

	case *fullNode:
		// Hash the full node's children, caching the newly hashed subtrees
		collapsed, cached := n.copy(), n.copy()

		for i := 0; i < 16; i++ {
			if n.Children[i] != nil {
				collapsed.Children[i], cached.Children[i], err = h.hash(n.Children[i], db, false)
				if err != nil {
					return original, original, err
				}
			}
		}
		cached.Children[16] = n.Children[16]
		return collapsed, cached, nil

	default:
		// Value and hash nodes don't have children so they're left as were
		return n, original, nil
	}
}

// store hashes the node n and if we have a storage layer specified, it writes
// the key/value pair to it and tracks any node->child references as well as any
// node->external trie references. Returned value must be hashNode except for
// node's length is smaller than 32.
// The function does the following:
// 	1. Encode rlp of the node.
//	2. If rlp result is less than 32 and not force flag, return node itself.
// 	3. Insert the collapsed node to database.
// 	4. Call h.onleaf to n.Val or n.Children
func (h *hasher) store(n node, db *Database, force bool) (node, error) {
	// Don't store hashes or empty nodes.
	if _, isHash := n.(hashNode); n == nil || isHash {
		return n, nil
	}
	// Generate the RLP encoding of the node
	h.tmp.Reset()
	if err := rlp.Encode(&h.tmp, n); err != nil {
		panic("encode error: " + err.Error())
	}
	if len(h.tmp) < 32 && !force {
		return n, nil // Nodes smaller than 32 bytes are stored inside their parent
	}
	// Larger nodes are replaced by their hash and stored in the database.
	hash, _ := n.cache()
	if hash == nil {
		hash = h.makeHashNode(h.tmp)
	}

	if db != nil {
		// We are pooling the trie nodes into an intermediate memory cache
		hash := common.BytesToHash(hash)

		db.lock.Lock()
		db.insert(hash, h.tmp, n)
		db.lock.Unlock()

		// Track external references from account->storage trie
		if h.onleaf != nil {
			switch n := n.(type) {
			case *shortNode:
				if child, ok := n.Val.(valueNode); ok {
					h.onleaf(child, hash)
				}
			case *fullNode:
				for i := 0; i < 16; i++ {
					if child, ok := n.Children[i].(valueNode); ok {
						h.onleaf(child, hash)
					}
				}
			}
		}
	}
	return hash, nil
}

// makeHashNode make use of h.sha function to hash the data bytes
func (h *hasher) makeHashNode(data []byte) hashNode {
	n := make(hashNode, h.sha.Size())
	h.sha.Reset()
	h.sha.Write(data)
	h.sha.Read(n)
	return n
}
