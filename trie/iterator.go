package trie

import (
	"bytes"
	"container/heap"
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
)

type (
	// Iterator is a key-value trie iterator that traverses a Trie.
	Iterator struct {
		nodeIt NodeIterator

		Key   []byte // Current data key on which the iterator is positioned on
		Value []byte // Current data value on which the iterator is positioned on
		Err   error
	}

	// nodeIteratorState represents the iteration state at one particular node of the
	// trie, which can be resumed at a later invocation.
	nodeIteratorState struct {
		hash   common.Hash // Hash of the node being iterated (nil if not standalone)
		node   node        // Trie node being iterated
		parent common.Hash // Hash of the first full ancestor node (nil if current is the root)
		// Child to the processed next.
		// -1 	-- 	not visited
		// 0 	--  visited
		// 0-16	--	last visited index of full node
		index   int
		pathlen int // Length of the path to this node
	}

	// nodeIterator implements NodeIterator
	nodeIterator struct {
		trie  *Trie                // Trie being iterated
		stack []*nodeIteratorState // Hierarchy of trie nodes persisting the iteration state
		path  []byte               // Path to the current node
		err   error                // Failure set in caes of an internal error in the iterator
	}

	// seekError is stored in nodeIterator.err if the initial seek has failed.
	seekError struct {
		key []byte
		err error
	}

	// differenceIterator is a NodeIterator that iterates over elements in b that are not in a.
	differenceIterator struct {
		a, b  NodeIterator // Nodes returned are those in b - a.
		eof   bool         // Indicates a has run out of elements
		count int          // Number of nodes scanned on either trie
	}

	// nodeIteratorHeap is a heap of NodeIterators, which implements the heap structure
	nodeIteratorHeap []NodeIterator

	// unionIterator is a NodeIterator that iterates over elements in the union of the provided
	// NodeIterators.
	unionIterator struct {
		items *nodeIteratorHeap // Nodes returned are the union of the ones in these iterators
		count int               // Number of nodes scanned across all tries
	}
)

var (
	// errIteratorEnd is stored in nodeIterator.err when iteration is done.
	errIteratorEnd = errors.New("end of iteration")
)

// NodeIterator is an iterator to traverse the trie pre-order
type NodeIterator interface {
	// Next moves the iterator to the next node. If the parameter is false, any child
	// nodes will be skipped.
	Next(bool) bool

	// Error returns the error status of the iterator
	Error() error

	// Hash returns the hash of the current node
	Hash() common.Hash

	// Parent returns the hash of the parent of the current node. The hash may be the
	// one grandparent if the immediate parent is an internal node with no hash.
	Parent() common.Hash

	// Path returns the hex-encoded path to the current node.
	// Callers must not retain reference to the return value after Calling Next.
	// For leaf nodes, the last element of the path is the 'terminator symbol' 0x10.
	Path() []byte

	// Leaf returns true iff the current node is a leaf node.
	Leaf() bool

	// LeafKey returns the key of the leaf. The method panics if the iterator is not
	// positioned at a leaf. Callers must not retain references to the value after
	// calling Next.
	LeafKey() []byte

	// LeafBlob returns the content of the leaf. The method panics if the iterator is
	// not positioned at a leaf. Callers must not retain references to the value after
	// calling Next
	LeafBlob() []byte

	// LeafProof returns the Merkle proof of the leaf. The method panics if the
	// iterator is not positioned at a leaf. Callers must not retain references to
	// the value after calling Next.
	LeafProof() [][]byte
}

// NewIterator creates a new key-value iterator from a node iterator
func NewIterator(it NodeIterator) *Iterator {
	return &Iterator{
		nodeIt: it,
	}
}

