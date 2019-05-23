// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

// downloadDestination is a wrapper for the different types of writing that we
// can do when recovering and writing the logical data of a file.
type downloadDestination interface {
	WriteAt(data []byte, offset int64) (int, error)
}
