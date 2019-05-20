// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
)

// TODO: Refactor the apply update to write page updates. Each page has next Offset
// 		 Each read will stop at next Offset == -1. Call it paged file.

// createInsertUpdate create a DxFile insertion update.
func (df *DxFile) createInsertUpdate(offset uint64, data []byte) (*storage.InsertUpdate, error) {
	// check validity of the Offset variable
	if offset < 0 {
		return nil, fmt.Errorf("file Offset cannot be negative: %d", offset)
	}
	return &storage.InsertUpdate{
		FileName: df.filePath,
		Offset:   offset,
		Data:     data,
	}, nil
}

// createDeleteUpdate create a DxFile deletion update
func (df *DxFile) createDeleteUpdate() (*storage.DeleteUpdate, error) {
	return &storage.DeleteUpdate{
		FileName: df.filePath,
	}, nil
}
