package dxdir

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const threadDepth = 3

type (
	// DirSet is the manager of all DxDirs
	DirSet struct {
		rootDir string
		dirMap  map[DxPath]*dirSetEntry

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

// NewDirSet creates a New DirSet with the given parameters
func NewDirSet(rootDir string, wal *writeaheadlog.Wal) *DirSet {
	return &DirSet{
		rootDir: rootDir,
		dirMap:  make(map[DxPath]*dirSetEntry),
		wal:     wal,
	}
}

// Start initialize the root directory on disk.
func (ds *DirSet) Start() error {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	exist, err := ds.exists("")
	if exist {
		return nil
	}
	if os.IsNotExist(err) {
		_, err = New("", dirPath(ds.rootDir), ds.wal)
		return err
	}
	return err
}

// NewDxDir creates a DxDir. Return a DirSetEntryWithID that extends DxDir and the error
func (ds *DirSet) NewDxDir(path DxPath) (*DirSetEntryWithID, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	// Check the directory file already exists
	exist, err := ds.exists(path)
	if exist {
		return nil, os.ErrExist
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// create the dxdir
	d, err := New(path, ds.dirPath(path), ds.wal)
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
func (ds *DirSet) Open(path DxPath) (*DirSetEntryWithID, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	return ds.open(path)
}

// open opens the DxDir with path, add the New threadInfo to the entry
func (ds *DirSet) open(path DxPath) (*DirSetEntryWithID, error) {
	var entry *dirSetEntry
	entry, exist := ds.dirMap[path]
	if !exist {
		d, err := load(ds.dirPath(path), ds.wal)
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
func (ds *DirSet) Exists(path DxPath) (bool, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	return ds.exists(path)
}

// exists checks whether DxDir with path exist
func (ds *DirSet) exists(path DxPath) (bool, error) {
	_, exists := ds.dirMap[path]
	if exists {
		return exists, nil
	}
	_, err := os.Stat(ds.dirFilePath(path))
	if err == nil {
		return true, nil
	}
	return false, err
}

// Delete delete the dxdir. If file not exist, return os.ErrNotExist
func (ds *DirSet) Delete(path DxPath) error {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	// check whether exists
	exists, err := ds.exists(path)
	if !exists && os.IsNotExist(err) {
		return os.ErrNotExist
	}
	if err != nil {
		return err
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
func (ds *DirSet) UpdateMetadata(path DxPath, metadata Metadata) error {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	// Check whether the dxdir exists
	exist, err := ds.exists(path)
	if !exist || os.IsNotExist(err) {
		return os.ErrNotExist
	}
	if err != nil {
		return err
	}
	// Open the entry, and apply the updates
	entry, err := ds.open(path)
	if err != nil {
		return err
	}
	defer ds.closeEntry(entry)
	return entry.UpdateMetadata(metadata)
}

func (ds *DirSet) dirFilePath(path DxPath) string {
	return filepath.Join(string(path), string(path), dirFileName)
}

// dirPath convert the DxPath concatenate with root path to dirPath
func (ds *DirSet) dirPath(path DxPath) dirPath {
	return dirPath(filepath.Join(ds.rootDir, string(path)))
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
