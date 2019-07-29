// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/davecgh/go-spew/spew"
)

var dumper = spew.ConfigState{DisableMethods: true, Indent: "    "}

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "storagehost", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v: %v", path, err))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return path
}

func newTestStorageHost(t *testing.T) *StorageHost {
	dir := tempDir(t.Name())
	h, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// load the default settings
	if err = h.load(); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestStorageHost_Load(t *testing.T) {
	h := newTestStorageHost(t)
	// Check whether the file has default settings
	if err := checkHostConfigFile(filepath.Join(h.persistDir, HostSettingFile), defaultConfig()); err != nil {
		t.Fatal(err)
	}
	// Set one of the value
	if err := h.setAcceptContracts(true); err != nil {
		t.Fatal(err)
	}
	// stop the host
	if err := h.tm.Stop(); err != nil {
		t.Fatal(err)
	}
	h.StorageManager.Start()
	h.StorageManager.Close()
	h.db.Close()

	h2, err := New(h.persistDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := h2.load(); err != nil {
		t.Fatal(err)
	}
	newConfig := defaultConfig()
	newConfig.AcceptingContracts = true
	if err := checkHostConfigFile(filepath.Join(h2.persistDir, HostSettingFile), newConfig); err != nil {
		t.Fatal(err)
	}
}

func checkHostConfigFile(path string, expect storage.HostIntConfig) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file stat: %v", err)
	}
	var p persistence
	if err := common.LoadDxJSON(storageHostMeta, path, &p); err != nil {
		return fmt.Errorf("cannot load DxJSON: %v", err)
	}
	if !reflect.DeepEqual(p.Config, expect) {
		return fmt.Errorf("config not expected. \n\tExpect %vGot%v", dumper.Sdump(expect), dumper.Sdump(p.Config))
	}
	return nil
}
