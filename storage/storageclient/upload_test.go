// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/pborman/uuid"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestUploadDirectory(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	rt := newStorageClientTester()
	defer rt.Client.Close()

	dxPath := filepath.Dir("dxchain")
	testUploadFile, err := ioutil.TempFile(dxPath, "*")
	if err != nil {
		t.Fatal(err)
	}

	ec, err := erasurecode.New(erasurecode.ECTypeStandard, DefaultMinSectors, DefaultNumSectors)
	if err != nil {
		t.Fatal(err)
	}
	params := FileUploadParams{
		Source:      filepath.Join(dxPath, testUploadFile.Name()),
		DxPath:      storage.DxPath{Path: uuid.New()},
		ErasureCode: ec,
	}
	err = rt.Client.Upload(params)
	if err == nil {
		t.Fatal("expected Upload to fail with empty directory as source")
	}
}

func TestSome(t *testing.T) {
	m := make(map[string][]int)
	a := []int{1, 2}
	m["t"] = a

	for k, v := range m {
		v = v[1:]
		m[k] = v

	}
	for k, v := range m {
		fmt.Println(k)
		fmt.Println(v)
	}

}
