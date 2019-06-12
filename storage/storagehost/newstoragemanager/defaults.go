// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

const (
	// database related keys and prefixes
	prefixFolder         = "storageFolder"
	prefixFolderSector   = "folderToSector"
	prefixFolderIDToPath = "folderIDToPath"
	sectorSaltKey        = "sectorSalt"
	prefixSector         = "sector"
)

const (
	opNameAddStorageFolder  = "add storage folder"
	opNameAddSector         = "add sector"
	opNameAddVirtualSector  = "add virtual sector"
	opNameAddPhysicalSector = "add physical sector"
)

const (
	databaseFileName = "storagemanager.db"
	walFileName      = "storagemanager.wal"
	dataFileName     = "dxstorage.dat"
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

const (
	// maxSectorsPerFolder defines the maximum number of sectors in a folder
	maxSectorsPerFolder uint64 = 1 << 32

	// minSectorsPerFolder defines the minimum number of sectors in a folder
	minSectorsPerFolder uint64 = 1 << 3

	// maxNumFolders defines the maximum number of storage folders
	maxNumFolders = 1 << 16
)

const (
	// bitVectorGranularity is the granularity of one bitVector.
	// Since bitVector is of type uint64, and each bit represents a single sector,
	// so the granularity is 64 per bitVector
	bitVectorGranularity = 64
)

const (
	// maxCreateFolderIDRetries defines maximum times of trying to create a folder id
	maxCreateFolderIDRetries = 20

	// maxFolderSelectionRetries is the max retry numbers used for selecting a folder to put a
	// sector
	maxFolderSelectionRetries = 3
)
