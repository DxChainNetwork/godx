package dxfile

import (
	"fmt"
	"github.com/DxChainNetwork/godx/rlp"
)

func (df *DxFile) createMetadataUpdate() (*insertUpdate, error) {
	metaBytes, err := rlp.EncodeToBytes(df.metaData)
	if err != nil {
		return nil, err
	}
	if len(metaBytes) > PageSize {
		// This shall never happen
		return nil, fmt.Errorf("metadata should not have length larger than %v", PageSize)
	}
	return df.createInsertUpdate(0, metaBytes)
}

//func (df *DxFile) createHostTableUpdate() (*insertUpdate, error) {
//
//}

// createSegmentShiftUpdate shift the first segment
//func (df *DxFile) createSegmentShiftUpdate() (*insertUpdate, error) {
//
//}

// TODO: implement this
func (df *DxFile) save() error {
	return nil
}
