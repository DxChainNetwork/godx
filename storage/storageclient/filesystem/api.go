// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
)

const (
	defaultGoDeepRate = float32(0.7)
	defaultGoWideRate = float32(0.5)
	defaultMaxDepth   = 3
	defaultMissRate   = float32(0.1)
)

// PublicFileSystemDebugAPI is the APIs for the file system
type PublicFileSystemDebugAPI struct {
	fs *FileSystem
}

// NewPublicFileSystemDebugAPI is the function to create the PublicFileSystem API
// with the input file system
func NewPublicFileSystemDebugAPI(fs *FileSystem) *PublicFileSystemDebugAPI {
	return &PublicFileSystemDebugAPI{fs}
}

// CreateRandomFiles create some random files. This API is only used in tests
// The randomed file is defined randomly by goDeepRate, goWideRate, maxDepth, and missRate
// 	goDeepRate is the possibility of when creating a file, it goes deep into
//  	a subdirectory of the current directory.
// 	goWideRate is the possibility of when going deep, instead of using an existing
//  	directory, it creates a new one
//  maxDepth is the maximum directory depth that a file could reach
//  missRate is a number between 0 and 1 that defines the possibility that file's sector
//     	is missing
// Now the params are default to some preset values. These values could be easily changed
func (api *PublicFileSystemDebugAPI) CreateRandomFiles(numFiles int) string {
	goDeepRate, goWideRate, maxDepth, missRate := defaultGoDeepRate, defaultGoWideRate, defaultMaxDepth, defaultMissRate
	err := api.fs.createRandomFiles(numFiles, goDeepRate, goWideRate, maxDepth, missRate)
	if err != nil {
		return fmt.Sprintf("Cannot create random files: %v", err)
	}
	return fmt.Sprintf("Successfully created %d files.", numFiles)
}

// PublicFileSystemAPI is the api for file system
type PublicFileSystemAPI struct {
	fs *FileSystem
}

// NewPublicFileSystemAPI creates a new file system api
func NewPublicFileSystemAPI(fs *FileSystem) *PublicFileSystemAPI {
	return &PublicFileSystemAPI{fs}
}

// RootDir returns the root directory of the file system
func (api *PublicFileSystemAPI) RootDir() string {
	return string(api.fs.rootDir)
}

// PersistDir return the directory where the files locates
func (api *PublicFileSystemAPI) PersistDir() string {
	return string(api.fs.persistDir)
}

// DetailedFileInfo returns the detailed file info of a file specified by the path
func (api *PublicFileSystemAPI) DetailedFileInfo(path string) storage.FileInfo {
	dxpath, err := storage.NewDxPath(path)
	if err != nil {
		// Invalid path
		api.fs.logger.Warn("Cannot get detailed file info", "path", path, "error", err.Error())
		return storage.FileInfo{}
	}
	fileInfo, err := api.fs.fileDetailedInfo(dxpath, make(storage.HostHealthInfoTable))
	return fileInfo
}

// FileList is the API function that returns all uploaded files
func (api *PublicFileSystemAPI) FileList() []storage.FileBriefInfo {
	fileList, err := api.fs.fileList()
	if err != nil {
		api.fs.logger.Warn("cannot get the file list", "error", err.Error())
		return []storage.FileBriefInfo{}
	}
	return fileList
}

// Uploads is the API function that return all files currently uploading in progress
func (api *PublicFileSystemAPI) Uploads() []storage.FileBriefInfo {
	rawFileList, err := api.fs.fileList()
	if err != nil {
		api.fs.logger.Warn("cannot get the file list", "error", err.Error())
		return []storage.FileBriefInfo{}
	}
	var fileList []storage.FileBriefInfo
	for _, file := range rawFileList {
		if file.UploadProgress >= 100 {
			continue
		}
		fileList = append(fileList, file)
	}
	return fileList
}

// Rename is the API function that rename a file from prevPath to newPath
func (api *PublicFileSystemAPI) Rename(prevPath, newPath string) string {
	prevDxPath, err := storage.NewDxPath(prevPath)
	if err != nil {
		return fmt.Sprintf("Path not valid: %v", prevPath)
	}
	newDxPath, err := storage.NewDxPath(newPath)
	if err != nil {
		return fmt.Sprintf("Path not valid: %v", newPath)
	}
	if err = api.fs.FileSet.Rename(prevDxPath, newDxPath); err != nil {
		return fmt.Sprintf("Cannot rename from %v to %v: %v", prevPath, newPath, err)
	}

	if prevParent, err := prevDxPath.Parent(); err == nil {
		// If got error, must be ErrAlreadyRoot. No point to update
		err = api.fs.InitAndUpdateDirMetadata(prevParent)
		if err != nil {
			api.fs.logger.Warn(err.Error())
		}
	}
	if newParent, err := newDxPath.Parent(); err == nil {
		err = api.fs.InitAndUpdateDirMetadata(newParent)
		if err != nil {
			api.fs.logger.Warn(err.Error())
		}
	}
	return fmt.Sprintf("File %v renamed to %v", prevPath, newPath)
}

// Delete delete a file specified by the path
func (api *PublicFileSystemAPI) Delete(path string) string {
	dxPath, err := storage.NewDxPath(path)
	if err != nil {
		return fmt.Sprintf("Path not valid: %v", path)
	}
	if err = api.fs.FileSet.Delete(dxPath); err != nil {
		return fmt.Sprintf("Cannot delete file %v: %v", path, err)
	}
	if newParent, err := dxPath.Parent(); err == nil {
		err = api.fs.InitAndUpdateDirMetadata(newParent)
		if err != nil {
			api.fs.logger.Warn(err.Error())
		}
	}
	return fmt.Sprintf("File %v deleted", path)
}
