// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

const (
	// database related keys and prefixes
	prefixFolder         = "storagefolder"
	prefixFolderPathToID = "folderPathToID"

	sectorSaltKey = "sectorSalt"
)

const (
	databaseFileName = "storagemanager.db"
	walFileName      = "storagemanager.wal"
	dataFileName     = "dxstorage.data"
)

const (
	// target is the process target
	// targetNormal is the normal execution of an update
	targetNormal uint8 = iota

	// targetRecoverCommitted is the state of recovering a committed transaction
	targetRecoverCommitted

	// targetRecoverUncommitted is the state of recovering an uncommitted trnasaction
	targetRecoverUncommitted
)

const (
	folderAvailable uint32 = iota
	folderUnavailable
)
