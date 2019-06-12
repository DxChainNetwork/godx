// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"time"
)

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

const (
	// constants used in create random files
	defaultGoDeepRate = float32(0.7)
	defaultGoWideRate = float32(0.5)
	defaultMaxDepth   = 3
	defaultMissRate   = float32(0.1)
)

const (
	// A file healthy status is presented by four human readable string
	statusHealthyStr       = "healthy"
	statusRecoverableStr   = "recoverable"
	statusInDangerStr      = "in danger"
	statusUnrecoverableStr = "unrecoverable"

	// Thresholds defines the threshold between status
	healthyThreshold     = dxfile.RepairHealthThreshold
	recoverableThreshold = uint32(125)
	inDangerThreshold    = uint32(100)
)

const (
	// healthCheckInterval is the interval between two health checks
	healthCheckInterval = 30 * time.Minute
)
