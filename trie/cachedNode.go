package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
)

// cachedNode.go defines a list of cached node structs used for node info stored in memory.

// Structs:
//  rawNode:   			Collapsed trie nodes []byte

//  rawFullNode:		Collapsed fullNode [17]node
//		EncodeRLP(w)  	RLP encode the list of nodes

//  rawShortNode:   	Collapsed shortNode Key-Value Pair

//  cachedNode:     	Cached entry saved in Database. Containing cached collapsed node and
//							cache info, including size, parents, children, ...
//		rlp()			Encode n.node to blob
//      obj()			Expand the node recursively to shortNode/fullNode/valueNode/hashNode
// 		childs()		Return n.children + hashNode (recursively)

// Methods:
// 	simplifyNode(n)		Simplify n and its children recursively. Return rawShortNode if
// 							shortNode, rawFullNode if fullNode.
//	expandNode(n)		Expand n and its children recursively. Return rawFullNode/rawShortNode.

type (
	// rawNode is a simple binary blob used to differentiate between collapsed trie
	// nodes and already encoded RLP binary blobs (while at the same time store them
	// in the same cache fields).
	rawNode []byte

	// rawFullNode represents only the useful data content of a full node, with the
	// caches and flags stripped out to minimize its data storage. This type honors
	// the same RLP encoding as the original parent.
	rawFullNode [17]node

	// rawShortNode represents only the useful data content of a short node, with the
	// caches and flags stripped out to minimize its data storage. This type honors
	// the same RLP encoding as the original parent.
	rawShortNode struct {
		Key []byte
		Val node
	}

	// cachedNode is all the information we know about a single cached node in the
	// memory database write layer.
	cachedNode struct {
		node node   // Cached collapsed trie node, or raw rlp data
		size uint16 // Byte size of the useful cached data

		parents  uint32                 // Number of live nodes referencing this one
		children map[common.Hash]uint16 // External children referenced by this node

		flushPrev common.Hash // Previous node in the flush-list
		flushNext common.Hash // Next node in the flush-list
	}
)

// rawNode need to implement the node interface
func (n rawNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

// fawFullNode need to implement the node interface
func (n rawFullNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawFullNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawFullNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

// EncodeRLP encode the rawFullNode and write to input param w
func (n rawFullNode) EncodeRLP(w io.Writer) error {
	var nodes [17]node
	for i, child := range n {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	return rlp.Encode(w, nodes)
}

// rawShortNode need to implement the nodei nterface
func (n rawShortNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawShortNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawShortNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

// rlp returns the raw rlp encoded blob of the cached node, either directly from
// the cache, or by regenerating it from the collapsed node. The result will not
// have information about the cache details.
func (n *cachedNode) rlp() []byte {
	// if n.node could be transformed to rawNode, simply return node
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	// Else use encode using rlp to bytes
	blob, err := rlp.EncodeToBytes(n.node)
	if err != nil {
		panic(err)
	}
	return blob
}

// obj returns the decoded and expanded trie node from n.node, either directly from
// the cache, or by regenerating it from the rlp encoded blob.
func (n *cachedNode) obj(hash common.Hash, cachegen uint16) node {
	if node, ok := n.node.(rawNode); ok {
		return mustDecodeNode(hash[:], node, cachegen)
	}
	return expandNode(hash[:], n.node, cachegen)
}

// childs returns all the tracked children of this node, both the implicit ones
// from inside the node as well as the explicit ones from outside the node.
func (n *cachedNode) childs() []common.Hash {
	children := make([]common.Hash, 0, 16)
	for child := range n.children {
		children = append(children, child)
	}
	if _, ok := n.node.(rawNode); !ok {
		gatherChildren(n.node, &children)
	}
	return children
}

// gatherChildren traverses the node hierarchy of a collapsed storage node and
// retrieves all the hashnode children.
func gatherChildren(n node, children *[]common.Hash) {
	switch n := n.(type) {
	case *rawShortNode:
		gatherChildren(n.Val, children)

	case rawFullNode:
		for i := 0; i < 16; i++ {
			gatherChildren(n[i], children)
		}
	case hashNode:
		*children = append(*children, common.BytesToHash(n))

	case valueNode, nil:

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// simplifyNode traverses the hierarchy of an expanded memory node and discards
// all the internal caches, returning a node that only contains the raw data.
func simplifyNode(n node) node {
	switch n := n.(type) {
	case *shortNode:
		// Short nodes discard the flags and cascade
		return &rawShortNode{Key: n.Key, Val: simplifyNode(n.Val)}
	case *fullNode:
		// Full nodes discard the flags and cascade
		node := rawFullNode(n.Children)
		for i := 0; i < len(node); i++ {
			if node[i] != nil {
				node[i] = simplifyNode(node[i])
			}
		}
		return node
	case valueNode, hashNode, rawNode:
		return n
	default:
		panic(fmt.Sprintf("unknown node type %T", n))
	}
}

// expandNode traverses the node hierarchy of a collapsed storage node and converts
//// all fields and keys into expanded memory form.
func expandNode(hash hashNode, n node, cachegen uint16) node {
	switch n := n.(type) {
	case *rawShortNode:
		// short node need key and child expansion
		return &shortNode{
			Key: compactToHex(n.Key),
			Val: expandNode(nil, n.Val, cachegen),
			flags: nodeFlag{
				hash: hash,
				gen:  cachegen,
			},
		}

	case rawFullNode:
		// Full nodes need child expansion
		node := &fullNode{
			flags: nodeFlag{
				hash: hash,
				gen:  cachegen,
			},
		}
		for i := 0; i < len(node.Children); i++ {
			if n[i] != nil {
				node.Children[i] = expandNode(nil, n[i], cachegen)
			}
		}
		return node

	case valueNode, hashNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}
