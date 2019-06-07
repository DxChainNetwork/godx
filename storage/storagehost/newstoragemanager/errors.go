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
)

// updateError is the error happened during processing the update.
// The error should be registered in the update
type updateError struct {
	prepareErr  error
	processErrs []error
	reverseErrs []error
	releaseErr  error
}

func newUpdateError() *updateError {
	return &updateError{
		processErrs: make([]error, 0, numConsecutiveFailsRelease),
		reverseErrs: make([]error, 0, numConsecutiveFailsRelease),
	}
}

// Error return the organized error message of the updateErr
func (upErr *updateError) Error() string {
	var parsedErr error
	// compose the prepareErr
	if upErr.prepareErr != nil {
		parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("prepare error: %v", upErr.prepareErr))
	}
	// compose the processErrs
	for i, processErr := range upErr.processErrs {
		if processErr != nil {
			parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("process error[%d]: %v", i, processErr))
		}
	}
	// compose the reverseErr
	for i, reverseErr := range upErr.reverseErrs {
		if reverseErr != nil {
			parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("reverse error[%d]: %v", i, reverseErr))
		}
	}
	parsedErr = common.ErrCompose(parsedErr, fmt.Errorf("release error: %v", upErr.releaseErr))
	return parsedErr.Error()
}

// setPrepareError set the prepare error for the updateError
func (upErr *updateError) setPrepareError(err error) *updateError {
	upErr.prepareErr = err
	return upErr
}

// addProcessError add an error to the updateError
func (upErr *updateError) addProcessError(err error) *updateError {
	upErr.processErrs = append(upErr.processErrs, err)
	return upErr
}

// setReverseError set the reverse error for the updateError
func (upErr *updateError) addReverseError(err error) *updateError {
	upErr.reverseErrs = append(upErr.reverseErrs, err)
	return upErr
}

// setReleaseError set the release error
func (upErr *updateError) setReleaseError(err error) *updateError {
	upErr.releaseErr = err
	return upErr
}

// isNil determines whether the updateError is nil
func (upErr *updateError) isNil() (isNil bool) {
	isNil = upErr.prepareErr == nil && len(upErr.processErrs) == 0 && len(upErr.processErrs) == 0
	return
}

// logError determine the behavior of logging the updateErr for the update
func (sm *storageManager) logError(up update, err *updateError) {
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
	if len(err.processErrs) != 0 {
		for i, processErr := range err.processErrs {
			args = append(args, fmt.Sprintf("process err[%d]", i), processErr)
		}
	}
	if len(err.reverseErrs) != 0 {
		for i, reverseErr := range err.reverseErrs {
			args = append(args, fmt.Sprintf("reverse err[%d]", i), reverseErr)
		}
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
