package filesystem

import (
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

type FileSystem struct {
	fileSet *dxfile.FileSet
	dirSet  *dxdir.DirSet

	// fileWal is the wal responsible for storage.InsertUpdate / storage.DeleteUpdate
	// that is used in dxfile and dxdir
	fileWal *writeaheadlog.Wal

	dirMetdataWal
}
