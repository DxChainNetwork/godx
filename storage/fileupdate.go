package storage

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"os"
	"path/filepath"
)

const (
	// OpInsertFile is the operation name for an InsertUpdate
	OpInsertFile = "insert_file"

	// OpDeleteFile is the operation name for an DeleteUpdate
	OpDeleteFile = "delete_file"
)

// FileUpdate defines the update interface for dxfile and dxdir update.
// It is the intermediate layer between dxfile/dxdir persist and wal
// Currently FileUpdate is implemented by InsertUpdate and DeleteUpdate
type FileUpdate interface {
	Apply() error                                    // Apply the content of the update
	EncodeToWalOp() (writeaheadlog.Operation, error) // convert an dxfileUpdate to writeaheadlog.Operation
}

type (
	// InsertUpdate defines an update of insert Data into FileName at Offset
	InsertUpdate struct {
		FileName string
		Offset   uint64
		Data     []byte
	}

	// DeleteUpdate defines an update of delete the FileName
	DeleteUpdate struct {
		FileName string
	}
)

// Apply execute the InsertUpdate, writing data to the location
func (iu *InsertUpdate) Apply() (err error) {
	// Open the file
	err = os.MkdirAll(filepath.Dir(iu.FileName), 0700)
	if err != nil {
		return fmt.Errorf("failed to make directory: %v", err)
	}
	f, err := os.OpenFile(iu.FileName, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to apply InsertUpdate: %v", err)
	}
	defer func() {
		err = common.ErrCompose(err, f.Close())
	}()
	// Write the data
	if n, err := f.WriteAt(iu.Data, int64(iu.Offset)); err != nil {
		return fmt.Errorf("failed to write insertUpdate Data: %v", err)
	} else if n < len(iu.Data) {
		return fmt.Errorf("failed to write full Data of an insertUpdate: %v", err)
	}
	// Flush the data
	return f.Sync()
}

// EncodeToWalOp encode the InsertUpdate to wal.Operation, named by OpInsertFile
func (iu *InsertUpdate) EncodeToWalOp() (writeaheadlog.Operation, error) {
	data, err := rlp.EncodeToBytes(*iu)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: OpInsertFile,
		Data: data,
	}, err
}

// Apply of DeleteUpdate delete du.FileName
func (du *DeleteUpdate) Apply() (err error) {
	err = os.Remove(du.FileName)
	if os.IsNotExist(err) {
		return nil
	}
	return
}

// EncodeToWalOp encode the DeleteUpdate to Operation, named by OpDeleteFile
func (du *DeleteUpdate) EncodeToWalOp() (writeaheadlog.Operation, error) {
	data, err := rlp.EncodeToBytes(*du)
	if err != nil {
		return writeaheadlog.Operation{}, nil
	}
	return writeaheadlog.Operation{
		Name: OpDeleteFile,
		Data: data,
	}, nil
}

// decodeFromWalOp will decode the wal.Operation to a specified type of dxfileUpdate based on the op.Name field
func OpToUpdate(op writeaheadlog.Operation) (FileUpdate, error) {
	switch op.Name {
	case OpInsertFile:
		return decodeOpToInsertUpdate(op)
	case OpDeleteFile:
		return decodeOpToDeleteUpdate(op)
	default:
		return nil, fmt.Errorf("invalid op.Name: %v", op.Name)
	}
}

// decodeOpToDeleteUpdate decode the wal.Operation to a deleteUpdate
func decodeOpToDeleteUpdate(op writeaheadlog.Operation) (*DeleteUpdate, error) {
	var du DeleteUpdate
	err := rlp.DecodeBytes(op.Data, &du)
	if err != nil {
		return nil, err
	}
	return &du, nil
}

// decodeOpToInsertUpdate decode the op to an insertUpdate
func decodeOpToInsertUpdate(op writeaheadlog.Operation) (*InsertUpdate, error) {
	var iu InsertUpdate
	err := rlp.DecodeBytes(op.Data, &iu)
	if err != nil {
		return nil, err
	}
	return &iu, nil
}

// updatesToOps encode the updates to Operations
func updatesToOps(updates []FileUpdate) ([]writeaheadlog.Operation, error) {
	var fullErr error
	ops := make([]writeaheadlog.Operation, len(updates))
	for i, update := range updates {
		op, err := update.EncodeToWalOp()
		if err != nil {
			fullErr = common.ErrCompose(fullErr, err)
			continue
		}
		ops[i] = op
	}
	return ops, fullErr
}

// ApplyOperations apply the operations
func ApplyOperations(ops []writeaheadlog.Operation) error {
	var fullErr error
	for i, op := range ops {
		up, err := OpToUpdate(op)
		if err != nil {
			fullErr = common.ErrCompose(fullErr, fmt.Errorf("cannot decode op to update at index %d: %v", i, err))
			continue
		}
		err = up.Apply()
		if err != nil {
			fullErr = common.ErrCompose(fullErr, fmt.Errorf("cannot apply operation %d: %v", i, err))
			continue
		}
	}
	return fullErr
}

// ApplyUpdates apply the updates on the wal
func ApplyUpdates(wal *writeaheadlog.Wal, updates []FileUpdate) error {
	// Decode the updates to Operations
	ops, err := updatesToOps(updates)
	if err != nil {
		return fmt.Errorf("failed to encode updates: %v", err)
	}
	// Create the writeaheadlog transaction
	txn, err := wal.NewTransaction(ops)
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
	err = ApplyOperations(ops)
	if err != nil {
		return err
	}
	if err = txn.Release(); err != nil {
		return fmt.Errorf("failed to release transaction: %v", err)
	}
	return nil
}
