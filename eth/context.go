// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package eth

import (
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"sync"
)

type Context interface {
	Store(peerID string, key string, context interface{})
	Load(peerID string, key string) interface{}
}

type StorageContractContextInfo struct {
	StorageContract         types.StorageContract
	StorageContractRevision types.StorageContractRevision
	StorageObligation       storagehost.StorageObligation
	Extra                   []byte
}

type StorageContractContext struct {
	Env  map[string]map[string]StorageContractContextInfo
	lock sync.RWMutex
}

func NewStorageContractContext() *StorageContractContext {
	return &StorageContractContext{
		Env: make(map[string]map[string]StorageContractContextInfo),
	}
}

func (c *StorageContractContext) Store(peerID string, key string, context interface{}) {
	if info, ok1 := c.Env[peerID]; ok1 {
		info[key] = context.(StorageContractContextInfo)
	} else {
		m := make(map[string]StorageContractContextInfo)
		m[key] = context.(StorageContractContextInfo)
		c.Env[peerID] = m
	}
}

func (c *StorageContractContext) Load(peerID string, key string) interface{} {
	if info, ok1 := c.Env[peerID]; ok1 {
		return info[key]
	}
	return nil
}
