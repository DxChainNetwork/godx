// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"errors"
	"fmt"
	"os"

	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	opDeleteFile = "delete_file" // name of the operation to delete a dxfile
	opInsertFile = "insert_file" // name of the operation to create or update a dxfile
)

// dxfileUpdate defines the update interface for dxfile update.
// Two types of update is supported: insertUpdate and deleteUpdate
type dxfileUpdate interface {
	apply() error                                    // Apply the content of the update
	encodeToWalOp() (writeaheadlog.Operation, error) // convert an dxfileUpdate to writeaheadlog.Operation
	fileName() string                                // filename returns the filename associated with the update
}

// insertUpdate defines an insert dxfile instruction, including filenamefwqxzA, offset, and data to write
type (
	insertUpdate struct {
		filename string
		offset   int64
		data     []byte
	}

	// deleteUpdate defines a delete dxfile instruction, just the filename to be deleted
	deleteUpdate struct {
		filename string
	}
)

// EncodeToWalOp convert an insertUpdate to wal.Operation
func (iu *insertUpdate) encodeToWalOp() (writeaheadlog.Operation, error) {
	data, err := rlp.EncodeToBytes(*iu)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: opInsertFile,
		Data: data,
	}, err
}

// apply will open or create the file defined by iu.filename, and write iu.data at iu.offset
func (iu *insertUpdate) apply() error {
	// open the file
	f, err := os.OpenFile(iu.filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to apply insertUpdate: %v", err)
	}
	// write data
	if n, err := f.WriteAt(iu.data, iu.offset); err != nil {
		return fmt.Errorf("failed to write insertUpdate data: %v", err)
	} else if n < len(iu.data) {
		return fmt.Errorf("failed to write full data of an insertUpdate: %v", err)
	}
	return f.Sync()
}

func (iu *insertUpdate) fileName() string {
	return iu.filename
}

// encodeToWalOp will encode a deleteUpdate to writeaheadlog.Operation
func (du *deleteUpdate) encodeToWalOp() (writeaheadlog.Operation, error) {
	data, err := rlp.EncodeToBytes(*du)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: opDeleteFile,
		Data: data,
	}, nil
}

// apply remove the file specified by deleteUpdate.filename
func (du *deleteUpdate) apply() error {
	err := os.Remove(du.filename)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (du *deleteUpdate) fileName() string {
	return du.filename
}

// decodeFromWalOp will decode the wal.Operation to a specified type of dxfileUpdate based on the op.Name field
func decodeFromWalOp(op writeaheadlog.Operation) (dxfileUpdate, error) {
	switch op.Name {
	case opInsertFile:
		return decodeOpToInsertUpdate(op)
	case opDeleteFile:
		return decodeOpToDeleteUpdate(op)
	default:
		return nil, fmt.Errorf("invalid op.Name: %v", op.Name)
	}
}

func decodeOpToDeleteUpdate(op writeaheadlog.Operation) (du *deleteUpdate, err error) {
	err = rlp.DecodeBytes(op.Data, du)
	if err != nil {
		return nil, err
	}
	return
}

// decodeOpToInsertUpdate decode the op to insert update
func decodeOpToInsertUpdate(op writeaheadlog.Operation) (iu *insertUpdate, err error) {
	err = rlp.DecodeBytes(op.Data, iu)
	if err != nil {
		return nil, err
	}
	return
}

func (df *DxFile) createInsertUpdate(offset int64, data []byte) (*insertUpdate, error) {
	// check validity of the offset variable
	if offset < 0 {
		return nil, fmt.Errorf("file offset cannot be negative: %d", offset)
	}
	return &insertUpdate{
		filename: df.filename,
		offset:   offset,
		data:     data,
	}, nil
}

func (df *DxFile) createDeleteUpdate() (*deleteUpdate, error) {
	return &deleteUpdate{
		filename: df.filename,
	}, nil
}

// applyUpdates use Wal to create a transaction to apply the updates
func (df *DxFile) applyUpdates(updates []dxfileUpdate) error {
	if df.deleted {
		return errors.New("cannot apply updates on deleted file")
	}
	// first filter the unnecessary updates
	updates = df.filterUpdates(updates)
	ops, err := updatesToOps(updates)
	if err != nil {
		return fmt.Errorf("cannot apply updates: %v", err)
	}
	// Create the writeaheadlog transaction and apply till release
	txn, err := df.wal.NewTransaction(ops)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %v", err)
	}
	<-txn.InitComplete
	if txn.InitErr != nil {
		return fmt.Errorf("failed to create transaction: %v", err)
	}
	if err = <-txn.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	if err = txn.Release(); err != nil {
		return fmt.Errorf("failed to release transaction: %v", err)
	}
	return nil
}

// filterUpdates filter the updates associated with the DxFile, and also filter all updates before a delete update
func (df *DxFile) filterUpdates(updates []dxfileUpdate) []dxfileUpdate {
	filtered := make([]dxfileUpdate, len(updates))
	for _, update := range updates {
		if update.fileName() == df.filename {
			if _, isDeleteUpdate := update.(*deleteUpdate); isDeleteUpdate {
				// If the update is deletion, remove all previous updates
				filtered = filtered[:0]
			}
			filtered = append(filtered, update)
		}
	}
	return filtered
}

// updatesToOps convert []dxfileUpdate to []wal.Operation
func updatesToOps(updates []dxfileUpdate) ([]writeaheadlog.Operation, error) {
	errs := make([]error, len(updates))
	ops := make([]writeaheadlog.Operation, len(updates))
	for _, update := range updates {
		op, err := update.encodeToWalOp()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ops = append(ops, op)
	}
	return ops, composeError(errs...)
}
