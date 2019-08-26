// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/storage"
)

// Evaluator defines an interface used to evaluate the host info
type Evaluator interface {
	Evaluate(info storage.HostInfo) int64
}
