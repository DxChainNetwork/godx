// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package enode

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Keys in the node database.
const (
	dbVersionKey = "version" // Version of the database to flush if changes
	dbItemPrefix = "n:"      // Identifier to prefix node entries with

	dbDiscoverRoot      = ":discover"
	dbDiscoverSeq       = dbDiscoverRoot + ":seq"
	dbDiscoverPing      = dbDiscoverRoot + ":lastping"
	dbDiscoverPong      = dbDiscoverRoot + ":lastpong"
	dbDiscoverFindFails = dbDiscoverRoot + ":findfail"
	dbLocalRoot         = ":local"
	dbLocalSeq          = dbLocalRoot + ":seq"
)

var (
	dbNodeExpiration = 24 * time.Hour // Time after which an unseen node should be dropped.
	dbCleanupCycle   = time.Hour      // Time period for running the expiration task.
	dbVersion        = 7
)

// DB is the node database, storing previously seen nodes and any collected metadata about
// them for QoS purposes.
type DB struct {
	// this will either be memory or persistent db
	lvl    *leveldb.DB // Interface to the database itself
	runner sync.Once   // Ensures we can start at most one expirer, which is used to check
	// if data stored in the db will be expired
	quit chan struct{} // Channel to signal the expiring thread to stop (stop running expierer)
}

// OpenDB opens a node database for storing and retrieving infos about known peers in the
// network. If no path is given an in-memory, temporary database is constructed.
//
// NOTE: if the db existed, it will open the db only, otherwise, it will create and then
// open the db. For the leveldb, the key is byte slice, and the blob is byte slice as well
func OpenDB(path string) (*DB, error) {
	if path == "" {
		return newMemoryDB()
	}
	return newPersistentDB(path)
}

// newMemoryNodeDB creates a new in-memory node database without a persistent backend.
func newMemoryDB() (*DB, error) {
	// opens and create the DB for given storage
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &DB{lvl: db, quit: make(chan struct{})}, nil
}

// newPersistentNodeDB creates/opens a leveldb backed persistent node database,
// also flushing its contents in case of a version mismatch.
//
// NOTE: it checks if the db version matches, if not, delete old db
// and re-create new db
func newPersistentDB(path string) (*DB, error) {
	// allows max 5 files to store in system cache
	opts := &opt.Options{OpenFilesCacheCapacity: 5}
	// opens or creates db for the given path
	db, err := leveldb.OpenFile(path, opts)
	if _, iscorrupted := err.(*errors.ErrCorrupted); iscorrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}
	if err != nil {
		return nil, err
	}
	// The nodes contained in the cache correspond to a certain protocol version.
	// Flush all nodes if the version doesn't match.
	currentVer := make([]byte, binary.MaxVarintLen64)
	currentVer = currentVer[:binary.PutVarint(currentVer, int64(dbVersion))]

	// get the dbVersion from the database, insert the dbVersion if not found
	// otherwise, compare the dbVersion with current version, if they are not equal,
	// remove everything and create a new persistentDB
	blob, err := db.Get([]byte(dbVersionKey), nil)
	switch err {
	case leveldb.ErrNotFound:
		// Version not found (i.e. empty cache), insert it
		if err := db.Put([]byte(dbVersionKey), currentVer, nil); err != nil {
			db.Close()
			return nil, err
		}

	case nil:
		// Version present, flush if different
		if !bytes.Equal(blob, currentVer) {
			db.Close()
			if err = os.RemoveAll(path); err != nil {
				return nil, err
			}
			return newPersistentDB(path)
		}
	}
	return &DB{lvl: db, quit: make(chan struct{})}, nil
}

// makeKey generates the leveldb key-blob from a node id and its particular
// field of interest.
// this is the database key contains prefix n:, node ID, and field
// format: n:IDfield if it is the node key
func makeKey(id ID, field string) []byte {
	if (id == ID{}) {
		return []byte(field)
	}
	return append([]byte(dbItemPrefix), append(id[:], field...)...)
}

// splitKey tries to split a database key into a node id and a field part.
func splitKey(key []byte) (id ID, field string) {
	// If the key is not of a node, return it plainly
	if !bytes.HasPrefix(key, []byte(dbItemPrefix)) {
		return ID{}, string(key)
	}
	// Otherwise split the id and field
	item := key[len(dbItemPrefix):]
	copy(id[:], item[:len(id)])
	field = string(item[len(id):])

	return id, field
}

// fetchInt64 retrieves an integer associated with a particular key.
// Varint decode the data to the largest available signed integer types (int640
func (db *DB) fetchInt64(key []byte) int64 {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return 0
	}
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