// Next moves the iterator forward one key-value entry. If there is no more entries,
// return false, else return true. This is the function to be used directly by the
// upper level codes.
func (it *Iterator) Next() bool {
	for it.nodeIt.Next(true) {
		if it.nodeIt.Leaf() {
			it.Key = it.nodeIt.LeafKey()
			it.Value = it.nodeIt.LeafBlob()
			return true
		}
	}
	it.Key = nil
	it.Value = nil
	it.Err = it.nodeIt.Error()
	return false
}

// Prove generates the Merkle proof for the leaf node the iterator is currently
// positioned on.
func (it *Iterator) Prove() [][]byte {
	return it.nodeIt.LeafProof()
}

// Error is the function to print out the error message of seekError.
// Thus seekError implements error interface
func (e seekError) Error() string {
	return "seek error: " + e.err.Error()
}

// newNodeIterator create a new trie with start. Start is where the search begins
func newNodeIterator(trie *Trie, start []byte) NodeIterator {
	if trie.Hash() == emptyState {
		return new(nodeIterator)
	}
	it := &nodeIterator{trie: trie}
	it.err = it.seek(start)
	return it
}

// Hash returns the hash of the current node
func (it *nodeIterator) Hash() common.Hash {
	if len(it.stack) == 0 {
		return common.Hash{}
	}
	return it.stack[len(it.stack)-1].hash
}

// Parent returns the hash of the parent of the current node. The hash may be the
// one grandparent if the immediate parent is an internal node with no hash.
func (it *nodeIterator) Parent() common.Hash {
	if len(it.stack) == 0 {
		return common.Hash{}
	}
	return it.stack[len(it.stack)-1].parent
}

// The node in iterator is a leaf node only when it.path end with 0x10
func (it *nodeIterator) Leaf() bool {
	return hasTerm(it.path)
}

// LeafKey return the key of the current valueNode. Else panic.
func (it *nodeIterator) LeafKey() []byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return hexToKeybytes(it.path)
		}
	}
	panic("not at leaf")
}

// LeafBlob return the value of a valueNode
func (it *nodeIterator) LeafBlob() []byte {
	if len(it.stack) > 0 {
		if node, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return []byte(node)
		}
	}
	panic("not at leaf")
}

// LeafProof returns a merkle proof for the leaf.
// The proof is a list of rlp bytes of stack nodes after hashed.
func (it *nodeIterator) LeafProof() [][]byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			hasher := newHasher(0, 0, nil)
			defer returnHasherToPool(hasher)

			proofs := make([][]byte, 0, len(it.stack))

			for i, item := range it.stack[:len(it.stack)-1] {
				// Gather nodes that end up as hash nodes (or the root)
				node, _, _ := hasher.hashChildren(item.node, nil)
				hashed, _ := hasher.store(node, nil, false)
				if _, ok := hashed.(hashNode); ok || i == 0 {
					enc, _ := rlp.EncodeToBytes(node)
					proofs = append(proofs, enc)
				}
			}
			return proofs
		}
	}
	panic("not at leaf")
}

// Path return the path to the current node in []byte
func (it *nodeIterator) Path() []byte {
	return it.path
}

func (it *nodeIterator) Error() error {
	if it.err == errIteratorEnd {
		return nil
	}
	if seek, ok := it.err.(seekError); ok {
		return seek.err
	}
	return it.err
}

// Next moves the iterator to the next node, returning whether there are any
// further nodes. In case of an internal error this method returns false and
// sets the Error field to the encountered failure. If `descend` is false,
// skips iterating over any subnodes of the current node.
// Two functions is combined to achieve the goal of next: peek + push.
// Peek will find the next state but not add to iterator. After the result is
// confirmed, push will push the result of peek to iterator.
func (it *nodeIterator) Next(descend bool) bool {
	if it.err == errIteratorEnd {
		return false
	}
	if seek, ok := it.err.(seekError); ok {
		if it.err = it.seek(seek.key); it.err != nil {
			return false
		}
	}
	// Otherwise step forward with the iterator and report any errors.
	state, parentIndex, path, err := it.peek(descend)
	it.err = err
	if it.err != nil {
		return false
	}
	it.push(state, parentIndex, path)
	return true
}

