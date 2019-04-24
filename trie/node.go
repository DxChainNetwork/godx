package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
)

// node.go defines the node interface and four types of nodes. The node types defined here
// are fully expanded.

// Interface:
// 	node { fstring, cache, canUnload }

// Structs:
// 	fullNode:  branch node. [17]node with nodeFlag
// 	shortNode: leaf node or extension node. Key-Value pair with nodeFlag
// 	hashNode:  used for computing merkle root
//  valueNode: value of a leaf node

// Methods:
//  mustDecodeNode: Decode the input buffer to shortNode/fullNode. The method will decode the
//  				buffer recursively.

// TODO: Add testcases for decode functions

type (
	// nodeFlag contains caching-related metadata about a node.
	nodeFlag struct {
		hash  hashNode // cached hash of the node (may be nil)
		gen   uint16   // cache generation counter
		dirty bool     // whether the node has changed that must be written to the database
	}

	// fullNode is the branch node in the yellow paper, which fork the search tree into multiple branches
	fullNode struct {
		Children [17]node // all 17 available slots could be allocated, aka. indices
		flags    nodeFlag // flags indicating status of the node, including cached hash, cachegen,
		// and whether node is dirty.
	}

	// shortNode is the leaf node and extension node in the yellow paper.
	// leaf node is the leaf, meaning a key value pair.
	// extension node serves as a "bridge" between branch nodes and leaf nodes.
	// The actual type could be distinguished by checking shortNode.Val type.
	shortNode struct {
		Key   []byte   // Key is the string key contained in this node.
		Val   node     // Val is the node this node is pointing to.
		flags nodeFlag // flags indicating status of the node, including cached hash, cachegen,
		// and whether node is dirty.
	}

	// The rlp string is too long so that the node is hashed.
	hashNode []byte

	// valueNode is the value of the leafNode, which should be able to be decoded into string
	valueNode []byte
)

// indices is all available slot for the branch node
var (
	indices = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "[17]"}

	nilValueNode = valueNode(nil)
)

// hashLen is the length of hash from common module, which is 32
const hashLen = len(common.Hash{})

// node interface defines the basic interface of a node
type node interface {
	fstring(string) string
	// node.cache return the cached hash of the node and dirty flag if exist
	cache() (hashNode, bool)
	// whether the node can be unloaded
	canUnload(cachegen uint16, cachelimit uint16) bool
}

