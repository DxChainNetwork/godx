// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
)

var (
	// ErrNotFound is the error that happens when a sector data is not found
	ErrNotFound = errors.New("not found")

	// errStopped is the error that during update, an error happened
	errStopped = errors.New("storage manager has been stopped")

	// errInvalidTransactionType is the error that an invalid transaction type found
	errInvalidTransactionType = errors.New("invalid wal transaction type")

	// errRevert is the error type to signal the update has to be reverted
	errRevert = errors.New("update to be reverted")

	// errFolderAlreadyFull is the error trying to add a sector to an already full folder
	errFolderAlreadyFull = errors.New("folder already full")

	// errAllFoldersFullOrUsed is the error happened when all folders are full or in use
	errAllFoldersFullOrUsed = errors.New("all folders are full or in use")

	// errDisrupted is the error that is disrupted during test
	errDisrupted = errors.New("disrupted")
)

// updateError is the error happened during processing the update.
// The error should be registered in the update
type updateError struct {
	prepareErr error
	processErr error
	releaseErr error
}

func newUpdateError() *updateError {
	return &updateError{}
}

// Error return the organized error message of the updateErr
func (upErr *updateError) Error() string {
	var parsedErr error
	// compose the prepareErr
	if upErr.prepareErr != nil {
		parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("prepare error: %v", upErr.prepareErr))
	}
	// compose the processErrs
	if upErr.processErr != nil {
		parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("process error: %v", upErr.processErr))
	}
	// compose the reverseErr
	if upErr.releaseErr != nil {
		parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("release error: %v", upErr.releaseErr))
	}
	return parsedErr.Error()
}

// setPrepareError set the prepare error for the updateError
func (upErr *updateError) setPrepareError(err error) *updateError {
	upErr.prepareErr = err
	return upErr
}

// addProcessError add an error to the updateError
func (upErr *updateError) setProcessError(err error) *updateError {
	upErr.processErr = err
	return upErr
}

// setReleaseError set the release error
func (upErr *updateError) setReleaseError(err error) *updateError {
	upErr.releaseErr = err
	return upErr
}

// isNil determines whether the updateError is nil
func (upErr *updateError) isNil() (isNil bool) {
	isNil = upErr.prepareErr == nil && upErr.processErr == nil && upErr.releaseErr == nil
	return
}

// hasErrStopped determines whether the updateError contains error errStopped
func (upErr *updateError) hasErrStopped() (has bool) {
	return upErr.prepareErr == errStopped || upErr.processErr == errStopped || upErr.releaseErr == errStopped
}

// logError determine the behavior of logging the updateErr for the update
func (sm *storageManager) logError(up update, err *updateError) {
	// If there is errReverted, set it to nil
	if err.prepareErr == errRevert {
		err.prepareErr = nil
	}
	if err.processErr == errRevert {
		err.processErr = nil
	}
	// If there is no error in err, or the error is errStopped, simply return
	if err == nil || err.isNil() || err.hasErrStopped() {
		return
	}
	var msg string
	if up != nil {
		msg = fmt.Sprintf("cannot %v", up.str())
	} else {
		msg = "error during update"
	}
	// compose the arguments for log
	var args []interface{}
	if err.prepareErr != nil {
		args = append(args, "prepare err", err.prepareErr)
	}
	if err.processErr != nil {
		args = append(args, "process err", err.processErr)
	}
	if err.releaseErr != nil {
		args = append(args, "release err", err.releaseErr)
	}
	// If no error recorded, no logging is needed
	if len(args) == 0 {
		return
	}
	sm.log.Warn(msg, args...)
}
