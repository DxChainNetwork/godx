package trie

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/metrics"
	"github.com/allegro/bigcache"
	"sync"
	"time"
)

// Database.go defines the struct Database and its method

// Structs:
// 	Database: 							Memory database. Responsible for memory to ethdb interface.
//		DiskDB():						Return the diskdb associated with db
// 		InsertBlob(hash, blob):			Insert the blob as cachedNode{rawNode} into db.dirties
// 		Node(hash):						Retrieve node of as rlp encoded byte
//		Nodes():						Retrieve hashes of all nodes in db.dirties
// 		Reference(child, parent):		Reference adds a new reference from a parent node to a child node
// 											Update child & parent's cache info if child in db.dirties.
//		Dereference(root):				Dereference removes an existing reference from the input root node.
//											The function will do recursively when node.parents == 0 after dereference.
//		Cap(limit):						Flush preimages if db.preimageSize > 4MB. Flush db.oldest until
//											memorySize is smaller than limit.
//		Commit(node, report):			Flush all db.preimages, write node and all its child nodes to diskdb and
//											update cached info for node and all its children.
//		Size()							Return the total memory size of Database
//
//		insert(hash, blob, node): 		Insert cachedNode{simplifyNode(node)} into db.dirties
// 		node(hash, cachegen): 			Retrieve node as an expandedNode. Find from db.cleans, next
// 											db.dirties, finally from ethdb
//		preimage(hash):					Retrive a cached pre-image from db.preimages
//		insertPreimage(hash, preimage): Add preimage to db.preimages
//    	preimage(hash):					Retrieves the preimage from db.preimages, then diskdb
//		verifyIntegrity():				Verify whether all dirties could be reached from root

// Methods:
//	NewDatabase(ethdb):					A new database with no cleans cache
//  NewDatabaseWithCache(ethdb, cache):	A new database with cleans cache

// Database is an intermediate write layer between the trie data structures and
// the disk database. The aim is to accumulate trie writes in-memory and only
// periodically flush a couple tries to disk, garbage collecting the remainder.
type Database struct {
	diskdb ethdb.Database // Persistent storage for matured trie nodes

	cleans  *bigcache.BigCache          // GC friendly memory cache of clean node RLPs.
	dirties map[common.Hash]*cachedNode // Data and references relationships of dirty nodes
	oldest  common.Hash                 // Oldest tracked node, flush-list head
	newest  common.Hash                 // Newest tracked node, flush-list tail

	preimages map[common.Hash][]byte // Preimages of nodes from the secure trie
	seckeybuf [secureKeyLength]byte  // Ephemeral buffer for calculating preimage keys

	gctime  time.Duration      // Time spent on garbage collection since last commit
	gcnodes uint64             // Nodes garbage collected since last commit
	gcsize  common.StorageSize // Data storage garbage collected since last commit

	flushtime  time.Duration      // Time spent on data flushing since last commit
	flushnodes uint64             // Nodes flushed since last commit
	flushsize  common.StorageSize // Data storage flushed since last commit

	dirtiesSize   common.StorageSize // Storage size of the dirty node cache (exc. flushlist)
	preimagesSize common.StorageSize // Storage size of the preimages cache

	lock sync.RWMutex
}

var (
	// secureKeyPrefix is the database key prefix used to store trie node preimages.
	secureKeyPrefix = []byte("secure-key-")

	// Register the meters for metrics
	memcacheCleanHitMeter   = metrics.NewRegisteredMeter("trie/memcache/clean/hit", nil)
	memcacheCleanMissMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/miss", nil)
	memcacheCleanReadMeter  = metrics.NewRegisteredMeter("trie/memchache/clean/read", nil)
	memcacheCleanWriteMeter = metrics.NewRegisteredMeter("trie/memcache/clean/write", nil)

	// Register flush metrics
	memcacheFlushTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/flush/time", nil)
	memcacheFlushNodesMeter = metrics.NewRegisteredMeter("trie/memcache/flush/nodes", nil)
	memcacheFlushSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/flush/size", nil)

	// Register garbage collection metrics
	memcacheGCTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/gc/time", nil)
	memcacheGCNodesMeter = metrics.NewRegisteredMeter("trie/memcache/gc/nodes", nil)
	memcacheGCSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/gc/size", nil)

	// Register Commit metrics
	memcacheCommitTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/commit/time", nil)
	memcacheCommitNodesMeter = metrics.NewRegisteredMeter("trie/memcache/commit/nodes", nil)
	memcacheCommitSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/commit/size", nil)
)