// storeInt64 stores an integer in the given key.
// PutVarint encode int64 to byte slice
func (db *DB) storeInt64(key []byte, n int64) error {
	// max length allowed is 10
	blob := make([]byte, binary.MaxVarintLen64)
	// resize the blob to the size of data
	// do not want to waste space
	blob = blob[:binary.PutVarint(blob, n)]
	return db.lvl.Put(key, blob, nil)
}

// fetchUint64 retrieves an integer associated with a particular key.
// Uvarint decode the data to the largest available unsigned integer types (uint64)
func (db *DB) fetchUint64(key []byte) uint64 {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return 0
	}
	val, _ := binary.Uvarint(blob)
	return val
}

// storeUint64 stores an integer in the given key.
// PutVarint encode uint64 to byte slice
//
// convert uint64 to byte slice and stores into db based on the given key
func (db *DB) storeUint64(key []byte, n uint64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutUvarint(blob, n)]
	return db.lvl.Put(key, blob, nil)
}

// Node retrieves a node with a given id from the database.
func (db *DB) Node(id ID) *Node {
	// rlp encoded Node Record database key: nodeid:discover
	// NOTE, blob contains node record only, key is the combination of
	// node id + dbDiscoverRoot
	blob, err := db.lvl.Get(makeKey(id, dbDiscoverRoot), nil)
	if err != nil {
		return nil
	}
	return mustDecodeNode(id[:], blob)
}

// using rlp decoding method to decode data to Node type
func mustDecodeNode(id, data []byte) *Node {
	node := new(Node)
	if err := rlp.DecodeBytes(data, &node.r); err != nil {
		panic(fmt.Errorf("p2p/enode: can't decode node %x in DB: %v", id, err))
	}
	// Restore node id cache.
	copy(node.id[:], id)
	return node
}

// UpdateNode inserts - potentially overwriting - a node into the peer database.
// update both node record and node sequence number (node record is encoded by RLP)
// node sequence number is used to indicate if the record is the newest record
func (db *DB) UpdateNode(node *Node) error {
	if node.Seq() < db.NodeSeq(node.ID()) {
		return nil
	}
	blob, err := rlp.EncodeToBytes(&node.r)
	if err != nil {
		return err
	}
	if err := db.lvl.Put(makeKey(node.ID(), dbDiscoverRoot), blob, nil); err != nil {
		return err
	}
	return db.storeUint64(makeKey(node.ID(), dbDiscoverSeq), node.Seq())
}

// NodeSeq returns the stored record sequence number of the given node.
func (db *DB) NodeSeq(id ID) uint64 {
	return db.fetchUint64(makeKey(id, dbDiscoverSeq))
}

// Resolve returns the stored record of the node if it has a larger sequence
// number than n.
//
// if the node sequence is greater than the node sequence stored in DB, return the original node
// otherwise, return the node from the db
// Resolve returns the node with newest node record sequence
func (db *DB) Resolve(n *Node) *Node {
	if n.Seq() > db.NodeSeq(n.ID()) {
		return n
	}
	return db.Node(n.ID())
}

// DeleteNode deletes all information/keys associated with a node.
// any key has n:nodeID as prefix
func (db *DB) DeleteNode(id ID) error {
	deleter := db.lvl.NewIterator(util.BytesPrefix(makeKey(id, "")), nil)
	for deleter.Next() {
		if err := db.lvl.Delete(deleter.Key(), nil); err != nil {
			return err
		}
	}
	return nil
}

// ensureExpirer is a small helper method ensuring that the data expiration
// mechanism is running. If the expiration goroutine is already running, this
// method simply returns.
//
// The goal is to start the data evacuation only after the network successfully
// bootstrapped itself (to prevent dumping potentially useful seed nodes). Since
// it would require significant overhead to exactly trace the first successful
// convergence, it's simpler to "ensure" the correct state when an appropriate
// condition occurs (i.e. a successful bonding), and discard further events.
func (db *DB) ensureExpirer() {
	db.runner.Do(func() { go db.expirer() })
}

// expirer should be started in a go routine, and is responsible for looping ad
// infinitum and dropping stale data from the database.
// the codes on checking data expiration will run every hour
func (db *DB) expirer() {
	tick := time.NewTicker(dbCleanupCycle)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := db.expireNodes(); err != nil {
				log.Error("Failed to expire nodedb items", "err", err)
			}
		case <-db.quit:
			return
		}
	}
}

