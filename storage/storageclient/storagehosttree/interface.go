// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// StorageHostTree is the interface for storageHostTree which stores the host info in a tree
// structure
type StorageHostTree interface {
	Insert(hi storage.HostInfo, eval int64) error
	HostInfoUpdate(hi storage.HostInfo, eval int64) error
	Remove(enodeID enode.ID) error
	All() []storage.HostInfo
	RetrieveHostInfo(enodeID enode.ID) (storage.HostInfo, bool)
	RetrieveHostEval(enodeID enode.ID) (int64, bool)
	SelectRandom(needed int, blacklist, addrBlacklist []enode.ID) []storage.HostInfo
}