const secureKeyLength = 11 + 32

// DatabaseReader wraps the Get and Has method of a backing store for the trie.
type DatabaseReader interface {
	// Get retrieves the value associated with key from the database.
	Get(key []byte) (value []byte, err error)

	// Has retrieves whether a key is present in the database.
	Has(key []byte) (bool, error)
}

// NewDatabase creates a new trie database to store ephemeral trie content before
// its written out to disk or garbage collected. No read cache is created, so all
// data retrievals will hit the underlying disk database.
func NewDatabase(diskdb ethdb.Database) *Database {
	return NewDatabaseWithCache(diskdb, 0)
}

// NewDatabaseWithCache creates a new trie database to store ephemeral trie content
// before its written out to disk or garbage collected. It also acts as a read cache
// for nodes loaded from disk.
func NewDatabaseWithCache(diskdb ethdb.Database, cache int) *Database {
	var cleans *bigcache.BigCache
	if cache > 0 {
		cleans, _ = bigcache.NewBigCache(bigcache.Config{
			Shards:             1024,
			LifeWindow:         time.Hour,
			MaxEntriesInWindow: cache * 1024,
			MaxEntrySize:       512,
			HardMaxCacheSize:   cache,
		})
	}
	return &Database{
		diskdb:    diskdb,
		cleans:    cleans,
		dirties:   map[common.Hash]*cachedNode{{}: {}},
		preimages: make(map[common.Hash][]byte),
	}
}

// DiskDB retrieves the persistent storage backing the trie database.
func (db *Database) DiskDB() DatabaseReader {
	return db.diskdb
}

// InsertBlob writes a new reference tracked blob to the memory database if it's
// yet unknown. This method should only be used for non-trie nodes that require
// reference counting, since trie nodes are garbage collected directly through
// their embedded children.
func (db *Database) InsertBlob(hash common.Hash, blob []byte) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.insert(hash, blob, rawNode(blob))
}

// Node retrieves an encoded cached trie node from memory. If it cannot be found
// cached, the method queries the persistent database for the content.
// The returned node is rlp encoded.
func (db *Database) Node(hash common.Hash) ([]byte, error) {
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc, err := db.cleans.Get(string(hash[:])); err == nil && enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return enc, nil
		}
	}
	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.rlp(), nil
	}

	// Content unavailable in memory, attempt to retrieve from disk
	enc, err := db.diskdb.Get(hash[:])
	if err == nil && enc != nil {
		if db.cleans != nil {
			db.cleans.Set(string(hash[:]), enc)
			memcacheCleanMissMeter.Mark(1)
			memcacheCleanWriteMeter.Mark(int64(len(enc)))
		}
	}
	return enc, err
}