// expireNodes iterates over the database and deletes all nodes that have not
// been seen (i.e. received a pong from) for some allotted time.
func (db *DB) expireNodes() error {
	// one day before
	threshold := time.Now().Add(-dbNodeExpiration)

	// Find discovered nodes that are older than the allowance
	// create iterator to iterate through database content
	it := db.lvl.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
		// Skip the item if not a discovery node
		id, field := splitKey(it.Key())
		if field != dbDiscoverRoot {
			continue
		}
		// Skip the node if not expired yet (and not self)
		// check if pong time is after threshold (which is one day before)
		if seen := db.LastPongReceived(id); seen.After(threshold) {
			continue
		}
		// Otherwise delete all associated information
		db.DeleteNode(id)
	}
	return nil
}

// LastPingReceived retrieves the time of the last ping packet received from
// a remote node.
func (db *DB) LastPingReceived(id ID) time.Time {
	return time.Unix(db.fetchInt64(makeKey(id, dbDiscoverPing)), 0)
}

// UpdateLastPingReceived updates the last time we tried contacting a remote node.
func (db *DB) UpdateLastPingReceived(id ID, instance time.Time) error {
	return db.storeInt64(makeKey(id, dbDiscoverPing), instance.Unix())
}

// LastPongReceived retrieves the time of the last successful pong from remote node.
func (db *DB) LastPongReceived(id ID) time.Time {
	// Launch expirer
	db.ensureExpirer()
	return time.Unix(db.fetchInt64(makeKey(id, dbDiscoverPong)), 0)
}

// UpdateLastPongReceived updates the last pong time of a node.
func (db *DB) UpdateLastPongReceived(id ID, instance time.Time) error {
	return db.storeInt64(makeKey(id, dbDiscoverPong), instance.Unix())
}

// FindFails retrieves the number of findnode failures since bonding.
func (db *DB) FindFails(id ID) int {
	return int(db.fetchInt64(makeKey(id, dbDiscoverFindFails)))
}

// UpdateFindFails updates the number of findnode failures since bonding.
func (db *DB) UpdateFindFails(id ID, fails int) error {
	return db.storeInt64(makeKey(id, dbDiscoverFindFails), int64(fails))
}

// LocalSeq retrieves the local record sequence counter.
// for localnode
func (db *DB) localSeq(id ID) uint64 {
	return db.fetchUint64(makeKey(id, dbLocalSeq))
}

// storeLocalSeq stores the local record sequence counter.
func (db *DB) storeLocalSeq(id ID, n uint64) {
	db.storeUint64(makeKey(id, dbLocalSeq), n)
}

// QuerySeeds retrieves random nodes to be used as potential seed nodes
// for bootstrapping.
// n defines how many bootstrap nodes want to retrieve
// 5 * n defines the max number of loops can run, after this time, even if I
// did not get the desired number of nodes, still exit the for loop
// maxAge is used to determine if the node is active
// based on the last pong message received
func (db *DB) QuerySeeds(n int, maxAge time.Duration) []*Node {
	var (
		now   = time.Now()
		nodes = make([]*Node, 0, n)
		it    = db.lvl.NewIterator(nil, nil)
		id    ID
	)
	defer it.Release()

seek:
	for seeks := 0; len(nodes) < n && seeks < n*5; seeks++ {
		// Seek to a random entry. The first byte is incremented by a
		// random amount each time in order to increase the likelihood
		// of hitting all existing nodes in very small databases.
		ctr := id[0]

		// get random id
		rand.Read(id[:])

		// replace the first byte
		id[0] = ctr + id[0]%16

		// return whether the pair with key is greater or equal
		// to the key passed in exist
		it.Seek(makeKey(id, dbDiscoverRoot))

		n := nextNode(it)
		if n == nil {
			id[0] = 0
			continue seek // iterator exhausted
		}

		// if the node is not as active as expected, skip
		if now.Sub(db.LastPongReceived(n.ID())) > maxAge {
			continue seek
		}
		// if the node already existed in the node list, skip
		for i := range nodes {
			if nodes[i].ID() == n.ID() {
				continue seek // duplicate
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// reads the next node record from the iterator, skipping over other
// database entries.
func nextNode(it iterator.Iterator) *Node {
	for end := false; !end; end = !it.Next() {
		id, field := splitKey(it.Key())
		// only return field with dbDiscoverRoot
		if field != dbDiscoverRoot {
			continue
		}
		return mustDecodeNode(id[:], it.Value())
	}
	return nil
}

// close flushes and closes the database files.
func (db *DB) Close() {
	close(db.quit)
	db.lvl.Close()
}
