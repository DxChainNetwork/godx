// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxdir

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/storage"
)

const threadDepth = 3

type (
	// DirSet is the manager of all DxDirs
	DirSet struct {
		rootDir storage.SysPath
		dirMap  map[storage.DxPath]*dirSetEntry

		lock sync.Mutex
		wal  *writeaheadlog.Wal
	}

	// dirSetEntry is the entry stored in the DirSet. It also keeps a map of current accessing threads
	dirSetEntry struct {
		*DxDir
		dirSet *DirSet

		threadMap     map[threadID]threadInfo
		threadMapLock sync.Mutex
	}

	// DirSetEntryWithID is the entry with the threadID. It extends DxDir
	DirSetEntryWithID struct {
		*dirSetEntry
		threadID threadID
	}

	// threadInfo is the structure of an thread access over a dirSetEntry
	threadInfo struct {
		callingFiles []string
		callingLines []int
		lockTime     time.Time
	}

	threadID uint64
)

// NewDirSet creates a New DirSet with the given parameters. If the root DxDir not exist,
// create a new DxDir
func NewDirSet(rootDir storage.SysPath, wal *writeaheadlog.Wal) (*DirSet, error) {
	ds := &DirSet{
		rootDir: rootDir,
		dirMap:  make(map[storage.DxPath]*dirSetEntry),
		wal:     wal,
	}
	exist := ds.exists(storage.RootDxPath())
	if exist {
		return ds, nil
	}
	_, err := New(storage.RootDxPath(), ds.rootDir, ds.wal)
	if err != nil {
		return nil, err
	}
	return ds, nil

}

// NewDxDir creates a DxDir. Return a DirSetEntryWithID that extends DxDir and the error
func (ds *DirSet) NewDxDir(path storage.DxPath) (*DirSetEntryWithID, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	// Check the directory file already exists
	exist := ds.exists(path)
	if exist {
		return nil, os.ErrExist
	}
	// create the dxdir
	d, err := New(path, ds.rootDir, ds.wal)
	if err != nil {
		return nil, err
	}
	// create the entry and update dxdir
	entry := ds.newDirSetEntry(d)
	tid := randomThreadID()
	entry.threadMap[tid] = newThread()
	ds.dirMap[path] = entry
	return &DirSetEntryWithID{
		dirSetEntry: entry,
		threadID:    tid,
	}, nil
}

// newDirSetEntry create a New dirSetEntry with the DxDir
func (ds *DirSet) newDirSetEntry(d *DxDir) *dirSetEntry {
	threads := make(map[threadID]threadInfo)
	return &dirSetEntry{
		DxDir:     d,
		dirSet:    ds,
		threadMap: threads,
	}
}

// Open opens a New DxDir. If file not exist, return an os file Not Exist error
func (ds *DirSet) Open(path storage.DxPath) (*DirSetEntryWithID, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	return ds.open(path)
}

// open opens the DxDir with path, add the New threadInfo to the entry
func (ds *DirSet) open(path storage.DxPath) (*DirSetEntryWithID, error) {
	var entry *dirSetEntry
	entry, exist := ds.dirMap[path]
	if !exist {
		d, err := load(ds.dirFilePath(path), ds.wal)
		if err != nil {
			return nil, err
		}
		entry = ds.newDirSetEntry(d)
		ds.dirMap[path] = entry
	}
	tid := randomThreadID()
	entry.threadMapLock.Lock()
	entry.threadMap[tid] = newThread()
	entry.threadMapLock.Unlock()
	return &DirSetEntryWithID{
		dirSetEntry: entry,
		threadID:    tid,
	}, nil
}

// Close close the entry. If all threads with the entry is closed, remove the entry from the DirSet
func (entry *DirSetEntryWithID) Close() error {
	entry.dirSet.lock.Lock()
	defer entry.dirSet.lock.Unlock()
	entry.dirSet.closeEntry(entry)
	return nil
}

// closeEntry close the DirSetEntryWithID within the DirSet. If the entry has no more
// threads that holds, remove the entry from the DirSet
func (ds *DirSet) closeEntry(entry *DirSetEntryWithID) {
	// delete the thread id in threadMap
	entry.threadMapLock.Lock()
	defer entry.threadMapLock.Unlock()
	delete(entry.threadMap, entry.threadID)

	// If DxDir is already deleted, simply return
	currentEntry := ds.dirMap[entry.metadata.DxPath]
	if currentEntry != entry.dirSetEntry {
		return
	}
	// If there is no more threads holding the entry, remove the DxDir from the DirSet
	if len(currentEntry.threadMap) == 0 {
		delete(ds.dirMap, entry.metadata.DxPath)
	}
}

// Exists checks whether DxDir with path exists. If file not exist, return
// an os File Not Exist error
func (ds *DirSet) Exists(path storage.DxPath) bool {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	return ds.exists(path)
}

// exists checks whether DxDir with path exist
func (ds *DirSet) exists(path storage.DxPath) bool {
	_, exists := ds.dirMap[path]
	if exists {
		return true
	}
	_, err := os.Stat(string(ds.dirFilePath(path)))
	if err == nil {
		return true
	}
	return false
}

// Delete delete the dxdir. If file not exist, return os.ErrNotExist
func (ds *DirSet) Delete(path storage.DxPath) error {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	// check whether exists
	exists := ds.exists(path)
	if !exists {
		return os.ErrNotExist
	}
	// open the entry
	entry, err := ds.open(path)
	if err != nil {
		return err
	}
	defer ds.closeEntry(entry)
	entry.threadMapLock.Lock()
	defer entry.threadMapLock.Unlock()
	err = entry.Delete()
	if err != nil {
		return err
	}
	delete(ds.dirMap, path)
	return nil
}

// UpdateMetadata update the metadata of the dxdir specified by DxPath
func (ds *DirSet) UpdateMetadata(path storage.DxPath, metadata Metadata) error {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	// Check whether the dxdir exists
	exist := ds.exists(path)
	if !exist {
		return os.ErrNotExist
	}
	// Open the entry, and apply the updates
	entry, err := ds.open(path)
	if err != nil {
		return err
	}
	defer ds.closeEntry(entry)
	return entry.UpdateMetadata(metadata)
}

func (ds *DirSet) dirFilePath(path storage.DxPath) storage.SysPath {
	return ds.rootDir.Join(path, DirFileName)
}

// newThread create the threadInfo by calling runtime.Caller
func newThread() threadInfo {
	ti := threadInfo{
		callingFiles: make([]string, threadDepth+1),
		callingLines: make([]int, threadDepth+1),
		lockTime:     time.Now(),
	}
	for i := 0; i <= threadDepth; i++ {
		_, ti.callingFiles[i], ti.callingLines[i], _ = runtime.Caller(2 + i)
	}
	return ti
}

// randomThreadID create a random number used for threadID
func randomThreadID() threadID {
	b := make([]byte, 8)
	rand.Read(b)
	return threadID(binary.LittleEndian.Uint64(b))
}
