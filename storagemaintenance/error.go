// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemaintenance

import "errors"

var (
	// ErrProgramExit indicates that the program will exit (Ctrl + c)
	ErrProgramExit       = errors.New("program exist")
)
