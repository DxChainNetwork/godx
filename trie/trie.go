package trie

import (
	"bytes"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/metrics"
)

// struct:
// 		Trie    Merkle Patricia Trie sits on top of a Database
//

type (
	// Trie is a Merkle Patricia Trie.
	// The zero value is an empty trie with no database.
	// Use New to create a trie that sits on top of a database
	//
	// Trie is not safe for concurrent use.
	Trie struct {
		db   *Database
		root node

		// Cache generation values.
		// cachegen increases by one each commit operation
		// new nodes are tagged with the current generation and unloaded
		// when their generation is older than cachegen-cachelimit
		cachegen, cachelimit uint16
	}

	// LeafCallback is a callback type invoked when a trie operation reaches a leaf
	// node. It's used by state sync and commit to allow handling external references
	// between account and storage tries.
	LeafCallback func(leaf []byte, parent common.Hash) error
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// Register the counter for cache miss and cache unload
	cacheMissCounter   = metrics.NewRegisteredCounter("trie/cachemiss", nil)
	cacheUnloadCounter = metrics.NewRegisteredCounter("trie/cacheunload", nil)
)

// SetCacheLimit sets the number of 'cache generations' to keep.
// A cache generation is created by a call to Commit
func (t *Trie) SetCacheLimit(l uint16) {
	t.cachelimit = l
}

// newFalg returns the cache flag value for a newly created node.
func (t *Trie) newFlag() nodeFlag {
	return nodeFlag{dirty: true, gen: t.cachegen}
}

// New creates a trie with an existing root node from db.
//
// If root is the zero hash or the sha3 hash of an empty string, the
// trie is initially empty and does not require a database. Otherwise,
// New will panic if db is nil and returns a MissingNodeError if root does
// not exist in the database. Accessing the trie loads nodes from db on demand.
func New(root common.Hash, db *Database) (*Trie, error) {
	if db == nil {
		panic("Trie.New called without a database")
	}
	trie := &Trie{
		db: db,
	}
	if root != (common.Hash{}) && root != emptyRoot {
		rootnode, err := trie.resolveHash(root[:], nil)
		if err != nil {
			return nil, err
		}
		trie.root = rootnode
	}
	return trie, nil
}

// NodeIterator returns an iterator that returns nodes of the trie. Iteration
// starts at the key after the given start key
func (t *Trie) NodeIterator(start []byte) NodeIterator {
	return newNodeIterator(t, start)
}

// Get returns the value for key stored in the trie
// The value bytes must not be modified by the caller
func (t *Trie) Get(key []byte) []byte {
	res, err := t.TryGet(key)
	if err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
	return res
}

// TryGet returns the value for key stored in the trie.
// The value bytes must not be modified by the caller.
// If a node was not found in the database, a MissingNodeError is returned.
func (t *Trie) TryGet(key []byte) ([]byte, error) {
	key = keybytesToHex(key)
	value, newroot, didResolve, err := t.tryGet(t.root, key, 0)
	if err == nil && didResolve {
		t.root = newroot
	}
	return value, err
}

// tryGet is the private method used to get the value for the key recursively.
// The method will traverse the trie (including hashNode) and return the value and new root node.
// The path from new root node to leafnode has no hashNode, so that it could be traversed directly
// top to bottom. Return value of didResolve is the bool whether the node is a hashNode and
// the hashed node has to be dealt with.
func (t *Trie) tryGet(origNode node, key []byte, pos int) (value []byte, newnode node, didResolve bool, err error) {
	switch n := (origNode).(type) {
	case nil:
		// node not found
		return nil, nil, false, nil
	case valueNode:
		return n, n, false, nil
	case *shortNode:
		if len(key)-pos < len(n.Key) || !bytes.Equal(n.Key, key[pos:pos+len(n.Key)]) {
			// key not found in trie
			return nil, n, false, nil
		}
		value, newnode, didResolve, err := t.tryGet(n.Val, key, pos+len(n.Key))
		if err == nil && didResolve {
			n = n.copy()
			n.Val = newnode
			n.flags.gen = t.cachegen
		}
		return value, n, didResolve, err
	case *fullNode:
		// The key always have terminator 16. key[pos] could be 16 if key ends
		value, newnode, didResolve, err = t.tryGet(n.Children[key[pos]], key, pos+1)
		if err == nil && didResolve {
			n = n.copy()
			n.flags.gen = t.cachegen
			n.Children[key[pos]] = newnode
		}
		return value, n, didResolve, err
	case hashNode:
		child, err := t.resolveHash(n, key[:pos])
		if err != nil {
			return nil, n, true, err
		}
		value, newnode, _, err := t.tryGet(child, key, pos)
		return value, newnode, true, err
	default:
		panic(fmt.Sprintf("%T: invalid node: %v", origNode, origNode))
	}
}

