package filesystem

import (
	"github.com/DxChainNetwork/godx/storage"
	"sync/atomic"
)

const (
	redoNotNeeded uint32 = iota
	redoNeeded
)

// dirMetadataUpdate is a single dirMetadataUpdate for updating a dxdir metadata
type dirMetadataUpdate struct {
	// dxPath is the path relates to the root path of the directory currently being repaired
	dxPath storage.DxPath

	// updateInProgress is the atomic field of whether the current threadedUpdate is on going.
	// There should be at most one threadedUpdate on going
	updateInProgress uint32

	// stop is a channel indicating whether a stop request is received for the update.
	// The source of stop comes from two conditions:
	//  1. the program is shutting down
	//  2. A new thread is trying to update the current DxPath
	stop chan struct{}

	// redo is the atomic field of whether the update should be redo again.
	// The redo field check only happens after the a struct is received from stop channel.
	// If the condition is shutting down the program, redo should be RedoNotNeeded,
	// If the condition is a new thread accessing, redo should be RedoNeeded.
	// During normal processing, redo should be 0.
	redo uint32
}

func (fs *FileSystem) NewDirMetadataUpdate(path storage.DxPath) error {

}

// stopAndRedo is a helper function that update the redo field and try to fill the stop channel.
// It is triggered when a new update is created while currently a dirMetadataUpdate is in progress.
func (update *dirMetadataUpdate) stopAndRedo() {
	atomic.StoreUint32(&update.redo, redoNeeded)
	// Try to insert into the channel. If cannot, there is already struct in stop channel.
	select {
	case update.stop <- struct{}{}:
	default:
	}
}