// Nodes retrieves the hashes of all the nodes cached within the memory database.
// This method is extremely expensive and should only be used to validate internal
// states in test code.
func (db *Database) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.dirties))
	for hash := range db.dirties {
		if hash != (common.Hash{}) { // Special case for "root" references/nodes
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

// Reference adds a new reference from a parent node to a child node
func (db *Database) Reference(child common.Hash, parent common.Hash) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	db.reference(child, parent)
}

// Dereference removes an existing reference from a root node
// The function calls the dereference function and do the metrics calculation
func (db *Database) Dereference(root common.Hash) {
	// sanity check to ensure that the meta-root is not removed
	if root == (common.Hash{}) {
		log.Error("Attempted to dereference the trie cache meta root")
		return
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	db.dereference(root, common.Hash{})

	db.gcnodes += uint64(nodes - len(db.dirties))
	db.gcsize += storage - db.dirtiesSize
	db.gctime += time.Since(start)

	memcacheGCTimeTimer.Update(time.Since(start))
	memcacheGCSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheGCNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Dereferenced trie from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)
}

// Cap iteratively flushed old but still referenced trie nodes until the total
// memory usage goes below the given threshold
func (db *Database) Cap(limit common.StorageSize) error {
	// Create a database batch to flush persistent data out. It is important that
	// outside code doesn't see an inconsistent state (referenced data removed fom
	// memory cache during commit but not yet in persistent storage). This is ensured
	// by only uncaching existing data when the database write finalizes.
	db.lock.RLock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	batch := db.diskdb.NewBatch()

	// db.dirtiesSize only contains the useful data in the cache, but when reporting
	// the total memory consumption, the maintenance metadata is also needed to be
	// counted. For every useful node, we track 2 extra hashes as the flushlist.
	size := db.dirtiesSize + common.StorageSize((len(db.dirties)-1)*2*common.HashLength)

	// If the preimage cache got large enough, push to disk. If it's still small
	// leave for later to duplicate writes
	flushPreimages := db.preimagesSize > 4*1024*1024
	if flushPreimages {
		for hash, preimage := range db.preimages {
			if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
				log.Error("Failed to commit preimage from trie database", "err", err)
				db.lock.RUnlock()
				return err
			}
			if batch.ValueSize() > ethdb.IdealBatchSize {
				if err := batch.Write(); err != nil {
					db.lock.RUnlock()
					return err
				}
				batch.Reset()
			}
		}
	}
	// Keep committing nodes from the flush-list until we are below allowance
	oldest := db.oldest
	for size > limit && oldest != (common.Hash{}) {
		// Fetch the oldest referenced node and push into the batch
		node := db.dirties[oldest]
		if err := batch.Put(oldest[:], node.rlp()); err != nil {
			db.lock.RUnlock()
			return err
		}
		// if we exceeded the ideal batch size, commit and reset
		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				log.Error("Failed to write flush list to disk", "err", err)
				db.lock.RUnlock()
				return err
			}
			batch.Reset()
		}
		// Iterate to the next flush item, or abort if the size cap was achieved. Size
		// is the total size, including both the useful cached data (hash -> blob), as
		// well as the flushlist metadata (2*hash). When flushing items from the cache,
		// we need to reduce both.
		size -= common.StorageSize(3*common.HashLength + int(node.size))
		oldest = node.flushNext
	}
	// Flush out any remainder data from the last batch
	if err := batch.Write(); err != nil {
		log.Error("Failed to write flush lsit to disk", "err", err)
		db.lock.RUnlock()
		return err
	}
	db.lock.RUnlock()

	//Write successful, clear out the flushed data
	db.lock.Lock()
	defer db.lock.Unlock()

	if flushPreimages {
		db.preimages = make(map[common.Hash][]byte)
		db.preimagesSize = 0
	}
	for db.oldest != oldest {
		node := db.dirties[db.oldest]
		delete(db.dirties, db.oldest)
		db.oldest = node.flushNext

		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
	}
	if db.oldest != (common.Hash{}) {
		db.dirties[db.oldest].flushPrev = common.Hash{}
	}
	db.flushnodes += uint64(nodes - len(db.dirties))
	db.flushsize += storage - db.dirtiesSize
	db.flushtime += time.Since(start)

	memcacheFlushTimeTimer.Update(time.Since(start))
	memcacheFlushSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheFlushNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Persisted nodes from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"flushnodes", db.flushnodes, "flushsize", db.flushsize, "flushtime", db.flushtime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	return nil
}