// seek moves the iterator to the position of the prefix
func (it *nodeIterator) seek(prefix []byte) error {
	// The path we're looking for is the hex encoded key without terminator.
	key := keybytesToHex(prefix)
	key = key[:len(key)-1] // skip the terminator
	// Move forward until we are just before the closest match to key.
	for {
		state, parentIndex, path, err := it.peek(bytes.HasPrefix(key, it.path))
		if err == errIteratorEnd {
			return errIteratorEnd
		} else if err != nil {
			return seekError{prefix, err}
		} else if bytes.Compare(path, key) >= 0 {
			return nil
		}
		it.push(state, parentIndex, path)
	}
}

// peek creates the next state of the iterator. Combined with push, it will move the
// iterator one step to the next node (not the next leaf node). One step is to move the
// iterator goes up until the next branch or reach the end, and then move down one step.
func (it *nodeIterator) peek(descend bool) (*nodeIteratorState, *int, []byte, error) {
	if len(it.stack) == 0 {
		// Initialize the iterator if we've just started
		root := it.trie.Hash()
		state := &nodeIteratorState{node: it.trie.root, index: -1}
		if root != emptyRoot {
			state.hash = root
		}
		err := state.resolve(it.trie, nil)
		return state, nil, nil, err
	}
	if !descend {
		// If we are skipping children, pop the current node first
		it.pop()
	}

	// Continue iteration to the next child
	for len(it.stack) > 0 {
		parent := it.stack[len(it.stack)-1]
		ancestor := parent.hash
		if (ancestor == common.Hash{}) {
			ancestor = parent.parent
		}
		state, path, ok := it.nextChild(parent, ancestor)
		if ok {
			if err := state.resolve(it.trie, path); err != nil {
				return parent, &parent.index, path, err
			}
			return state, &parent.index, path, nil
		}
		// no more child nodes, move back up
		it.pop()
	}
	return nil, nil, nil, errIteratorEnd
}

func (it *nodeIterator) nextChild(parent *nodeIteratorState, ancestor common.Hash) (*nodeIteratorState, []byte, bool) {
	switch node := parent.node.(type) {
	case *fullNode:
		// Full node, move to the first non-nil child
		for i := parent.index + 1; i < len(node.Children); i++ {
			child := node.Children[i]
			if child != nil {
				hash, _ := child.cache()
				state := &nodeIteratorState{
					hash:    common.BytesToHash(hash),
					node:    child,
					parent:  ancestor,
					index:   -1,
					pathlen: len(it.path),
				}
				path := append(it.path, byte(i))
				parent.index = i - 1
				return state, path, true
			}
		}
	case *shortNode:
		// Short node, return the pointer singleton child.
		if parent.index < 0 {
			// If the node is not visited, the index is -1. Else always >= 0.
			hash, _ := node.Val.cache()
			state := &nodeIteratorState{
				hash:    common.BytesToHash(hash),
				node:    node.Val,
				parent:  ancestor,
				index:   -1,
				pathlen: len(it.path),
			}
			path := append(it.path, node.Key...)
			return state, path, true
		}
	}
	// return false, allow peek function to call pop to go up a node.
	return parent, it.path, false
}

// resolve will expand the current node if it's a hashNode.
func (st *nodeIteratorState) resolve(tr *Trie, path []byte) error {
	if hash, ok := st.node.(hashNode); ok {
		// Retrieve the expanded node from trie
		resolved, err := tr.resolveHash(hash, path)
		if err != nil {
			return err
		}
		st.node = resolved
		st.hash = common.BytesToHash(hash)
	}
	return nil
}

// push will push the state to it.stack, and increment the parentIndex if not nil.
func (it *nodeIterator) push(state *nodeIteratorState, parentIndex *int, path []byte) {
	it.path = path
	it.stack = append(it.stack, state)
	if parentIndex != nil {
		*parentIndex++
	}
}

