// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// TODO(mzhang): implement this.
// contractor is the contractor interface used in file system
type contractor interface {
	// HostHealthMapByID return storage.HostHealthInfoTable for hosts specified by input
	HostHealthMapByID([]enode.ID) storage.HostHealthInfoTable
}
