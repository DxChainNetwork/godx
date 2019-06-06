// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxdir

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"io"
	"os"
)

var (
	// ErrAlreadyDeleted is the error that happens when save or delete a DxDir
	// that is already deleted
	ErrAlreadyDeleted = errors.New("DxDir has already been deleted")

	// ErrUploadDirectory indicates that we can't upload directory
	ErrUploadDirectory = errors.New("cannot upload directory")

	// ErrPathOverload is an error when a file already exists at that location
	ErrPathOverload = errors.New("a file already exists at that location")

	// ErrUnknownPath is an error when a file cannot be found with the given path
	ErrUnknownPath = errors.New("no file known with that path")
)

// EncodeRLP define the RLP rule for DxDir. Only the metadata is RLP encoded.
func (d *DxDir) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, *d.metadata)
}

// DecodeRLP define the RLP decode rule for DxDir. Only metadata is decoded.
func (d *DxDir) DecodeRLP(st *rlp.Stream) error {
	var m Metadata
	if err := st.Decode(&m); err != nil {
		return err
	}
	d.metadata = &m
	return nil
}

// createInsertUpdate create the insert update of the rlp data of dxdir
func (d *DxDir) createInsertUpdate() (storage.FileUpdate, error) {
	data, err := rlp.EncodeToBytes(d)
	if err != nil {
		return nil, fmt.Errorf("cannot encode DxDir: %v", err)
	}
	iu := &storage.InsertUpdate{
		FileName: string(d.dirFilePath),
		Offset:   0,
		Data:     data,
	}
	return iu, nil
}

// createDeleteUpdate create the delete update
func (d *DxDir) createDeleteUpdate() (storage.FileUpdate, error) {
	du := &storage.DeleteUpdate{
		FileName: string(d.dirFilePath),
	}
	return du, nil
}

// save save the current DxDir to disk
func (d *DxDir) save() error {
	if d.deleted {
		return ErrAlreadyDeleted
	}
	fu, err := d.createInsertUpdate()
	if err != nil {
		return err
	}
	return storage.ApplyUpdates(d.wal, []storage.FileUpdate{fu})
}

// delete create and apply the delete update
func (d *DxDir) delete() error {
	if d.deleted {
		return ErrAlreadyDeleted
	}
	d.deleted = true
	fu, err := d.createDeleteUpdate()
	if err != nil {
		return err
	}
	return storage.ApplyUpdates(d.wal, []storage.FileUpdate{fu})
}

// load load the DxDir metadata.
// input path should be the path of the DxDir file
func load(dirFilePath storage.SysPath, wal *writeaheadlog.Wal) (*DxDir, error) {
	// Open the file
	f, err := os.OpenFile(string(dirFilePath), os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Decode the encoded string
	var d *DxDir
	err = rlp.Decode(f, &d)
	if err != nil {
		return nil, fmt.Errorf("cannot load DxDir: %v", err)
	}
	d.wal = wal
	d.dirFilePath = dirFilePath
	return d, nil
}
