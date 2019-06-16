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
//  2. whitelist: only the storage host in both whitelist and hostPool can be inserted into filteredTree
const (
	DisableFilter FilterMode = iota
	WhitelistFilter
)

// SetFilterMode will be used to set the host ip filter mode. Actions are required only
// when the mode is set to be whitelist, meaning that only the storage host in both whitelist
// and hostPool can be inserted into the filteredTree
func (shm *StorageHostManager) SetFilterMode(fm FilterMode, hostInfo []enode.ID) error {
	shm.lock.Lock()
	defer shm.lock.Unlock()

	// if the filter is disabled, return directly
	if fm == DisableFilter {
		return nil
	}

	// if the filter mode is not disabled and it is not hostInfo, then return error
	if fm != WhitelistFilter {
		return errors.New("filter mode provided not recognized")
	}

	// if filter mode is blacklist filter

	// check the number of hosts in the hostInfo, if there are no hostInfo hosts defined, return error
	if len(hostInfo) == 0 {
		return errors.New("failed to set the filter mode, empty hostInfo")
	}

	// initialize filtered tree
	shm.filteredTree = storagehosttree.New(shm.evalFunc)
	shm.filteredHosts = make(map[enode.ID]struct{})
	shm.filterMode = fm

	// update the filter host
	for _, id := range hostInfo {
		shm.filteredHosts[id] = struct{}{}
	}

	// insert the hosts contained in the white list to the filtered tree
	allHosts := shm.storageHostTree.All()
	for _, host := range allHosts {
		if _, exist := shm.filteredHosts[host.EnodeID]; exist {
			if err := shm.filteredTree.Insert(host); err != nil {
				return err
			}
		}
	}
	return nil
}

// String will convert the filter mode into string, used for displaying purpose
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
