// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"os"
	"os/user"
	"path/filepath"
)

type StorageClientTester struct {
	Client *StorageClient
}

func newStorageClientTester() *StorageClientTester {
	client, err := New(filepath.Join(homeDir(), "storageclient"))
	if err != nil {
		return nil
	}
	return &StorageClientTester{Client: client}
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