// pop will pop the last element in it.stack.
func (it *nodeIterator) pop() {
	parent := it.stack[len(it.stack)-1]
	it.path = it.path[:parent.pathlen]
	it.stack = it.stack[:len(it.stack)-1]
}

// compareNodes will compare the two nodes.
// 1. Compare path
// 2. If one leaf, another not leaf, the leaf node is always smaller
// 3. compare two hashes
// 4. If both leaf nodes, compare values.
// 5. return 0.
func compareNodes(a, b NodeIterator) int {
	if cmp := bytes.Compare(a.Path(), b.Path()); cmp != 0 {
		// If path different, return the comparison result of the pathes
		return cmp
	}
	// Leaf node is always smaller
	if a.Leaf() && !b.Leaf() {
		return -1
	} else if b.Leaf() && !a.Leaf() {
		return 1
	}
	// If both is leaf or not leaf, return the comparison result of two hashes
	if cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && b.Leaf() {
		return bytes.Compare(a.LeafBlob(), b.LeafBlob())
	}
	return 0
}

// NewDifferenceIterator constructs a NodeIterator that iterates over elements in b that
// are not in a. Returns the iterator, and a pointer to an integer recording the number
// of nodes seen.
func NewDifferenceIterator(a, b NodeIterator) (NodeIterator, *int) {
	a.Next(true)
	it := &differenceIterator{
		a: a,
		b: b,
	}
	return it, &it.count
}

// Hash will return the Hash of iterator b
func (it *differenceIterator) Hash() common.Hash {
	return it.b.Hash()
}

// Parent will return the Parent of iterator b
func (it *differenceIterator) Parent() common.Hash {
	return it.b.Parent()
}

// Leaf will return whether the node in iterator b is leaf
func (it *differenceIterator) Leaf() bool {
	return it.b.Leaf()
}

// LeafKey will return the key of the node in iterator b. The function will panic if not
// leaf.
func (it *differenceIterator) LeafKey() []byte {
	return it.b.LeafKey()
}

// LeafBlob will return the value of the node in iterator b. The function will panic if
// not leaf.
func (it *differenceIterator) LeafBlob() []byte {
	return it.b.LeafBlob()
}

// LeafProof will return the proof of the node in iterator b.
func (it *differenceIterator) LeafProof() [][]byte {
	return it.b.LeafProof()
}

// Path will return the current path of node b
func (it *differenceIterator) Path() []byte {
	return it.b.Path()
}

// Next will move the difference iterator to the next node.
func (it *differenceIterator) Next(bool) bool {
	// Invariants:
	// - We always advance at least one element in b.
	// - At the start of this function, a's path is lexically greater than b's.
	if !it.b.Next(true) {
		return false
	}
	it.count++

	if it.eof {
		// a has reached eof, so we just return all elements from b
		return true
	}

	for {
		// Advance a until a surpasses b, or either a or b reached end.
		// Meanwhile, skip nodes that a and b are the same.
		switch compareNodes(it.a, it.b) {
		case -1:
			// b jumped past a; advance a
			if !it.a.Next(true) {
				// mark a has reached end
				it.eof = true
				return true
			}
			it.count++
		case 1:
			// b is before a
			return true
		case 0:
			// a and b are identical; skip this whole subtree if the nodes have hashes
			hasHash := it.a.Hash() == common.Hash{}
			if !it.b.Next(hasHash) {
				return false
			}
			it.count++
			if !it.a.Next(hasHash) {
				it.eof = true
				return true
			}
			it.count++
		}
	}
}

// Error will print out the error. Error in a will overwrite error in b.
func (it *differenceIterator) Error() error {
	if err := it.a.Error(); err != nil {
		return err
	}
	return it.b.Error()
}

