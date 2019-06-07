// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import "github.com/DxChainNetwork/godx/common/writeaheadlog"

// update is the data structure used for all storage manager operations
type update interface {
	str() string
	apply(manager *storageManager) error
	reverse(manager *storageManager) error
	encodeToTransaction() (*writeaheadlog.Transaction, error)
	transaction() *writeaheadlog.Transaction
}

// decodeFromTransaction decode and create an update from the transaction
func decodeFromTransaction(*writeaheadlog.Transaction) (up update, err error) {
	return
}
