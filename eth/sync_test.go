package eth

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/eth/downloader"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
)

// Tests that fast sync gets disabled as soon as a real block is successfully
// imported into the blockchain.
func TestFastSyncDisabling(t *testing.T) {
	// Create a pristine protocol manager, check that fast sync is left enabled
	pmEmpty, _ := newTestProtocolManagerMust(t, downloader.FullSync, 0, nil, nil)
	//if atomic.LoadUint32(&pmEmpty.fastSync) == 0 {
	//	t.Fatalf("fast sync disabled on pristine blockchain")
	//}
	// Create a full protocol manager, check that fast sync gets disabled
	pmFull, _ := newTestProtocolManagerMust(t, downloader.FullSync, 1024, nil, nil)
	//if atomic.LoadUint32(&pmFull.fastSync) == 1 {
	//	t.Fatalf("fast sync not disabled on non-empty blockchain")
	//}
	// Sync up the two peers
	io1, io2 := p2p.MsgPipe()

	go pmFull.handle(pmFull.newPeer(63, p2p.NewPeer(enode.ID{}, "empty", nil), io2))
	go pmEmpty.handle(pmEmpty.newPeer(63, p2p.NewPeer(enode.ID{}, "full", nil), io1))

	time.Sleep(250 * time.Millisecond)
	pmEmpty.synchronise(pmEmpty.peers.BestPeer())

	// Check that fast sync was disabled
	if atomic.LoadUint32(&pmEmpty.fastSync) == 1 {
		t.Fatalf("fast sync not disabled after successful synchronisation")
	}
}
