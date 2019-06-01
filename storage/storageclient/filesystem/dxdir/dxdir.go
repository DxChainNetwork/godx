// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxdir

import (
	"encoding/json"
	"errors"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	// ErrPathOverload is an error when a siadir already exists at that location
	ErrPathOverload = errors.New("a siadir already exists at that location")
	// ErrUnknownPath is an error when a siadir cannot be found with the given path
	ErrUnknownPath = errors.New("no siadir known with that path")
	// ErrUnknownThread is an error when a siadir is trying to be closed by a
	// thread that is not in the threadMap
	ErrUnknownThread = errors.New("thread should not be calling Close(), does not have control of the siadir")

	ErrUploadDirectory = errors.New("cannot upload directory")
)

const (
	dirFileName = ".dxdir"

	defaultHealth = uint32(200)
)

type (
	// DxDir is the data structure for the directory for the meta info for a directory.
	DxDir struct {
		// metadata
		metadata *Metadata

		// utilities
		deleted bool
		lock    sync.RWMutex
		wal     *writeaheadlog.Wal

		// dirPath is the actual path without dirFileName the DxDir locates
		dirPath dirPath
	}

	// Metadata is the necessary metadata to be saved in DxDir
	Metadata struct {
		// Total number of files in directory and its subdirectories
		NumFiles uint64

		// Total size of the directory and its subdirectories
		TotalSize uint64

		// Health is the min Health all files and subdirectories
		Health uint32

		// StuckHealth is the min StuckHealth for all files and subdirectories
		StuckHealth uint32

		// MinRedundancy is the minimum redundancy
		MinRedundancy uint32

		// TimeLastHealthCheck is the last health check time
		TimeLastHealthCheck time.Time

		// TimeModify is the last content modification time
		TimeModify time.Time

		// NumStuckSegments is the total number of segments that is stuck
		NumStuckSegments uint64

		// DxPath is the DxPath which is the path related to the root directory
		DxPath DxPath
	}

	// dirPath is the system path of the directory
	dirPath string

	// DxPath is the path to the root directory
	DxPath string
)

//New create a DxDir with representing the dirPath metadata.
//Note that the only access method should be from dirSet
func New(dxPath DxPath, sysPath dirPath, wal *writeaheadlog.Wal) (*DxDir, error) {
	_, err := os.Stat(filepath.Join(string(sysPath), dirFileName))
	if err == nil {
		return nil, os.ErrExist
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err = os.MkdirAll(string(sysPath), 0700); err != nil {
		return nil, err
	}
	metadata := &Metadata{
		Health:      defaultHealth,
		StuckHealth: defaultHealth,
		TimeModify:  uint64(time.Now().Unix()),
		DxPath:      dxPath,
	}
	d := &DxDir{
		metadata: metadata,
		deleted:  false,
		wal:      wal,
		dirPath:  sysPath,
	}
	err = d.save()
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Delete delete the dxfile
func (d *DxDir) Delete() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	return d.delete()
}

// Deleted return the delete status
func (d *DxDir) Deleted() bool {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.deleted
}

// Metadata return the copy of the Metadata
func (d *DxDir) Metadata() Metadata {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return *d.metadata
}

// DxPath return the DxPath of the Dxdir
func (d *DxDir) DxPath() DxPath {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.metadata.DxPath
}

// UpdateMetadata update the metadata with the given metadata.
// Not the DxPath field is not updated
func (d *DxDir) UpdateMetadata(metadata Metadata) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	// Update the fields
	d.metadata.NumFiles = metadata.NumFiles
	d.metadata.TotalSize = metadata.TotalSize
	d.metadata.Health = metadata.Health
	d.metadata.StuckHealth = metadata.StuckHealth
	d.metadata.MinRedundancy = metadata.MinRedundancy
	d.metadata.TimeLastHealthCheck = metadata.TimeLastHealthCheck
	d.metadata.TimeModify = metadata.TimeModify
	d.metadata.NumStuckSegments = metadata.NumStuckSegments

	// DxPath field should never be updated
	return d.save()
}