// EncodeRLP encodes a full node into the consensus RLP format.
func (n *fullNode) EncodeRLP(w io.Writer) error {
	var nodes [17]node

	for i, child := range &n.Children {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	return rlp.Encode(w, nodes)
}

// copy function deep copy the fullNode or shortNode
func (n *fullNode) copy() *fullNode   { copy := *n; return &copy }
func (n *shortNode) copy() *shortNode { copy := *n; return &copy }

// canUnload return whether the nodeFlag indicates unload message.
// The node can be unload only if the node is not dirty and generation has not changed exceeding chachelimit
func (n *nodeFlag) canUnload(cachegen, cachelimit uint16) bool {
	return !n.dirty && cachegen-n.gen >= cachelimit
}

// whether full node and short node could be unloaded based on n.flags info
func (n *fullNode) canUnload(gen, limit uint16) bool  { return n.flags.canUnload(gen, limit) }
func (n *shortNode) canUnload(gen, limit uint16) bool { return n.flags.canUnload(gen, limit) }

// hashNode and valueNode does not have field flags thus always return false
func (n hashNode) canUnload(uint16, uint16) bool  { return false }
func (n valueNode) canUnload(uint16, uint16) bool { return false }

func (n *fullNode) cache() (hashNode, bool)  { return n.flags.hash, n.flags.dirty }
func (n *shortNode) cache() (hashNode, bool) { return n.flags.hash, n.flags.dirty }
func (n hashNode) cache() (hashNode, bool)   { return nil, true }
func (n valueNode) cache() (hashNode, bool)  { return nil, true }

// Pretty printing
func (n *fullNode) String() string  { return n.fstring("") }
func (n *shortNode) String() string { return n.fstring("") }
func (n hashNode) String() string   { return n.fstring("") }
func (n valueNode) String() string  { return n.fstring("") }

// fullNode.fstring return the formated string for print
func (n *fullNode) fstring(ind string) string {
	resp := fmt.Sprintf("[\n%s  ", ind)
	// loop over all children for the node
	for i, node := range &n.Children {
		if node == nil {
			resp += fmt.Sprintf("%s: <nil> ", indices[i])
		} else {
			resp += fmt.Sprintf("%s: %v", indices[i], node.fstring(ind+"  "))
		}
	}
	return resp + fmt.Sprintf("\n%s] ", ind)
}

// shortnode.fstring return the formatted string for print
func (n *shortNode) fstring(ind string) string {
	return fmt.Sprintf("{%x: %v} ", n.Key, n.Val.fstring(ind+"  "))
}

// hashNode.fstring return the formatted string for print
func (n hashNode) fstring(ind string) string {
	return fmt.Sprintf("<%x> ", []byte(n))
}

// valueNode.fstring return the formatted string for print
func (n valueNode) fstring(ind string) string {
	return fmt.Sprintf("%x ", []byte(n))
}

// mustDecodeNode called the function decodeNode. decode the buf to nodes
// If an error is returned, panic. Else, return the decoded node
func mustDecodeNode(hash []byte, buf []byte, cachegen uint16) node {
	n, err := decodeNode(hash, buf, cachegen)
	if err != nil {
		panic(fmt.Sprintf("node %x: %v", hash, err))
	}
	return n
}

// decodeNode use RLP decoding to determine the type of the encoded node type,
// and pass the parameters to decodeShort or decodeFull.
func decodeNode(hash []byte, buf []byte, cachegen uint16) (node, error) {
	if len(buf) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	// Use rlp to decode the first list element from buf, and ignore the tails
	elems, _, err := rlp.SplitList(buf)
	if err != nil {
		return nil, fmt.Errorf("decode error: %v", err)
	}
	switch c, _ := rlp.CountValues(elems); c {
	case 2:
		// If the first list element of encoded buf has two elements, means the node is a short node
		// Why is 2? Key: Value
		n, err := decodeShort(hash, elems, cachegen)
		return n, wrapError(err, "short")
	case 17:
		// If the first list element of encoded buf has 17 elements, the node must be a full node
		n, err := decodeFull(hash, elems, cachegen)
		return n, wrapError(err, "full")
	default:
		return nil, fmt.Errorf("invalid number of list elements: %v", c)
	}
}

// decodeShort decode the input elems to a shortNode
func decodeShort(hash, elems []byte, cachegen uint16) (node, error) {
	// Use rlp to have the first string element as type []byte from elems
	kbuf, rest, err := rlp.SplitString(elems)
	if err != nil {
		return nil, err
	}
	// parse the input flags to nodeFlag
	flag := nodeFlag{hash: hash, gen: cachegen}
	key := compactToHex(kbuf)
	if hasTerm(key) {
		// Key has termination 0x10, meaning the node is a leaf node pointing to valueNode.
		val, _, err := rlp.SplitString(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid value node: %v", err)
		}
		return &shortNode{key, append(valueNode{}, val...), flag}, nil
	}
	// Key does not have termination, meaning the node is an extension node, or hash node.
	// Decode the reference to node value
	r, _, err := decodeRef(rest, cachegen)
	if err != nil {
		return nil, wrapError(err, "val")
	}
	return &shortNode{key, r, flag}, nil
}

// decodeFull decode a fullNode from bytes
func decodeFull(hash, elems []byte, cachegen uint16) (*fullNode, error) {
	n := &fullNode{flags: nodeFlag{hash: hash, gen: cachegen}}
	for i := 0; i < 16; i++ {
		cld, rest, err := decodeRef(elems, cachegen)
		if err != nil {
			return n, wrapError(err, fmt.Sprintf("[%d]", i))
		}
		n.Children[i], elems = cld, rest
	}
	val, _, err := rlp.SplitString(elems)
	if err != nil {
		return n, err
	}
	if len(val) > 0 {
		// The branchnode itself has got a value node.
		n.Children[16] = append(valueNode{}, val...)
	}
	return n, nil
}

// decodeRef decode the reference of a node.
// If value is list, nested node reference. Decode the node recursively.
// If value is string, empty or hash node.]
func decodeRef(buf []byte, cachegen uint16) (node, []byte, error) {
	// read the first element of buf into kind and val
	kind, val, rest, err := rlp.Split(buf)
	if err != nil {
		return nil, buf, err
	}
	switch {
	case kind == rlp.List:
		// Nested node reference. The encoding must be smaller
		// than a hash in order to be valid.
		if size := len(buf) - len(rest); size > hashLen {
			err := fmt.Errorf("oversized embedded node (size is %d bytes, want size < %d", size, hashLen)
			return nil, buf, err
		}
		// Decode the first reference
		n, err := decodeNode(nil, buf, cachegen)
		return n, rest, err
	case kind == rlp.String && len(val) == 0:
		// empty node
		return nil, rest, err
	case kind == rlp.String && len(val) == 32:
		// hash node. string type in value
		return append(hashNode{}, val...), rest, nil
	default:
		return nil, nil, fmt.Errorf("invalid RLP string size %d (want 0 or 32)", len(val))
	}
}
