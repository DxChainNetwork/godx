// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import "fmt"

// PublicFileSystemAPI is the APIs for the file system
type PublicFileSystemAPI struct {
	fs *FileSystem
}

// NewPublicFileSystemDebugAPI is the function to create the PublicFileSystem API
// with the input file system
func NewPublicFileSystemDebugAPI(fs *FileSystem) *PublicFileSystemAPI {
	return &PublicFileSystemAPI{fs}
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
func (api *PublicFileSystemAPI) CreateRandomFiles(numFiles int, goDeepRate, goWideRate float32, maxDepth int, missRate float32) string {
	err := api.fs.createRandomFiles(numFiles, goDeepRate, goWideRate, maxDepth, missRate)
	if err != nil {
		return fmt.Sprintf("Cannot create random files: %v", err)
	}
	return fmt.Sprintf("Successfully created %d files.", numFiles)
}