// Commit iterates over all the children of a particular node, writes them out
// to disk, forcefully tearing down all references in both directions.
//
// As a side effect, all pre-images accumulated up to this point are also written
func (db *Database) Commit(node common.Hash, report bool) error {
	// Create a database batch to flush persistent data out. It is important that
	// outside code doesn't see an inconsistent state (referenced data removed from
	// memory cache during commit but not yet in persistent storage). This is ensured
	// by only uncaching existing data when the database write finalizes.
	db.lock.RLock()

	start := time.Now()
	batch := db.diskdb.NewBatch()

	// Move all of the accumulated preimages into a write batch
	for hash, preimage := range db.preimages {
		if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
			log.Error("Failed to commit preimage from trie database", "err", err)
			db.lock.RUnlock()
			return err
		}
		if batch.ValueSize() > ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}
	}
	// Move the trie itself into the batch, flushing if enough data is accumulated
	nodes, storage := len(db.dirties), db.dirtiesSize
	if err := db.commit(node, batch); err != nil {
		log.Error("Failed to commit trie from trie database", "err", err)
		db.lock.RUnlock()
		return err
	}
	// Write batch ready, unlock for readers during persistence
	if err := batch.Write(); err != nil {
		log.Error("Failed to write trie to disk", "err", err)
		db.lock.RUnlock()
		return err
	}
	db.lock.RUnlock()

	// Write successful, clear out the flushed data
	db.lock.Lock()
	defer db.lock.Unlock()

	db.preimages = make(map[common.Hash][]byte)
	db.preimagesSize = 0

	db.uncache(node)

	memcacheCommitTimeTimer.Update(time.Since(start))
	memcacheCommitSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheCommitNodesMeter.Mark(int64(nodes - len(db.dirties)))

	logger := log.Info
	if !report {
		logger = log.Debug
	}
	logger("Persisted trie from memory database", "nodes", nodes-len(db.dirties)+int(db.flushnodes), "size", storage-db.dirtiesSize+db.flushsize, "time", time.Since(start)+db.flushtime,
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	// Reset the garbage collection statistics
	db.gcnodes, db.gcsize, db.gctime = 0, 0, 0
	db.flushnodes, db.flushsize, db.flushtime = 0, 0, 0

	return nil
}

// Size returns the current storage size of the memory cache in front of the
// persistent database layer
func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	// db.dirtiesSize only contains the useful data in the cache, but when reporting
	// the total memory consumption, the maintenance metadata is also needed to be
	// counted. For every useful node, we track 2 extra hashes in the flushlist.
	flushlistSize := common.StorageSize((len(db.dirties) - 1) * 2 * common.HashLength)
	return db.dirtiesSize + flushlistSize, db.preimagesSize
}

// insert inserts a collapsed trie node into the memory database. This method is
// a more generic version of InsertBlob, supporting both raw blob insertions as
// well as trie node insertions. The blob must always be specified to allow proper
// size tracking.
// insert a blob with hash, data, node, create the entry and add to db.dirties.
// finally update the related nodes in parents field, and flushNext of the last
// node in flush list.
func (db *Database) insert(hash common.Hash, blob []byte, node node) {
	// If the node's already cached, skip
	if _, ok := db.dirties[hash]; ok {
		return
	}
	// Create the cached entry for this node
	entry := &cachedNode{
		node:      simplifyNode(node),
		size:      uint16(len(blob)),
		flushPrev: db.newest,
	}
	for _, child := range entry.childs() {
		if c := db.dirties[child]; c != nil {
			c.parents++
		}
	}
	db.dirties[hash] = entry

	// Update the flush-list endpoints
	if db.oldest == (common.Hash{}) {
		// db just initialized and is empty
		db.oldest, db.newest = hash, hash
	} else {
		// update the latest entry, and newest entry should be the one just inserted
		db.dirties[db.newest].flushNext, db.newest = hash, hash
	}
	// update the dirties size based on hashlength + entry size
	db.dirtiesSize += common.StorageSize(common.HashLength + entry.size)
}

