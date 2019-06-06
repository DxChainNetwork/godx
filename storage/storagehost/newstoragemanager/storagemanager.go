// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/log"
)

type storageManager struct {
	// sectorSalt is the salt used to generate the sector id with merkle root
	sectorSalt sectorSalt

	// database is the db that wraps leveldb. Folders and Sectors metadata info are
	// stored in database
	db *database

	// sectorLocks is the map from sector id to the sectorLock
	sectorLocks sectorLocks

	// utility field
	log        log.Logger
	persistDir string
	wal        *writeaheadlog.Wal
	tm         *threadmanager.ThreadManager
}

type sectorSalt [32]byte
