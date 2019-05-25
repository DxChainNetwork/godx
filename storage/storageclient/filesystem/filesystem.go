package filesystem

import (
	"fmt"
	"sync"

	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// Disrupt is a map for disrupt during test cases
var Disrupt = make(map[string]bool)

// FileSystem is the structure for a file system that include a fileSet and a dirSet
type FileSystem struct {
	// rootDir is the root directory of the file system
	rootDir storage.SysPath

	// fileSet is the FileSet from module dxfile
	fileSet *dxfile.FileSet

	// dirSet is the DirSet from module dxdir
	dirSet *dxdir.DirSet

	// contractor is the contractor used to give health info for the file system
	contractor contractor

	// fileWal is the wal responsible for storage.InsertUpdate / storage.DeleteUpdate
	// that is used in dxfile and dxdir
	fileWal *writeaheadlog.Wal

	// dirMetadataWal is the wal responsible for
	dirMetadataWal *writeaheadlog.Wal

	// tm is the thread manager for manage the threads in FileSystem
	tm *threadmanager.ThreadManager

	// unfinishedUpdates is the field for the mapping from DxPath to the directory to be
	// updated
	unfinishedUpdates map[storage.DxPath]*dirMetadataUpdate

	// lock is meant to protect the map unfinishedUpdates
	lock sync.Mutex
}

// NewFileSystem creates a new file system
func NewFileSystem(rootDir storage.SysPath, contractor contractor, fileWal *writeaheadlog.Wal, dirMetadataWal *writeaheadlog.Wal) (*FileSystem, error) {
	fileSet := dxfile.NewFileSet(rootDir, fileWal)
	dirSet, err := dxdir.NewDirSet(rootDir, fileWal)
	if err != nil {
		return nil, fmt.Errorf("cannot create dirset: %v", err)
	}
	// create the FileSystem
	return &FileSystem{
		rootDir:        rootDir,
		fileSet:        fileSet,
		dirSet:         dirSet,
		contractor:     contractor,
		fileWal:        fileWal,
		dirMetadataWal: dirMetadataWal,
		tm:             &threadmanager.ThreadManager{},
	}, nil
}

func (fs *FileSystem) Close() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	for _, update := range fs.unfinishedUpdates {
		close(update.stop)
	}
	err := fs.tm.Stop()
	if err != nil {
		return err
	}
}