// node retrieves a cached trie node from memory, or returns nil if none can be
// found in the memory cache.
func (db *Database) node(hash common.Hash, cachegen uint16) node {
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc, err := db.cleans.Get(string(hash[:])); err == nil && enc != nil {
			// Found the node in the clean cache
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return mustDecodeNode(hash[:], enc, cachegen)
		}
	}
	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()
	if dirty != nil {
		return dirty.obj(hash, cachegen)
	}
	// Content unavailable in memory, attempt to retrieve from disk
	enc, err := db.diskdb.Get(hash[:])
	if err != nil || enc == nil {
		return nil
	}
	if db.cleans != nil {
		db.cleans.Set(string(hash[:]), enc)
		memcacheCleanMissMeter.Mark(1)
		memcacheCleanWriteMeter.Mark(int64(len(enc)))
	}
	return mustDecodeNode(hash[:], enc, cachegen)
}

// insertPreimage writes a new trie node pre-image to the memory database if it's
// yet unknown. The method will make a copy of the slice.
//
// Note, this method assumes that the database's lock is held!
func (db *Database) insertPreimage(hash common.Hash, preimage []byte) {
	if _, ok := db.preimages[hash]; ok {
		return
	}
	// update the db info
	db.preimages[hash] = common.CopyBytes(preimage)
	db.preimagesSize += common.StorageSize(common.HashLength + len(preimage))
}

// preimage retrieves a cached trie node pre-image from memory. If it cannot be
// found cached, the method queries the persistent database for the content.
func (db *Database) preimage(hash common.Hash) ([]byte, error) {
	// Retrieve the node from cache if available
	db.lock.RLock()
	preimage := db.preimages[hash]
	db.lock.RUnlock()

	if preimage != nil {
		return preimage, nil
	}
	// Content unavailable in memory, attempt to retrieve from disk
	return db.diskdb.Get(db.secureKey(hash[:]))
}

// secureKey returns the database key for the preimage of key, as an ephemeral
// buffer. The caller must not hold ronto the return value because it will become
// valid on the next call
func (db *Database) secureKey(key []byte) []byte {
	buf := append(db.seckeybuf[:0], secureKeyPrefix...)
	buf = append(buf, key...)
	return buf
}

// reference adds a new reference from a parent node to a child node
// Actions to be taken if child node is in dirties. Assume parent is already
// in dirtys.
// If the child is not in dirties, do nothing; Else update child and parent's cache info
// parents & children
func (db *Database) reference(child common.Hash, parent common.Hash) {
	// If the node does not exist, it's a node pulled from disk, skip
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// if the reference already exist, only duplicate for roots
	if db.dirties[parent].children == nil {
		db.dirties[parent].children = make(map[common.Hash]uint16)
	} else if _, ok = db.dirties[parent].children[child]; ok && parent != (common.Hash{}) {
		// parent in dirty, and the parent already has this child
		return
	}
	// if parent is root, duplicate the root.children[child]++
	node.parents++
	db.dirties[parent].children[child]++
}