// Update associates key with value in the trie. Subsequent calls to
// Get will return value. If value has length zero, any existing value
// is deleted from the trie and calls to Get will return nil
//
// The value bytes must not be modified by the caller while they are
// stored in the trie.
func (t *Trie) Update(key, value []byte) {
	if err := t.TryUpdate(key, value); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

// TryUpdate associates key with value in the trie. Subsequent calls to
// Get will return value. If value has length zero, any existing value
// is deleted from the trie and calls to Get will return nil.
//
// The value bytes must not be modified by the caller while they are
// stored in the trie.
//
// If a node was not found in the database, a MissingNodeError is returned.
func (t *Trie) TryUpdate(key, value []byte) error {
	k := keybytesToHex(key)
	if len(value) != 0 {
		_, n, err := t.insert(t.root, nil, k, valueNode(value))
		if err != nil {
			return err
		}
		t.root = n
	} else {
		_, n, err := t.delete(t.root, nil, k)
		if err != nil {
			return err
		}
		t.root = n
	}
	return nil
}

// insert is a private method for inserting a key-value pair into the trie.
// params:
// 		n: 			Current node. The new key value pair is to be inserted to n's children
// 		prefix:  	Already applied prefix (path) of the key
// 		key: 		What's left in the key.
// 		value: 		what to be inserted after the key. Could be all expanded node type.
//
// return:
//  	dirty bool:	Whether the node shall be replaced with second argument node.
//		node node:
func (t *Trie) insert(n node, prefix, key []byte, value node) (bool, node, error) {
	if len(key) == 0 {
		// If the left key has length 0, meaning all the key has been consumed,
		// input node n could be valueNode, thus toUpdate the value of an existing
		// key-value pair.
		// This "if" has to be executed before switch type of n.
		if v, ok := n.(valueNode); ok {
			// dirty set to true if the value has to be updated
			return !bytes.Equal(v, value.(valueNode)), value, nil
		}
		return true, value, nil
	}
	switch n := n.(type) {
	case *shortNode:
		matchlen := prefixLen(key, n.Key)
		if matchlen == len(n.Key) {
			// If the whole key matches, keep this short node as is
			// and only update the value from n.Val
			// n.val could be valueNode, or branchNode.
			dirty, nn, err := t.insert(n.Val, append(prefix, key[:matchlen]...), key[matchlen:], value)
			if !dirty || err != nil {
				return false, n, err
			}
			// If the insert result is dirty, need create a new short node pointing to
			// the new node.
			return true, &shortNode{n.Key, nn, t.newFlag()}, nil
		}
		// Otherwise branch out at the index where they differ.
		branch := &fullNode{flags: t.newFlag()}
		var err error
		// Create the branch for the original n.Val
		// n.Key[matchlen+1] have at least length 1, so that the next recursion must go
		// to case shortNode
		_, branch.Children[n.Key[matchlen]], err = t.insert(nil, append(prefix, n.Key[:matchlen+1]...), n.Key[matchlen+1:], n.Val)
		if err != nil {
			return false, nil, err
		}
		// Create the branch for the new node.
		// After modification, branch.Children[key[matchlen]] could be valueNode
		// (if len(key[matchlen+1:]) == 0) or shortNode (else).
		_, branch.Children[key[matchlen]], err = t.insert(nil, append(prefix, key[:matchlen+1]...), key[matchlen+1:], value)
		if err != nil {
			return false, nil, err
		}
		// Replace this shortNode with the branch if it occurs at index 0
		if matchlen == 0 {
			return true, branch, nil
		}
		// Otherwise, replace it with a short node leading up to the branch
		return true, &shortNode{key[:matchlen], branch, t.newFlag()}, nil

	case *fullNode:
		// Insert the key-value pair to n.children[key[0]].
		// n.children[key[0]] could be nil or not.
		// input n has to be updated if insert result is dirty.
		dirty, nn, err := t.insert(n.Children[key[0]], append(prefix, key[0]), key[1:], value)
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = t.newFlag()
		n.Children[key[0]] = nn
		return true, n, nil

	case nil:
		// If the input node is nil, and what key left is not of length 0,
		// need to create a new shortNode linking to the value.
		return true, &shortNode{key, value, t.newFlag()}, nil

	case hashNode:
		// We've hit a part of the trie that isn't loaded yet. Load
		// the node and insert into it. This leaves all child nodes on
		// the path to the value in the trie.
		rn, err := t.resolveHash(n, prefix)
		if err != nil {
			return false, nil, err
		}
		dirty, nn, err := t.insert(rn, prefix, key, value)
		if !dirty || err != nil {
			return false, rn, err
		}
		return true, nn, nil

	default:
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

// Delete removes any existing value for key from the trie
func (t *Trie) Delete(key []byte) {
	if err := t.TryDelete(key); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

// TryDelete removes any existing value for key from the trie
// If a node was not found in the database, a MissingNodeError is returned.
func (t *Trie) TryDelete(key []byte) error {
	k := keybytesToHex(key)
	_, n, err := t.delete(t.root, nil, k)
	if err != nil {
		return err
	}
	t.root = n
	return nil
}

// delete returns the new root of the trie with key deleted.
// It reduces the trie to minimal form by simplifying
// nodes on the way up after deleting recursively.
func (t *Trie) delete(n node, prefix, key []byte) (bool, node, error) {
	switch n := n.(type) {
	case *shortNode:
		matchlen := prefixLen(key, n.Key)
		if matchlen < len(n.Key) {
			return false, n, nil // no match
		}
		if matchlen == len(key) {
			return true, nil, nil // whole match. remove n
		}
		// The key is longer than n.Key. Remove the remaining suffix
		// from the subtrie. Child can never be nil here since the
		// subtrie must contain at least two other values with keys
		// longer than n.Key
		dirty, child, err := t.delete(n.Val, append(prefix, key[:len(n.Key)]...), key[len(n.Key):])
		if !dirty || err != nil {
			return false, n, err
		}
		// After deletion, the child type might be changed. If it is changed from
		// fullNode to shortNode, concatenate two shortNodes
		switch child := child.(type) {
		case *shortNode:
			// Concatenate the two shortNodes
			return true, &shortNode{concat(n.Key, child.Key...), child.Val, t.newFlag()}, nil
		default:
			return true, &shortNode{n.Key, child, t.newFlag()}, nil
		}

	case *fullNode:
		dirty, nn, err := t.delete(n.Children[key[0]], append(prefix, key[0]), key[1:])
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = t.newFlag()
		n.Children[key[0]] = nn
		// Check how many non-nil entries are left after deleting and reduce the full
		// node to a short node if only one entry is left.
		pos := -1
		for i, cld := range &n.Children {
			if cld != nil {
				if pos == -1 {
					pos = i
				} else {
					pos = -2
					break
				}
			}
		}
		if pos >= 0 {
			if pos != 16 {
				// actually a short node.
				// Expand the children[pos].
				cnode, err := t.resolve(n.Children[pos], prefix)
				if err != nil {
					return false, nil, err
				}
				if cnode, ok := cnode.(*shortNode); ok {
					k := append([]byte{byte(pos)}, cnode.Key...)
					return true, &shortNode{k, cnode.Val, t.newFlag()}, nil
				}
			}
			// Otherwise, n is replaced by a one-nibble short node containing
			// the child.
			return true, &shortNode{[]byte{byte(pos)}, n.Children[pos], t.newFlag()}, nil
		}
		// n still contains at least two values and cannot be reduced.
		return true, n, nil

	case valueNode:
		// If it's a value node, direcly delete the node.
		// This will only happen when delete fullNode.Children[16]
		return true, nil, nil

	case nil:
		// Delete a key does not exist
		return false, nil, nil

	case hashNode:
		// Fetch the actual node from db
		rn, err := t.resolveHash(n, prefix)
		if err != nil {
			return false, nil, err
		}
		dirty, nn, err := t.delete(rn, prefix, key)
		if !dirty || err != nil {
			return false, rn, err
		}
		return true, nn, nil

	default:
		panic(fmt.Sprintf("%T: invalid node: %v (%v)", n, n, key))
	}
}

// resolve return a expanded node if input is a hashNode. else return the node itself
func (t *Trie) resolve(n node, prefix []byte) (node, error) {
	if n, ok := n.(hashNode); ok {
		return t.resolveHash(n, prefix)
	}
	return n, nil
}

// resolveHash retrieve a node from database.
// If the node exist in db, return an expandedNode with nil error.
// Else return the MissingNodeError.
func (t *Trie) resolveHash(n hashNode, prefix []byte) (node, error) {
	cacheMissCounter.Inc(1)

	hash := common.BytesToHash(n)
	if node := t.db.node(hash, t.cachegen); node != nil {
		return node, nil
	}
	return nil, &MissingNodeError{NodeHash: hash, Path: prefix}
}

// Root returns the root hash of the trie
// Deprecated: use Hash instead
func (t *Trie) Root() []byte { return t.Hash().Bytes() }

// Hash returns the root hash of the trie. It does not write to the database
// and can be used even if the trie does not have one.
func (t *Trie) Hash() common.Hash {
	hash, cached, _ := t.hashRoot(nil, nil)
	t.root = cached
	return common.BytesToHash(hash.(hashNode))
}

// Commit writes all nodes to the trie's memory database, tracking the internal
// and external (for account tries) references.
func (t *Trie) Commit(onleaf LeafCallback) (root common.Hash, err error) {
	if t.db == nil {
		panic("commit called on trie with nil database")
	}
	hash, cached, err := t.hashRoot(t.db, onleaf)
	if err != nil {
		return common.Hash{}, err
	}
	t.root = cached
	t.cachegen++
	return common.BytesToHash(hash.(hashNode)), nil
}

// hashRoot return the hashed root for the trie
func (t *Trie) hashRoot(db *Database, onleaf LeafCallback) (node, node, error) {
	if t.root == nil {
		return hashNode(emptyRoot.Bytes()), nil, nil
	}
	h := newHasher(t.cachegen, t.cachelimit, onleaf)
	defer returnHasherToPool(h)
	return h.hash(t.root, db, true)
}

// concat concatenate two []byte to one []byte
func concat(s1 []byte, s2 ...byte) []byte {
	r := make([]byte, len(s1)+len(s2))
	copy(r, s1)
	copy(r[len(s1):], s2)
	return r
}

// CacheMisses retrieves a global counter measuring the number of cache misses
// the trie had since process startup. This isn't useful for anything apart from
// trie debugging purpose
func CacheMisses() int64 {
	return cacheMissCounter.Count()
}

// CacheUnloads retrieves a global counter measuring the number of cache uploads
// the trie did since process startup. This isn't useful for anything apart from
// trie debugging purpose
func CacheUnloads() int64 {
	return cacheUnloadCounter.Count()
}