var (
	// ErrEmptyDxPath is an error when DxPath is empty
	ErrEmptyDxPath = errors.New("DxPath must be a nonempty string")

	// DxDirExtension is the extension for Dxdir metadata files on disk
	DxDirExtension = ".Dxdir"

	// DxFileExtension is the extension for Dxfiles on disk
	DxFileExtension = ".Dx"

	RootPath = DxPath("")

	EmptyPath = DxPath("")
)


// RootDxPath returns a DxPath for the root Dxdir which has a blank path
func RootDxPath() DxPath {
	return RootPath
}

// clean cleans up the string by converting an OS separators to forward slashes
// and trims leading and trailing slashes
func clean(s string) string {
	s = filepath.ToSlash(s)
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimSuffix(s, "/")
	return s
}

// newDxPath returns a new DxPath with the path set
func NewDxPath(s string) (DxPath, error) {
	dp := DxPath(clean(s))
	return dp, dp.validate(false)
}

// Dir returns the directory of the DxPath
func (dp DxPath) Dir() (DxPath, error) {
	str := filepath.Dir(string(dp))
	if str == "." {
		return RootDxPath(), nil
	}
	return NewDxPath(str)
}

// Equals compares two DxPath types for equality
func (dp DxPath) Equals(dxPath DxPath) bool {
	return dp == dxPath
}

// IsRoot indicates whether or not the DxPath path is a root directory Dxpath
func (dp DxPath) IsRoot() bool {
	return dp == RootPath
}

// Join joins the string to the end of the DxPath with a "/" and returns
// the new DxPath
func (dp DxPath) Join(s string) (DxPath, error) {
	if s == "" {
		return "", errors.New("cannot join an empty string to a DxPath")
	}
	return NewDxPath(dp.String() + "/" + clean(s))
}

// LoadString sets the path of the DxPath to the provided string
func (dp *DxPath) LoadString(s string) error {
	*dp = DxPath(clean(s))
	return dp.validate(false)
}

// MarshalJSON marshales a DxPath as a string.
func (dp DxPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(dp.String())
}

// UnmarshalJSON unmarshals a Dxpath into a DxPath object.
func (dp *DxPath) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &dp); err != nil {
		return err
	}
	*dp = DxPath(clean(dp.String()))
	return dp.validate(true)
}

// DxDirSysPath returns the system path needed to read a directory on disk, the
// input dir is the root Dxdir directory on disk
func (dp DxPath) DxDirSysPath(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(dp.String()), "")
}

// DxDirMetadataSysPath returns the system path needed to read the DxDir
// metadata file from disk, the input dir is the root Dxdir directory on disk
func (dp DxPath) DxDirMetadataSysPath(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(dp.String()), DxDirExtension)
}

// DxFileSysPath returns the system path needed to read the DxFile from disk,
// the input dir is the root Dxfile directory on disk
func (dp DxPath) DxFileSysPath(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(dp.String())+DxFileExtension)
}

// String returns the DxPath's path
func (dp DxPath) String() string {
	return string(dp)
}

// validate checks that a Dxpath is a legal filename. ../ is disallowed to
// prevent directory traversal, and paths must not begin with / or be empty.
func (dp DxPath) validate(isRoot bool) error {
	if dp == "" && !isRoot {
		return ErrEmptyDxPath
	}
	if dp == ".." {
		return errors.New("Dxpath cannot be '..'")
	}
	if dp == "." {
		return errors.New("Dxpath cannot be '.'")
	}
	// check prefix
	if strings.HasPrefix(dp.String(), "/") {
		return errors.New("Dxpath cannot begin with /")
	}
	if strings.HasPrefix(dp.String(), "../") {
		return errors.New("Dxpath cannot begin with ../")
	}
	if strings.HasPrefix(dp.String(), "./") {
		return errors.New("Dxpath connot begin with ./")
	}
	var prevElem string
	for _, pathElem := range strings.Split(dp.String(), "/") {
		if pathElem == "." || pathElem == ".." {
			return errors.New("Dxpath cannot contain . or .. elements")
		}
		if prevElem != "" && pathElem == "" {
			return ErrEmptyDxPath
		}
		if prevElem == "/" || pathElem == "/" {
			return errors.New("Dxpath cannot contain //")
		}
		prevElem = pathElem
	}
	return nil
}