// dereference is the private locked version of Dereference.
// parent must be in db.dirties.
// 1. If parent.children has child, decrement parent.children[child]. If zero, delete the key.
// 2. If child.parents > 0. decrement parent.parent
// 3. If child has no parents,
// 		a. remove child from flush list
// 		b. dereference child.childs() to child
//		c. remove child from db.dirties
func (db *Database) dereference(child common.Hash, parent common.Hash) {
	// Dereference the parent-child
	node := db.dirties[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			// remove the child key from node.children
			delete(node.children, child)
		}
	}
	// If the child does not exist, it's a previously committed node
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If there are no more references to the child, delete it and cascade
	if node.parents > 0 {
		// This is a special cornercase where a node loaded from disk (i.e. not in the
		// memcache any more) gets reinjected as a new node (short node split into full,
		// then reverted into short), causing a cached node to have no parents. That is
		// no problem in itself, but don't make maxint parents out of it.
		node.parents--
	}
	if node.parents == 0 {
		// Remove the node from the flush-list
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.dirties[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.dirties[node.flushPrev].flushNext = common.Hash{}
		default:
			db.dirties[node.flushPrev].flushNext = node.flushNext
			db.dirties[node.flushNext].flushPrev = node.flushPrev
		}
		// Dereference all children and delete the node
		for _, hash := range node.childs() {
			db.dereference(hash, child)
		}
		delete(db.dirties, child)
		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
	}
}

// commit is the private locked version of Commit
func (db *Database) commit(hash common.Hash, batch ethdb.Batch) error {
	// If the node does not exist, it's a previously committed node
	node, ok := db.dirties[hash]
	if !ok {
		return nil
	}
	for _, child := range node.childs() {
		if err := db.commit(child, batch); err != nil {
			return err
		}
	}
	if err := batch.Put(hash[:], node.rlp()); err != nil {
		return err
	}
	// If we've reached an optimal batch size, commit and start over
	if batch.ValueSize() >= ethdb.IdealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		batch.Reset()
	}
	return nil
}

// uncache is the post-processing step of a commit operation where the already
// persisted trie is removed from the cache. The reason behind the two-phase
// commit is to ensure consistent data availability while moving from memory
// to disk
func (db *Database) uncache(hash common.Hash) {
	// If the node does not exist, we are done on this path
	node, ok := db.dirties[hash]
	if !ok {
		return
	}
	// Node still exists, remove it from the flush list
	switch hash {
	case db.oldest:
		db.oldest = node.flushNext
		db.dirties[node.flushNext].flushPrev = common.Hash{}
	case db.newest:
		db.newest = node.flushPrev
		db.dirties[node.flushPrev].flushNext = common.Hash{}
	default:
		db.dirties[node.flushPrev].flushNext = node.flushNext
		db.dirties[node.flushNext].flushPrev = node.flushPrev
	}
	// Uncache the node's subtries and remove the node itself too
	for _, child := range node.childs() {
		db.uncache(child)
	}
	delete(db.dirties, hash)
	db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
}

// verifyIntegrity is debug method to iterate over the entire trie stored in
// memory and check whether every node is reachable from the meta root. The goal
// is to find any errors that might cause memory leaks and or trie nodes to go
// missing.
//
// This method is extremely CPU and memory intensive, only use when must.
func (db *Database) verifyIntegrity() {
	// Iterate over all the cached nodes and accumulate them into a set
	reachable := map[common.Hash]struct{}{{}: {}}

	for child := range db.dirties[common.Hash{}].children {
		db.accumulate(child, reachable)
	}
	// Find any unreachable but cached nodes
	var unreachable []string
	for hash, node := range db.dirties {
		if _, ok := reachable[hash]; !ok {
			unreachable = append(unreachable, fmt.Sprintf("%x: {Node: %v, Parents: %d, Prev: %x, Next: %x}",
				hash, node.node, node.parents, node.flushPrev, node.flushNext))
		}
	}
	if len(unreachable) != 0 {
		panic(fmt.Sprintf("trie cache memory leak: %v", unreachable))
	}
}

// accumulate interates over the trie defined by hash and accumulates all the
// cached children found in memory.
func (db *Database) accumulate(hash common.Hash, reachable map[common.Hash]struct{}) {
	// Mark the node reachable if present in the memory cache
	node, ok := db.dirties[hash]
	if !ok {
		return
	}
	reachable[hash] = struct{}{}

	// Iterate over all the children and accumulate them too
	for _, child := range node.childs() {
		db.accumulate(child, reachable)
	}
}
