package dxdir

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
	"os"
)

const (
	// OpDeleteDir is the operation name for delete a directory
	OpDeleteDir = "delete_dir"

	// OpInsertDir is the operation name for insert a directory
	OpInsertDir = "insert_dir"
)

// EncodeRLP define the RLP rule for DxDir. Only the metadata is RLP encoded.
func (d *DxDir) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, *d.metadata)
}

// DecodeRLP define the RLP decode rule for DxDir. Only metadata is decoded.
func (d *DxDir) DecodeRLP(st rlp.Stream) error {
	var m Metadata
	if err := st.Decode(&m); err != nil {
		return err
	}
	d.metadata = &m
	return nil
}

// dxdirUpdate defines the update interface for dxdir update.
// Two types of update is supported: insertUpdate and deleteUpdate
type dxdirUpdate interface {
	apply() error                                    // Apply the content of the update
	encodeToWalOp() (writeaheadlog.Operation, error) // convert an dxfileUpdate to writeaheadlog.Operation
	dirFile() string                                // filePath returns the filePath associated with the update
}

type (
	// insertUpdate is the update to update the
	insertUpdate struct {
		DirFile string
		Data    []byte
	}

	deleteUpdate struct{
		DirFile string
	}
)

// encodeToWalOp encode the insertUpdate to wal.Operation
func (iu *insertUpdate) encodeToWalOp() (writeaheadlog.Operation, error) {
	data, err := rlp.EncodeToBytes(*iu)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation {
		Name: OpInsertDir,
		Data: data,
	}, nil
}

// apply defines how the insert update update the metadata info
func (iu *insertUpdate) apply() error {
	// open file
	f, err := os.OpenFile(iu.DirFile, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to apply insertUpdate: %v", err)
	}
	defer f.Close()
	//write data
	if _, err := f.Write(iu.Data); err != nil {
		return fmt.Errorf("failed to write data: %v", err)
	}
	return f.Sync()
}

// dirFile return the directory effected by the insert update
func (iu *insertUpdate) dirFile() string {
	return iu.DirFile
}

// encodeToWalOp will encode a deleteUpdate to writeaheadlog.Operation
func (du *deleteUpdate) encodeToWalOp() (writeaheadlog.Operation, error) {
	data, err := rlp.EncodeToBytes(*du)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: OpDeleteDir,
		Data: data,
	}, nil
}

// apply remove the file specified by deleteUpdate.filePath
func (du *deleteUpdate) apply() error {
	err := os.Remove(du.DirFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// dirName return the directory name of the delete update
func (du *deleteUpdate) dirName() string {
	return du.DirFile
}

// decodeFromWalOp will decode the wal.Operation to a specified type of dxfileUpdate based on the op.Name field
func decodeFromWalOp(op writeaheadlog.Operation) (dxdirUpdate, error) {
	switch op.Name {
	case OpInsertDir:
		return decodeOpToInsertUpdate(op)
	case OpDeleteDir:
		return decodeOpToDeleteUpdate(op)
	default:
		return nil, fmt.Errorf("invalid op.Name: %v", op.Name)
	}
}

// decodeOpToDeleteUpdate decode the wal.Operation to a deleteUpdate
func decodeOpToDeleteUpdate(op writeaheadlog.Operation) (*deleteUpdate, error) {
	var du deleteUpdate
	err := rlp.DecodeBytes(op.Data, &du)
	if err != nil {
		return nil, err
	}
	return &du, nil
}

// decodeOpToInsertUpdate decode the op to an insertUpdate
func decodeOpToInsertUpdate(op writeaheadlog.Operation) (*insertUpdate, error) {
	var iu insertUpdate
	err := rlp.DecodeBytes(op.Data, &iu)
	if err != nil {
		return nil, err
	}
	return &iu, nil
}

// createInsertUpdate create a DxFile insertion update.
func (d *DxDir) createInsertUpdate() (*insertUpdate, error) {
	data, err := rlp.EncodeToBytes(d)
	if err != nil {
		return nil, err
	}
	return &insertUpdate{
		DirFile:  d.dirPath,
		Data:     data,
	}, nil
}

// createDeleteUpdate create a DxFile deletion update
func (d *DxDir) createDeleteUpdate() (*deleteUpdate, error) {
	return &deleteUpdate{
		DirFile: d.dirPath,
	}, nil
}

// applyUpdates use Wal to create a transaction to apply the updates
func (d *DxDir) applyUpdates(updates []dxdirUpdate) error {
	if d.deleted {
		return errors.New("cannot apply updates on deleted file")
	}
	// first filter the unnecessary updates
	ops, err := updatesToOps(updates)
	if err != nil {
		return fmt.Errorf("cannot apply updates: %v", err)
	}
	// Create the writeaheadlog transaction and apply till release
	txn, err := d.wal.NewTransaction(ops)
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
	for _, u := range updates {
		err := u.apply()
		if err != nil {
			return fmt.Errorf("failed to apply update: %v", err)
		}
	}
	if err = txn.Release(); err != nil {
		return fmt.Errorf("failed to release transaction: %v", err)
	}
	return nil
}

func updatesToOps(updates []dxdirUpdate) ([]writeaheadlog.Operation, error) {

}
