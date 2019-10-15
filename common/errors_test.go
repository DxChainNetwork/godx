// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package common

import (
	"errors"
	"fmt"
	"testing"
)

var err1 = errors.New("testing error 1")
var err2 = errors.New("testing error 2")
var err3 = errors.New("testing error 3")
var err4 = errors.New("testing error 4")

var nilErr error

func TestErrCompose(t *testing.T) {
	expectedOutput := fmt.Sprintf("{%s; %s; %s}", err1.Error(), err2.Error(), err3.Error())
	err := ErrCompose(err1, err2, err3)

	if expectedOutput != err.Error() {
		t.Errorf("output does not match with expectiatioin: expected: %s, got: %s", expectedOutput, err.Error())
	}
}

func TestErrExtend(t *testing.T) {
	errCompose := ErrCompose(err1, err2, err3)
	err := ErrExtend(errCompose, err4)
	expectedOutput := fmt.Sprintf("{%s; %s; %s; %s}", err1.Error(), err2.Error(), err3.Error(), err4.Error())
	if err.Error() != expectedOutput {
		t.Errorf("output does not match with expectiation: expected: %s, got %s", expectedOutput, err.Error())
	}

	err = ErrExtend(err4, errCompose)
	expectedOutput = fmt.Sprintf("{%s; %s; %s; %s; %s}", err4.Error(), err1.Error(), err2.Error(), err3.Error(), err4.Error())
	if err.Error() != expectedOutput {
		t.Errorf("output does not match with expectiation: expected: %s, got %s", expectedOutput, err.Error())
	}

	err = ErrExtend(err1, err2)
	expectedOutput = fmt.Sprintf("{%s; %s}", err1.Error(), err2.Error())
	if err.Error() != expectedOutput {
		t.Errorf("output does not match with expectiation: expected: %s, got %s", expectedOutput, err.Error())
	}
}

func TestErrContains(t *testing.T) {
	errCompose := ErrCompose(err1, err2)
	contain := ErrContains(errCompose, err1)
	if !contain {
		t.Errorf("source error: %s, target error: %s, the target error suppose to be contained in the source error",
			errCompose.Error(), err1.Error())
	}

	contain = ErrContains(err1, err2)
	if contain {
		t.Errorf("source error: %s, target error: %s, the target error should not be contained in the soruce error",
			err1.Error(), err2.Error())
	}

	contain = ErrContains(nilErr, err1)
	if contain {
		t.Errorf("source error: %s, target error: %s, the target error should not be contained in the source error",
			nilErr.Error(), err1.Error())
	}

	contain = ErrContains(err1, err1)
	if !contain {
		t.Errorf("source error: %s, target error: %s, the target error suppose to be contained in the source",
			err1.Error(), err1.Error())
	}
}
