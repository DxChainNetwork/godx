// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

// FilterMode defines a list of storage host that needs to be filtered
// there are four kinds of filter mode defined below
type FilterMode int

// Two kinds of filter mode
//  1. disable: filter mode is not allowed
//  2. whitelist: only the storage host in bot whitelist and hostPool can be inserted into filteredTree
const (
	DisableFilter FilterMode = iota
	WhitelistFilter
)

func (shm *StorageHostManager) SetFilterMode(fm FilterMode, whitelist []enode.ID) error {
	shm.lock.Lock()
	defer shm.lock.Unlock()

	if fm == DisableFilter {
		return nil
	}

	if fm == WhitelistFilter {

		if len(whitelist) == 0 {
			return errors.New("failed to set whitelist filter mode, empty whitelist")
		}

		// initialize filtered tree
		shm.filteredTree = storagehosttree.New(shm.evalFunc)
		shm.filteredHosts = make(map[string]enode.ID)
		shm.filterMode = fm

		// update the filter host
		for _, id := range whitelist {
			_, exist := shm.filteredHosts[id.String()]
			if !exist {
				shm.filteredHosts[id.String()] = id
			}
		}

		// insert the host in the whitelist into the filtered tree
		allHosts := shm.storageHostTree.All()
		for _, host := range allHosts {
			_, exist := shm.filteredHosts[host.EnodeID.String()]
			if exist {
				err := shm.filteredTree.Insert(host)
				if err != nil {
					return err
				}
			}
		}
		return nil

	}

	return errors.New("filter mode provided not recognized")
}

func (fm FilterMode) String() string {
	switch {
	case fm == DisableFilter:
		return fmt.Sprintf("Disabled")
	case fm == WhitelistFilter:
		return fmt.Sprintf("Whitelist")
	default:
		return fmt.Sprintf("Nil")
	}
}
