// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// +build linux darwin netbsd openbsd solaris

package fdlimit

import (
	"runtime"
	"syscall"
)

// darwinRMax is the maximum resource limit for darwin OS system.
// For Go 1.12.1, setting a current limit larger than this value will return
// an invalid argument error. We fix this error by hard coding the limit for darwin
// system and cap max value to this limit for darwin system.
//
// Discussion for this issue is here: https://github.com/golang/go/issues/30401
var darwinRMax = uint64(24576)

// Raise tries to maximize the file descriptor allowance of this process
// to the maximum hard-limit allowed by the OS.
func Raise(max uint64) error {
	// Get the current limit
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" && max > darwinRMax {
		max = darwinRMax
	}
	// Try to update the limit to the max allowance
	limit.Cur = limit.Max
	if limit.Cur > max {
		limit.Cur = max
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return err
	}
	return nil
}

// Current retrieves the number of file descriptors allowed to be opened by this
// process.
func Current() (int, error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, err
	}
	return int(limit.Cur), nil
}

// Maximum retrieves the maximum number of file descriptors this process is
// allowed to request for itself.
func Maximum() (int, error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, err
	}
	// If the operating system is darwin, set the rlimit to hard coded value.
	if runtime.GOOS == "darwin" && limit.Max > darwinRMax {
		return int(darwinRMax), nil
	}
	return int(limit.Max), nil
}
