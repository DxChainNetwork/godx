// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"math/rand"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// TODO(mzhang): implement this.
// contractManager is the contractManager interface used in file system
type contractManager interface {
	// HostHealthMapByID return storage.HostHealthInfoTable for hosts specified by input
	HostHealthMapByID([]enode.ID) storage.HostHealthInfoTable

	// HostHealthMap returns the full host info table of the contract manager
	HostHealthMap() (infoTable storage.HostHealthInfoTable)
}

// AlwaysSuccessContractManager is the contractManager that always return good condition for all host keys
type AlwaysSuccessContractManager struct{}

// HostHealthMapByID always return good condition
func (c *AlwaysSuccessContractManager) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		table[id] = storage.HostHealthInfo{
			Offline:      false,
			GoodForRenew: true,
		}
	}
	return table
}

// AlwaysSuccessContractManager is the contractManager that always return wrong condition for all host keys
type alwaysFailContractManager struct{}

// HostHealthMapByID always return bad condition
func (c *alwaysFailContractManager) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		table[id] = storage.HostHealthInfo{
			Offline:      true,
			GoodForRenew: false,
		}
	}
	return table
}

// randomContractManager is the contractManager that return condition is random possibility
// rate is the possibility between 0 and 1 for specified conditions
type randomContractManager struct {
	missRate         float32 // missRate is the rate that the input id is not in the table
	onlineRate       float32 // onlineRate is the rate the the id is online
	goodForRenewRate float32 // goodForRenewRate is the rate of goodForRenew

	missed map[enode.ID]struct{}       // missed node should be forever missed
	table  storage.HostHealthInfoTable // If previously stored the table, do not random again
	once   sync.Once                   // Only initialize the HostHealthInfoTable once
	lock   sync.Mutex                  // lock is the mutex to protect the table field
}

// HostHealthMapByID gives random host health map provided by ids
func (c *randomContractManager) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	c.once.Do(func() {
		c.table = make(storage.HostHealthInfoTable)
		c.missed = make(map[enode.ID]struct{})
	})
	rand.Seed(time.Now().UnixNano())
	c.lock.Lock()
	defer c.lock.Unlock()
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		// previously missed id will be forever missed
		if _, exist := c.missed[id]; exist {
			continue
		}
		if _, exist := c.table[id]; exist {
			table[id] = c.table[id]
			continue
		}
		num := rand.Float32()
		if num < c.missRate {
			c.missed[id] = struct{}{}
			continue
		}
		num = rand.Float32()
		var offline, goodForRenew bool
		if num >= c.onlineRate {
			offline = true
		}
		num = rand.Float32()
		if num < c.goodForRenewRate {
			goodForRenew = true
		}
		c.table[id] = storage.HostHealthInfo{
			Offline:      offline,
			GoodForRenew: goodForRenew,
		}
		table[id] = c.table[id]
	}
	return table
}

// HostHealthMap is not used in tests thus not implemented
func (c *randomContractManager) HostHealthMap() storage.HostHealthInfoTable {
	return make(storage.HostHealthInfoTable)
}
