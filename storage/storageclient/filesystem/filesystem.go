package filesystem

import (
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

type FileSystem struct {
	fileSet dxfile.FileSet
	dirSet  dxdir.DirSet
}