// nodeIteratorHeap implements the heap interface.
func (h nodeIteratorHeap) Len() int            { return len(h) }
func (h nodeIteratorHeap) Less(i, j int) bool  { return compareNodes(h[i], h[j]) < 0 }
func (h nodeIteratorHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeIteratorHeap) Push(x interface{}) { *h = append(*h, x.(NodeIterator)) }
func (h *nodeIteratorHeap) Pop() interface{} {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[0 : n-1]
	return x
}

// NewUnionIterator contracts a NodeIterator that iterates over element in the union
// of the provided NodeIterators. Returns the iterator, and a pointer to an integer
// recording the number of nodes visited.
func NewUnionIterator(iters []NodeIterator) (NodeIterator, *int) {
	h := make(nodeIteratorHeap, len(iters))
	copy(h, iters)
	heap.Init(&h)

	ui := &unionIterator{items: &h}
	return ui, &ui.count
}

// Hash returns the hash of the current iterator.
func (it *unionIterator) Hash() common.Hash {
	return (*it.items)[0].Hash()
}

// Parent returns the parent of the current node.
func (it *unionIterator) Parent() common.Hash {
	return (*it.items)[0].Parent()
}

// Leaf returns the whether the current node is leaf or not.
func (it *unionIterator) Leaf() bool {
	return (*it.items)[0].Leaf()
}

// LeafKey returns the key of the current leaf node. Panic if the current node is not
// leaf.
func (it *unionIterator) LeafKey() []byte {
	return (*it.items)[0].LeafKey()
}

// LeafBlob returns the value of the current leaf node. Panic if the current node is
// not leaf.
func (it *unionIterator) LeafBlob() []byte {
	return (*it.items)[0].LeafBlob()
}

// LeafProof returns the proof of te current leaf. Panic if the current node is not
//// leaf.
func (it *unionIterator) LeafProof() [][]byte {
	return (*it.items)[0].LeafProof()
}

// Path returns the path of the current node.
func (it *unionIterator) Path() []byte {
	return (*it.items)[0].Path()
}

// Next returns the next node in the union of tries being iterated over.
//
// It does this by maintaining a heap of iterators, sorted by the iteration
// order of their next elements, with one entry for each source trie. Each
// time Next() is called, it takes the least element from the heap to return,
// advancing any other iterators that also point to that same element. These
// iterators are called with descend=false, since we know that any nodes under
// these nodes will also be duplicates, found in the currently selected iterator.
// Whenever an iterator is advanced, it is pushed back into the heap if it
// still has elements remaining.
//
// In the case that descend=false -eg. we're asked to ignore all subnodes of the
// current node - we also advance any iterators in the heap that have the current
// path as a prefix.
func (it *unionIterator) Next(descend bool) bool {
	if len(*it.items) == 0 {
		return false
	}

	// get the next key from the union
	least := heap.Pop(it.items).(NodeIterator)

	// Skip over other nodes as long as they're identical, or, if we're not descending,
	// as long as they have the same prefix as the current node.
	for len(*it.items) > 0 && ((!descend && bytes.HasPrefix((*it.items)[0].Path(), least.Path())) || compareNodes(least, (*it.items)[0]) == 0) {
		skipped := heap.Pop(it.items).(NodeIterator)
		// Skip the whole subtree if the node have hashes; otherwise just skip this node
		if skipped.Next(skipped.Hash() == common.Hash{}) {
			it.count++
			// If there are more elements, push the iterator back on the heap
			heap.Push(it.items, skipped)
		}
	}
	if least.Next(descend) {
		it.count++
		heap.Push(it.items, least)
	}
	return len(*it.items) > 0
}

// Error return the error message of the first error message in it.items.
func (it *unionIterator) Error() error {
	for i := 0; i < len(*it.items); i++ {
		if err := (*it.items)[i].Error(); err != nil {
			return err
		}
	}
	return nil
}
