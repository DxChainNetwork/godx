// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import "time"

const (
	redoNotNeeded uint32 = iota
	redoNeeded
)

const (
	dirMetadataUpdateName = "dirMetadataUpdate"

	// numConsecutiveFailRelease defines the time when fail reaches this number,
	// dirMetadataUpdate is release and deleted from map
	numConsecutiveFailRelease = 3
)

const (
	// repairUnfinishedLoopInterval is the interval between two repairUnfinishedDirMetadataUpdate
	repairUnfinishedLoopInterval = time.Second

	// filesDirectory is the directory to put all files
	filesDirectory = "files"

	// fileWalName is the fileName for the fileWal
	fileWalName = "file.wal"

	// updateWalName is the fileName for the updateWal
	updateWalName = "update.wal"
)
