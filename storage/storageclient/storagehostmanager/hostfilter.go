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
//  2. whitelist: only the storage host in both whitelist will be inserted into filteredTree
//  3. blacklist: the storage host in the blacklist will not be inserted into filteredTree
const (
	DisableFilter FilterMode = iota
	WhitelistFilter
	BlacklistFilter
)

// RetrieveFilterMode will get the storage host manager filter mode information
func (shm *StorageHostManager) RetrieveFilterMode() (fm string) {
	shm.lock.RLock()
	defer shm.lock.RUnlock()
	return shm.filterMode.String()
}

// SetFilterMode will be used to set the host ip filter mode. Actions are required only
// when the mode is set to be whitelist, meaning that only the storage host in both whitelist
// and hostPool can be inserted into the filteredTree
func (shm *StorageHostManager) SetFilterMode(fm FilterMode, hostInfo []enode.ID) error {
	shm.lock.Lock()
	defer shm.lock.Unlock()

	// if the filter is disabled, return directly
	if fm == DisableFilter {
		shm.filteredTree = shm.storageHostTree
		shm.filteredHosts = make(map[enode.ID]struct{})
		shm.filterMode = fm
		return nil
	}

	// if the filter mode is not disabled and it is not hostInfo, then return error
	if fm != WhitelistFilter && fm != BlacklistFilter {
		return errors.New("filter mode provided not recognized")
	}

	// if filter mode is blacklist filter

	// check the number of hosts in the hostInfo, if there are no hostInfo hosts defined, return error
	if len(hostInfo) == 0 {
		return errors.New("failed to set the filter mode, empty hostInfo")
	}

	isWhitelist := fm == WhitelistFilter

	// initialize filtered tree
	shm.filteredTree = storagehosttree.New()
	shm.filteredHosts = make(map[enode.ID]struct{})
	shm.filterMode = fm

	// update the filter host
	for _, id := range hostInfo {
		shm.filteredHosts[id] = struct{}{}
	}

	// if whitelist and exist: insert into filteredTree
	// if blacklist and not exist: insert into filteredTree
	// filteredTree contains only valid/authorized filter hosts
	allHosts := shm.storageHostTree.All()
	for _, host := range allHosts {
		if _, exist := shm.filteredHosts[host.EnodeID]; exist == isWhitelist {
			score := shm.hostEvaluator.Evaluate(host)
			if err := shm.filteredTree.Insert(host, score); err != nil {
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
	case fm == BlacklistFilter:
		return fmt.Sprintf("Blacklist")
	default:
		return fmt.Sprintf("Nil")
	}
}

// ToFilterMode is used to convert the string formatted filterMode to FilterMode format
func ToFilterMode(fm string) (filterMode FilterMode, err error) {
	switch {
	case fm == "Disabled":
		filterMode = DisableFilter
		return
	case fm == "Whitelist":
		filterMode = WhitelistFilter
		return
	case fm == "Blacklist":
		filterMode = BlacklistFilter
		return
	default:
		err = fmt.Errorf("filter mode is not valid, available modes are: Disabled, Whitelist, and Blacklist")
		return
	}
}
