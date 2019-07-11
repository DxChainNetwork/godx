// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"fmt"
	"math"
	"os"

	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// Upload instructs the storage client to start tracking a file. The storage client will
// automatically upload and repair tracked files using a background loop.
func (sc *StorageClient) Upload(up storage.FileUploadParams) error {
	if err := sc.tm.Add(); err != nil {
		return err
	}
	defer sc.tm.Done()

	// Check whether file is a directory
	sourceInfo, err := os.Stat(up.Source)
	if err != nil {
		return fmt.Errorf("unable to stat input file, error: %v", err)
	}
	if sourceInfo.IsDir() {
		return dxdir.ErrUploadDirectory
	}

	file, err := os.Open(up.Source)
	if err != nil {
		return fmt.Errorf("unable to open the source file, error: %v", err)
	}
	if err := file.Close(); err != nil {
		return err
	}

	// Delete existing file if Override mode
	//if up.Mode == storage.Override {
	//	err := sc.DeleteFile(up.DxPath)
	//	if err != nil && err != dxdir.ErrUnknownPath {
	//		return fmt.Errorf("cannot to delete existing file, error: %v", err)
	//	}
	//}

	// Setup ECTypeStandard's ErasureCode with default params
	if up.ErasureCode == nil {
		up.ErasureCode, _ = erasurecode.New(erasurecode.ECTypeStandard, storage.DefaultMinSectors, storage.DefaultNumSectors)
	}

	numContracts := uint64(len(sc.contractManager.GetStorageContractSet().Contracts()))
	// requiredContracts = ceil(min + redundant/2)
	requiredContracts := math.Ceil(float64(up.ErasureCode.NumSectors()+up.ErasureCode.MinSectors()) / 2)
	if numContracts < uint64(requiredContracts) {
		return fmt.Errorf("not enough contracts to upload file: got %v, needed %v", numContracts, (up.ErasureCode.NumSectors()+up.ErasureCode.MinSectors())/2)
	}

	dirDxPath := up.DxPath

	// Try to create the directory. If ErrPathOverload is returned it already exists
	dxDirEntry, err := sc.fileSystem.DirSet().NewDxDir(dirDxPath)
	if err != os.ErrExist && err != nil {
		return fmt.Errorf("unable to create dx directory for new file, error: %v", err)
	} else if err == nil {
		if err := dxDirEntry.Close(); err != nil {
			return err
		}
	}
	//sc.log.Error("test error for NewDxDir in upload", "error", err)

	cipherKey, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		return fmt.Errorf("generate cipher key error: %v", err)
	}

	// Create the DxFile and add to client
	entry, err := sc.fileSystem.FileSet().NewDxFile(up.DxPath, storage.SysPath(up.Source), false, up.ErasureCode, cipherKey, uint64(sourceInfo.Size()), sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("could not create a new dx file, error: %v", err)
	}
	if sourceInfo.Size() == 0 {
		return fmt.Errorf("source file size is 0, fileName: %s", sourceInfo.Name())
	}

	// Update the health of the DxFile directory recursively to ensure the health is updated with the new file
	go sc.fileSystem.InitAndUpdateDirMetadata(dirDxPath)

	nilHostHealthInfoTable := make(storage.HostHealthInfoTable)

	// Send the upload to the repair loop
	hosts := sc.refreshHostsAndWorkers()

	if err := sc.createAndPushSegments([]*dxfile.FileSetEntryWithID{entry}, hosts, targetUnstuckSegments, nilHostHealthInfoTable); err != nil {
		return err
	}

	select {
	case sc.uploadHeap.segmentComing <- struct{}{}:
	default:
	}
	return nil
}
