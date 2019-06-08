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
	errStopped = errors.New("storage manager has been stopped")

	errInvalidTransactionType = errors.New("invalid wal transaction type")
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

// logError determine the behavior of logging the updateErr for the update
func (sm *storageManager) logError(up update, err *updateError) {
	// If there is no error in err, simply return
	if err == nil || err.isNil() {
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
