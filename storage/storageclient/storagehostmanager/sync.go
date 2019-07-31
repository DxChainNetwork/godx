// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
	"time"
)

// subscribeChainChangeEvent will receive changes on the blockchain (blocks added / reverted)
// once received, a function will be triggered to analyze those blocks
func (shm *StorageHostManager) subscribeChainChangEvent() {
	if err := shm.tm.Add(); err != nil {
		return
	}
	defer shm.tm.Done()

	chainChanges := make(chan core.ChainChangeEvent, 100)
	shm.b.SubscribeChainChangeEvent(chainChanges)

	for {
		select {
		case change := <-chainChanges:
			shm.analyzeChainEventChange(change)
		case <-shm.tm.StopChan():
			return
		}
	}
}

// analyzeChainEventChange will analyze block changing event and update the corresponded
// storage host manager field (block height)
func (shm *StorageHostManager) analyzeChainEventChange(change core.ChainChangeEvent) {

	revert := len(change.RevertedBlockHashes)
	apply := len(change.AppliedBlockHashes)

	// update the block height
	shm.lock.Lock()
	for i := 0; i < revert; i++ {
		shm.blockHeight--
		if shm.blockHeight < 0 {
			shm.log.Error("the block height stores in StorageHostManager should be positive")
			shm.blockHeight = 0
			break
		}
	}

	for i := 0; i < apply; i++ {
		shm.blockHeight++
	}
	shm.lock.Unlock()

	// get the block information
	for _, hash := range change.AppliedBlockHashes {
		hostAnnouncements, _, err := shm.b.GetHostAnnouncementWithBlockHash(hash)
		if err != nil {
			shm.log.Error("error extracting host announcement", "block hash", hash, "err", err.Error())
			continue
		}
		shm.analyzeHostAnnouncements(hostAnnouncements)
	}
}

// analyzeHostAnnouncements will parse the storage host announcement and insert it into the storage host
// manager
func (shm *StorageHostManager) analyzeHostAnnouncements(hostAnnouncements []types.HostAnnouncement) {
	// check if the block contains host announcements information
	if len(hostAnnouncements) == 0 {
		return
	}

	// loop through the hostAnnouncements,
	for _, announcement := range hostAnnouncements {
		info, err := parseHostAnnouncement(announcement)
		if err != nil {
			shm.log.Error("failed to parse the announcement information", "err", err.Error())
			continue
		}

		// check if the announcement is made by the local node
		if info.EnodeURL == shm.b.SelfEnodeURL() {
			continue
		}

		shm.insertStorageHostInformation(info)
	}
}

// insertStorageHostInformation will insert the storage host information into the storage
// host manager
func (shm *StorageHostManager) insertStorageHostInformation(info storage.HostInfo) {
	// check if the storage host information already existed
	oldInfo, exists := shm.storageHostTree.RetrieveHostInfo(info.EnodeID)
	if !exists {
		// if not existed before, modify the FirstSeen and insert into storage host manager,
		// start the scan loop
		info.FirstSeen = shm.blockHeight
		if err := shm.insert(info); err != nil {
			shm.log.Error("unable to insert the storage host information", "err", err.Error())
		}

		// start the scan
		shm.scanValidation(info)
		return
	}

	// if the storage host information already existed, update the settings
	oldInfo.EnodeURL = info.EnodeURL
	oldInfo.IP = info.IP

	// check if the ip address has been changed, if so, update the IP network field
	// and update the LastIPNetWorkChange time
	networkAddr, err := storagehosttree.IPNetwork(oldInfo.IP)
	if err != nil {
		shm.log.Error("failed to extract the network address from the IP address", "err", err.Error())
	} else if networkAddr.String() != oldInfo.IPNetwork {
		oldInfo.IPNetwork = networkAddr.String()
		oldInfo.LastIPNetWorkChange = time.Now()
	}

	// modify the old storage host information
	if err := shm.modify(oldInfo); err != nil {
		shm.log.Error("failed to modify the old storage host information", "err", err.Error())
	}

	// start the scan
	shm.scanValidation(oldInfo)
}

// parseHostAnnouncement will parse the storage host announcement into storage.HostInfo type
func parseHostAnnouncement(announcement types.HostAnnouncement) (hostInfo storage.HostInfo, err error) {
	hostInfo.EnodeURL = announcement.NetAddress

	// parse the enode URL, get enode id and ip address
	node, err := enode.ParseV4(announcement.NetAddress)
	if err != nil {
		return
	}
	hostInfo.EnodeID = node.ID()
	hostInfo.IP = node.IP().String()
	hostInfo.NodePubKey = crypto.FromECDSAPub(node.Pubkey())

	return
}
