// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

type (
	// dirTree is the structure for test to record the file structure of client's files
	dirTree struct {
		root *dirTreeNode

		// random params when generating a random file
		// goDeepRate is the possibility of when creating a file, it goes deep into
		// a subdirectory of the current directory.
		// goWideRate is the possibility of when going deep, instead of using an existing
		// directory, it creates a new one
		goDeepRate float32
		goWideRate float32

		// maxDepth is the max depth of a file
		maxDepth int
	}

	// dirTreeNode is the node of dirTree.
	dirTreeNode struct {
		subDirs map[string]*dirTreeNode
		files   map[string]struct{}
		dxPath  storage.DxPath
	}
)

// createRandomFiles create random files of numFiles. The file structure is defined randomly by
// goDeepRate, goWideRate, and maxDepth. More info about the params please read comment at dirTree
// missRate is the rate that a sector data is missing. Aimed for test the API of uploaded files
func (fs *fileSystem) createRandomFiles(numFiles int, goDeepRate, goWideRate float32, maxDepth int, missRate float32) error {
	dt := newDirTree(goDeepRate, goWideRate, maxDepth)
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(numFiles)
	errChan := make(chan error)
	for i := 0; i < numFiles; i++ {
		path, err := dt.randomPath()
		if err != nil {
			return err
		}
		// The default file size here is 11 segments
		go func() {
			defer wg.Done()
			fileSize := uint64(1 << 22 * 10 * 10)
			dxfile, err := fs.fileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, missRate)
			if err != nil {
				errChan <- err
				return
			}
			err = dxfile.Close()
			parentPath, err := path.Parent()
			err = fs.InitAndUpdateDirMetadata(parentPath)
			if err != nil {
				errChan <- err
				return
			}
			return
		}()
	}
	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()
	select {
	case err := <-errChan:
		return err
	case <-time.After(1 * time.Second * time.Duration(maxDepth) * time.Duration(numFiles)):
		return fmt.Errorf("createRandomFiles time out")
	case <-wait:
	}
	// wait for all the updates to finish
	if err = fs.waitForUpdatesComplete(1 * time.Second * time.Duration(maxDepth) * time.Duration(numFiles)); err != nil {
		return err
	}
	return nil
}

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) storage.SysPath {
	path := filepath.Join(os.TempDir(), "filesystem", filepath.Join(dirs...))
	if err := os.RemoveAll(path); err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v: %v", path, err))
	}
	if err := os.MkdirAll(path, 0777); err != nil {
		panic(fmt.Sprintf("cannot create directory %v: %v", path, err))
	}
	return storage.SysPath(path)
}

// newEmptyTestFileSystem creates an empty file system used for testing
func newEmptyTestFileSystem(t *testing.T, extraNaming string, contractor contractManager, disrupter disrupter) *fileSystem {
	var rootDir storage.SysPath
	if len(extraNaming) == 0 {
		rootDir = tempDir(t.Name())
	} else {
		rootDir = tempDir(t.Name(), extraNaming)
	}
	fs := newFileSystem(string(rootDir), contractor, disrupter)
	err := fs.Start()
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

//newDirTree creates a new dirTree with the params provides
func newDirTree(goDeepRate, goWideRate float32, maxDepth int) dirTree {
	rand.Seed(time.Now().UnixNano())
	return dirTree{
		&dirTreeNode{
			subDirs: make(map[string]*dirTreeNode),
			files:   make(map[string]struct{}),
			dxPath:  storage.RootDxPath(),
		},
		goDeepRate, goWideRate, maxDepth,
	}
}

// randomPath creates a random path under the based on dt settings
func (dt dirTree) randomPath() (storage.DxPath, error) {
	curDir := dt.root
	for i := 0; i != dt.maxDepth+1; i++ {
		num := rand.Float32()
		// Do not go deeper. create a new file
		if num > dt.goDeepRate || i == dt.maxDepth {
			var fileName string
			for {
				fileName = randomName()
				if _, exist := curDir.subDirs[fileName]; exist {
					continue
				}
				if _, exist := curDir.files[fileName]; exist {
					continue
				}
				break
			}
			curDir.files[fileName] = struct{}{}
			return curDir.dxPath.Join(fileName)
		}
		// go deeper. toll the dice again
		num = rand.Float32()
		if num < dt.goWideRate || len(curDir.subDirs) == 0 {
			// create a new directory
			var dirName string
			for {
				dirName = randomName()
				if _, exist := curDir.subDirs[dirName]; exist {
					continue
				}
				if _, exist := curDir.files[dirName]; exist {
					continue
				}
				break
			}
			subDirPath, err := curDir.dxPath.Join(dirName)
			if err != nil {
				return storage.DxPath{}, err
			}
			curDir.subDirs[dirName] = &dirTreeNode{
				subDirs: make(map[string]*dirTreeNode),
				files:   make(map[string]struct{}),
				dxPath:  subDirPath,
			}
			curDir = curDir.subDirs[dirName]
			continue
		}
		// Go deeper, and randomly use a current existing directory
		for _, curDir = range curDir.subDirs {
			break
		}
	}
	return storage.DxPath{}, fmt.Errorf("this should be never reached")
}

// waitForUpdatesComplete is the helper function that wait for update execution
func (fs *fileSystem) waitForUpdatesComplete(timeout time.Duration) error {
	c := make(chan struct{})
	// Wait until update complete
	go func() {
		defer close(c)
		for {
			<-time.After(50 * time.Millisecond)
			fs.lock.Lock()
			emptyUpdate := len(fs.unfinishedUpdates) == 0
			fs.lock.Unlock()
			if emptyUpdate {
				// There might be case the child directory completed update while
				// the parent update is not in unfinishedUpdates
				<-time.After(50 * time.Millisecond)
				fs.lock.Lock()
				emptyUpdate = len(fs.unfinishedUpdates) == 0
				fs.lock.Unlock()
				if emptyUpdate {
					return
				}
				continue
			}
		}
	}()
	select {
	case <-time.After(timeout):
		return fmt.Errorf("after %s, update still not completed", timeout)
	case <-c:
	}
	return nil
}

// randomName create a random name for the dirTree. name is a length 16 hex string
func randomName() string {
	b := make([]byte, 16)
	rand.Read(b)
	return common.Bytes2Hex(b)
}

// userHomeDir returns the home directory of user
func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	} else if runtime.GOOS == "linux" {
		home := os.Getenv("XDG_CONFIG_HOME")
		if home != "" {
			return home
		}
	}
	return os.Getenv("HOME")
}
