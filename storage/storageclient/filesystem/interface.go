// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// FileSystem is the interface for fileSystem
type FileSystem interface {
	// File System is could be used as a service with New, Start, and Close
	Start() error
	Close() error

	// Properties
	RootDir() storage.SysPath
	PersistDir() storage.SysPath

	// DxFile related methods, including New, Open, Rename and Delete
	NewDxFile(dxPath storage.DxPath, sourcePath storage.SysPath, force bool, erasureCode erasurecode.ErasureCoder, cipherKey crypto.CipherKey, fileSize uint64, fileMode os.FileMode) (*dxfile.FileSetEntryWithID, error)
	OpenDxFile(path storage.DxPath) (*dxfile.FileSetEntryWithID, error)
	RenameDxFile(prevDxPath, curDxPath storage.DxPath) error
	DeleteDxFile(dxPath storage.DxPath) error

	// DxDir related methods, including New and open
	NewDxDir(path storage.DxPath) (*dxdir.DirSetEntryWithID, error)
	OpenDxDir(path storage.DxPath) (*dxdir.DirSetEntryWithID, error)

	// Upload/Download logic related functions
	InitAndUpdateDirMetadata(path storage.DxPath) error
	SelectDxFileToFix() (*dxfile.FileSetEntryWithID, error)
	RandomStuckDirectory() (*dxdir.DirSetEntryWithID, error)
	OldestLastTimeHealthCheck() (storage.DxPath, time.Time, error)
	RepairNeededChan() chan struct{}
	StuckFoundChan() chan struct{}

	// private function fields used for APIs
	getLogger() log.Logger
	fileDetailedInfo(path storage.DxPath, table storage.HostHealthInfoTable) (storage.FileInfo, error)
	fileList() ([]storage.FileBriefInfo, error)
}

// New is the public function used for creating a production fileSystem
func New(persistDir string, contractor contractManager) FileSystem {
	d := newStandardDisrupter()
	return newFileSystem(persistDir, contractor, d)
}

// contractManager is the contractManager interface used in file system
type contractManager interface {
	// HostHealthMapByID return storage.HostHealthInfoTable for hosts specified by input
	HostHealthMapByID([]enode.ID) storage.HostHealthInfoTable

	// HostHealthMap returns the full host info table of the contract manager
	HostHealthMap() (infoTable storage.HostHealthInfoTable)
}

// AlwaysSuccessContractManager is the contractManager that always return good condition for all host keys
type AlwaysSuccessContractManager struct{}

// HostHealthMapByID always return good condition
func (c *AlwaysSuccessContractManager) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		table[id] = storage.HostHealthInfo{
			Offline:      false,
			GoodForRenew: true,
		}
	}
	return table
}

// HostHealthMap is not used by test case
func (c *AlwaysSuccessContractManager) HostHealthMap() storage.HostHealthInfoTable {
	return make(storage.HostHealthInfoTable)
}

// AlwaysSuccessContractManager is the contractManager that always return wrong condition for all host keys
type alwaysFailContractManager struct{}

// HostHealthMapByID always return bad condition
func (c *alwaysFailContractManager) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		table[id] = storage.HostHealthInfo{
			Offline:      true,
			GoodForRenew: false,
		}
	}
	return table
}

// HostHealthMap is not used by test case
func (c *alwaysFailContractManager) HostHealthMap() storage.HostHealthInfoTable {
	return make(storage.HostHealthInfoTable)
}

// randomContractManager is the contractManager that return condition is random possibility
// rate is the possibility between 0 and 1 for specified conditions
type randomContractManager struct {
	missRate         float32 // missRate is the rate that the input id is not in the table
	onlineRate       float32 // onlineRate is the rate the the id is online
	goodForRenewRate float32 // goodForRenewRate is the rate of goodForRenew

	missed map[enode.ID]struct{}       // missed node should be forever missed
	table  storage.HostHealthInfoTable // If previously stored the table, do not random again
	once   sync.Once                   // Only initialize the HostHealthInfoTable once
	lock   sync.Mutex                  // lock is the mutex to protect the table field
}

// HostHealthMapByID gives random host health map provided by ids
func (c *randomContractManager) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	c.once.Do(func() {
		c.table = make(storage.HostHealthInfoTable)
		c.missed = make(map[enode.ID]struct{})
	})
	rand.Seed(time.Now().UnixNano())
	c.lock.Lock()
	defer c.lock.Unlock()
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		// previously missed id will be forever missed
		if _, exist := c.missed[id]; exist {
			continue
		}
		if _, exist := c.table[id]; exist {
			table[id] = c.table[id]
			continue
		}
		num := rand.Float32()
		if num < c.missRate {
			c.missed[id] = struct{}{}
			continue
		}
		num = rand.Float32()
		var offline, goodForRenew bool
		if num >= c.onlineRate {
			offline = true
		}
		num = rand.Float32()
		if num < c.goodForRenewRate {
			goodForRenew = true
		}
		c.table[id] = storage.HostHealthInfo{
			Offline:      offline,
			GoodForRenew: goodForRenew,
		}
		table[id] = c.table[id]
	}
	return table
}

// HostHealthMap is not used in tests thus not implemented
func (c *randomContractManager) HostHealthMap() storage.HostHealthInfoTable {
	return make(storage.HostHealthInfoTable)
}
