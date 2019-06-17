// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrAlreadyRoot is the error happens when calling Parent function on
	// a root directory
	ErrAlreadyRoot = errors.New("cannot call the Parent function on the root directory")

	// ErrEmptyDxPath is the error happens when calling NewDxPath on an empty string
	ErrEmptyDxPath = errors.New("cannot create an empty DxPath")

	reservedNames = []string{
		".dxdir",
	}
)

type (
	// DxPath is the file Path or directory Path relates to the root directory of the DxFiles.
	// It is used in storage client and storage client's file system
	DxPath struct {
		Path string
	}

	// SysPath is the actual system Path of a file or a directory.
	SysPath string
)

// NewDxPath create a DxPath with provided s string.
// If validation is not passed for s, an error is returned
func NewDxPath(s string) (DxPath, error) {
	return newDxPath(s)
}

// newDxPath is the function to be used for NewDxPath
// It takes the input string, and clear prefix and suffix / character.
// Then it check whether the input s is valid. If not valid, return an error.
// Finally return the DxPath
func newDxPath(s string) (DxPath, error) {
	dp := DxPath{clean(s)}
	if err := dp.validate(); err != nil {
		return DxPath{}, err
	}
	return dp, nil
}

// clean clean the input string for the usage of DxPath
func clean(s string) string {
	s = filepath.ToSlash(s)
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimSuffix(s, "/")
	return s
}

// validate checks the validity of the dp. It is only used in newDxPath
func (dp DxPath) validate() error {
	if dp.Path == "" {
		return ErrEmptyDxPath
	}
	for _, pathElem := range strings.Split(dp.Path, "/") {
		if pathElem == "." || pathElem == ".." {
			return errors.New("dxpath could not contain . or .. elements")
		}
		if pathElem == "" || pathElem == "/" {
			return errors.New("dxpath could not contain //")
		}
		for _, reserved := range reservedNames {
			if pathElem == reserved {
				return fmt.Errorf("dxpath conflict with reserved name %v", reserved)
			}
		}
	}
	return nil
}

// IsRoot checks whether a DxPath is a root directory
func (dp DxPath) IsRoot() bool {
	return dp.Path == ""
}

// SysPath return the system Path of the DxPath. It concatenate the input rootDir and DxPath
func (dp DxPath) SysPath(rootDir SysPath) SysPath {
	if dp.IsRoot() {
		return rootDir
	}
	return SysPath(filepath.Join(string(rootDir), dp.Path))
}

// Parent returns the parent DxPath of the DxPath.
// If the receiver is already root, return an error of ErrAlreadyExist
func (dp DxPath) Parent() (DxPath, error) {
	if dp.IsRoot() {
		return DxPath{}, ErrAlreadyRoot
	}
	par := filepath.Dir(dp.Path)
	if par == "." {
		return RootDxPath(), nil
	}
	return DxPath{par}, nil
}

// RootDxPath return the special root DxPath which has Path as empty string
func RootDxPath() DxPath {
	return DxPath{""}
}

// Equals check whether the two DxPath are equal
func (dp DxPath) Equals(dp2 DxPath) bool {
	return dp.Path == dp2.Path
}

// Join join the DxPath with s
func (dp DxPath) Join(s string) (DxPath, error) {
	return NewDxPath(filepath.Join(dp.Path, s))
}

// Join join the receiver syspath with DxPath and some extrafields.
func (sp SysPath) Join(dp DxPath, extra ...string) SysPath {
	path := []string{string(sp)}
	if len(dp.Path) != 0 {
		path = append(path, dp.Path)
	}
	path = append(path, extra...)
	return SysPath(filepath.Join(path...))
}
