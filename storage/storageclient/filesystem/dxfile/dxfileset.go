package dxfile

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const threadDepth = 3

var (
	ErrUnknownFile = errors.New("file not known")
	ErrFileExist   = errors.New("file already exist")
)

type (
	// DxFileSet is the set of DxFile
	FileSet struct {
		// dxFileDir is the file directory for dxfile
		filesDir string

		// dxFileMap is the mapping from dxPath to contents
		filesMap map[string]*fileSetEntry

		lock sync.Mutex
		wal  *writeaheadlog.Wal
	}

	// fileSetEntry is an entry for fileSet
	fileSetEntry struct {
		*DxFile
		fileSet *FileSet

		threadMap     map[uint64]threadInfo
		threadMapLock sync.Mutex
	}

	// FileSetEntry is a fileSetEntry with the threadID
	FileSetEntryWithId struct {
		*fileSetEntry
		threadID uint64
	}

	// threadInfo is the entry in threadMap
	threadInfo struct {
		callingFiles []string
		callingLines []int
		lockTime     time.Time
	}
)

// NewDxFileSet create a new DxFileSet
func NewDxFileSet(filesDir string, wal *writeaheadlog.Wal) *FileSet {
	return &FileSet{
		filesDir: filesDir,
		filesMap: make(map[string]*fileSetEntry),
		wal:      wal,
	}
}

// CopyEntry copy the FileSetEntry
func (entry *FileSetEntryWithId) CopyEntry() *FileSetEntryWithId {
	entry.threadMapLock.Lock()
	defer entry.threadMapLock.Unlock()

	threadUID := randomThreadID()
	copied := &FileSetEntryWithId{
		fileSetEntry: entry.fileSetEntry,
		threadID:     threadUID,
	}
	entry.threadMap[threadUID] = newThreadInfo()
	return copied
}

// Open open a DxFile with dxPath, return the FileSetEntry, along with the threadID
func (fs *FileSet) Open(dxPath string) (*FileSetEntryWithId, error) {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	return fs.open(dxPath)
}

func (fs *FileSet) open(dxPath string) (*FileSetEntryWithId, error) {
	entry, exist := fs.filesMap[dxPath]
	if !exist {
		// file not loaded or not exist
		df, err := readDxFile(filepath.Join(fs.filesDir, dxPath), fs.wal)
		if os.IsNotExist(err) {
			return nil, ErrUnknownFile
		}
		if err != nil {
			return nil, err
		}
		entry = fs.newFileSetEntry(df)
		fs.filesMap[dxPath] = entry
	}
	if entry.Deleted() {
		return nil, ErrUnknownFile
	}
	threadID := randomThreadID()
	entry.threadMapLock.Lock()
	defer entry.threadMapLock.Unlock()
	entry.threadMap[threadID] = newThreadInfo()
	return &FileSetEntryWithId{
		fileSetEntry: entry,
		threadID:     threadID,
	}, nil
}

// Delete delete a file with dxPath from the file set
func (fs *FileSet) Delete(dxPath string) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	entry, err := fs.open(dxPath)
	if err != nil {
		return err
	}
	defer fs.closeEntry(entry)
	err = entry.Delete()
	if err != nil {
		return err
	}

	delete(fs.filesMap, entry.metadata.DxPath)
	return nil
}

// Exists return whether the dxPath exists (cached or on disk)
func (fs *FileSet) Exists(dxPath string) bool {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	return fs.exists(dxPath)
}

func (fs *FileSet) exists(dxPath string) bool {
	_, exists := fs.filesMap[dxPath]
	if exists {
		return exists
	}
	_, err := os.Stat(filepath.Join(fs.filesDir, dxPath))
	return !os.IsNotExist(err)
}

// Rename rename the file with dxPath to newDxPath.
func (fs *FileSet) Rename(dxPath, newDxPath string) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	exists := fs.exists(newDxPath)
	if exists {
		return ErrFileExist
	}
	entry, err := fs.open(dxPath)
	if err != nil {
		return err
	}
	defer fs.closeEntry(entry)

	fs.filesMap[newDxPath] = entry.fileSetEntry
	delete(fs.filesMap, dxPath)

	return entry.Rename(newDxPath, filepath.Join(fs.filesDir, newDxPath))
}

// Close close a FileSetEntryWithId
func (entry *FileSetEntryWithId) Close() error {
	entry.fileSet.lock.Lock()
	entry.fileSet.closeEntry(entry)
	entry.fileSet.lock.Unlock()
	return nil
}

// newFileSetEntry is a helper function to create a fileSetEntry based on input df
func (fs *FileSet) newFileSetEntry(df *DxFile) *fileSetEntry {
	return &fileSetEntry{
		DxFile:    df,
		fileSet:   fs,
		threadMap: make(map[uint64]threadInfo),
	}
}

// closeEntry close the entry with id in fileSet
func (fs *FileSet) closeEntry(entry *FileSetEntryWithId) {
	entry.threadMapLock.Lock()
	defer entry.threadMapLock.Unlock()
	delete(entry.threadMap, entry.threadID)

	currentEntry := fs.filesMap[entry.metadata.DxPath]
	if currentEntry != entry.fileSetEntry {
		return
	}
	if len(currentEntry.threadMap) == 0 {
		delete(fs.filesMap, entry.metadata.DxPath)
	}
}

// newThreadinfo created a threadInfo entry for the threadMap
func newThreadInfo() threadInfo {
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

// randomThreadID create a random thread id
func randomThreadID() uint64 {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic("cannot create random thread id")
	}
	return binary.LittleEndian.Uint64(b)
}
