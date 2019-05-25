package filesystem

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

const (
	redoNotNeeded uint32 = iota
	redoNeeded
)

const (
	dirMetadataUpdateName = "dirMetadataUpdate"

	// numConsecutiveFailRelease defines the time when fail reaches this number,
	// dirMetadataUpdate is release and deleted from map
	numConsecutiveFailRelease = 3
)

var (
	// errUpdateAlreadyInProgress is the error when creating a new update, found
	// there is already an update goroutine in progress.
	errUpdateAlreadyInProgress = errors.New("another goroutine is working on metadata update")

	// errStopped is the error that happens when the system stops during the update
	errStopped = errors.New("file system stopped during update")
)

type (
	// dirMetadataUpdate is a single dirMetadataUpdate for updating a dxdir metadata
	dirMetadataUpdate struct {
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

		// consecutiveFails is the atomic field for counting the consecutive failed times.
		// When consecutiveFails reaches a certain number, the dirMetadata update is released.
		consecutiveFails uint32

		// walTxn is the wal transaction associated with dirMetadataUpdate.
		// It is stored when first time the goroutine is established.
		walTxn *writeaheadlog.Transaction

		// lock is the field to protest walTxn.
		lock sync.Mutex
	}

	// metadataForUpdate contains the least information for dxdir update
	metadataForUpdate struct {
		numFiles            uint64
		totalSize           uint64
		health              uint32
		stuckHealth         uint32
		minRedundancy       uint32
		numStuckSegments    uint32
		timeLastHealthCheck time.Time
	}
)

// InitAndUpdateDirMetadata create the update intent, and then apply the intent.
// The actual metadata update is executed in a thread updateDirMetadata
func (fs *FileSystem) InitAndUpdateDirMetadata(path storage.DxPath) error {
	// Initialize the dirMetadataUpdate, that is, recordDirMetadataUpdate
	txn, err := fs.recordDirMetadataIntent(path)
	if err != nil {
		return fmt.Errorf("cannot update metadata at %v", path.Path)
	}
	// Apply hte update
	if err = fs.updateDirMetadata(path, txn); err != nil {
		// tm already closed or update thread already in progress
		if err == errUpdateAlreadyInProgress {
			err = nil
		}
		err = common.ErrCompose(err, txn.Release())
		return err
	}
	return nil
}

// recordDirMetadataIntent record the dirMetadata intent to the wal
func (fs *FileSystem) recordDirMetadataIntent(path storage.DxPath) (*writeaheadlog.Transaction, error) {
	op, err := createWalOp(path)
	if err != nil {
		return nil, err
	}
	txn, err := fs.dirMetadataWal.NewTransaction([]writeaheadlog.Operation{op})
	if err != nil {
		return nil, err
	}

	if <-txn.InitComplete; txn.InitErr != nil {
		return nil, txn.InitErr
	}
	if err = <-txn.Commit(); err != nil {
		return nil, err
	}
	return txn, nil
}

// createWalOp creates a dir metadata update based on the give path
func createWalOp(path storage.DxPath) (writeaheadlog.Operation, error) {
	b, err := rlp.EncodeToBytes(path)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: dirMetadataUpdateName,
		Data: b,
	}, nil
}

// applyDirMetadataUpdate creates a new dirMetadataUpdate and initialize a
// goroutine of calculateMetadataAndApply as necessary.
//  1. If the update already exists in fs.unfinishedUpdates, signal the current update thread
//     stopAndRedo
//  2. If there is already an update thread in progress, return
//  3. Else add to fs.tm, update fs.unfinishedUpdates, and start a goroutine to
//     calculateMetadataAndApply
// The returned error type is could be two cases:
//  1. threadmanager already stopped. It would be safe to throw the error during error handling
//  2. there is already a thread updating the metadata. return errUpdateAlreadyInProgress
// The function is called in two places:
//  1. A dxfile is updated and the update is bubble to the root
//  2. A MaintenanceLoop loops over the fs.unfinishedUpdates field for previously failed update.
//     In this case, walTxn should be nil
func (fs *FileSystem) updateDirMetadata(path storage.DxPath, walTxn *writeaheadlog.Transaction) (err error) {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	// If the update already exists in the unfinishedUpdates, stopAndRedo
	update, exist := fs.unfinishedUpdates[path]
	if exist {
		update.stopAndRedo()
	} else {
		update = &dirMetadataUpdate{
			dxPath:           path,
			updateInProgress: 0,
			stop:             make(chan struct{}),
			redo:             0,
		}
	}

	// Check whether the update has an update thread in progress.
	// Create an update thread only if the updateInProgress is false
	swapped := atomic.CompareAndSwapUint32(&update.updateInProgress, 0, 1)
	if !swapped {
		// Already an update thread in progress, return
		err = errUpdateAlreadyInProgress
		return
	}
	defer func() {
		// If error happened swap back the updateInProgress value
		if err != nil {
			atomic.StoreUint32(&update.updateInProgress, 0)
		}
	}()
	err = fs.tm.Add()
	if err != nil {
		err = fmt.Errorf("cannot create a dirMetadataUpdate: %v", err)
		return
	}
	fs.unfinishedUpdates[path] = update
	// If the input walTxn is not nil, update the walTxn field
	if walTxn != nil {
		update.lock.Lock()
		update.walTxn = walTxn
		update.lock.Unlock()
	}
	go fs.calculateMetadataAndApply(update)
	return
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

// calculateMetadataAndApply is the threaded function of calculate the metadata
// and save to dxdir. This is the core function of this file
func (fs *FileSystem) calculateMetadataAndApply(update *dirMetadataUpdate) {
	var err error

	for {
		// TODO: Calculate the metadata and apply

		// Termination. Only happens when redo value is redoNotNeeded
		select {
		case <-update.stop:
			err = common.ErrCompose(err, errStopped)
			break
		default:
		}
		if atomic.LoadUint32(&update.redo) == redoNotNeeded {
			break
		}
	}
	// Call cleanUp to clean up the update
	update.cleanUp(fs, err)
	return
}

// cleanUp is the defer function that is called for the goroutine of calculateMetadataAndApply
// It do the following:
// 1. Set the updateInProgress value to 0, notifying this goroutine is over
// 2. Notify thread manager this goroutine is done.
// 3. Determine whether release is necessary
// 4. If release is needed, delete the entry in fs.unfinishedUpdates.
// 5. If release is needed, Release the transaction
func (update *dirMetadataUpdate) cleanUp(fs *FileSystem, err error) {
	// Set the updateInProgress value to 0, notifying this goroutine is over
	atomic.StoreUint32(&update.updateInProgress, 0)

	// fs.tm.Done()
	fs.tm.Done()

	// determine whether release is necessary
	// release could happen either non err or consecutiveFails reaches the limit
	if err != nil {
		atomic.AddUint32(&update.consecutiveFails, 1)
	}
	fails := atomic.LoadUint32(&update.consecutiveFails)
	var release bool
	if fails == numConsecutiveFailRelease || err == nil {
		release = true
	}

	// 3. If no error happened, delete the entry in fs.unfinishedUpdates.
	// 4. If no error happened, Release the transaction
	if release {
		fs.lock.Lock()
		delete(fs.unfinishedUpdates, update.dxPath)
		fs.lock.Unlock()

		update.lock.Lock()
		err = common.ErrCompose(err, update.walTxn.Release())
		update.lock.Unlock()
	}
	if err != nil {
		// error handling. Currently just log the error
		if fails == numConsecutiveFailRelease {
			log.Crit("cannot update the metadata", err)
		} else {
			log.Warn("cannot update the metadata", err)
		}
	} else {
		if update.dxPath.IsRoot() {
			// dxPath is already root. return
			return
		}
		parent, err := update.dxPath.Parent()
		if err != nil {
			log.Warn("cannot create parent directory of %v: %v", update.dxPath.Path, err)
			return
		}
		if err = fs.InitAndUpdateDirMetadata(parent); err != nil {
			log.Warn("cannot update parent directory of %v: %v", &update.dxPath.Path, err)
		}

	}
}

// calculateDirMetadata loops over all files under the DxPath and calculate the updated
// metadata of the update
func (fs *FileSystem) LoopDirAndCalculateDirMetadata(update *dirMetadataUpdate) (*dxdir.Metadata, error) {
	// Set default metadata value
	metadata := &dxdir.Metadata{
		NumFiles:            0,
		TotalSize:           0,
		Health:              dxdir.DefaultHealth,
		StuckHealth:         dxdir.DefaultHealth,
		MinRedundancy:       math.MaxUint32,
		TimeLastHealthCheck: uint64(time.Now().Unix()),
		TimeModify:          uint64(time.Now().Unix()),
		NumStuckSegments:    0,
		DxPath:              update.dxPath,
		RootPath:            fs.rootDir,
	}
	// Read all files and directories under the path
	fileinfos, err := ioutil.ReadDir(string(fs.rootDir.Join(update.dxPath)))
	if err != nil {
		return nil, err
	}
	// Iterate over all files under the directory
	for _, file := range fileinfos {
		// If there is a stop signal, return the error of errStopped
		select {
		case <-update.stop:
			return nil, errStopped
		default:
		}
		ext := filepath.Ext(file.Name())
		var md *metadataForUpdate
		if ext == storage.DxFileExt {
			// File type DxFile
			md, err = fs.calculateDxFileMetadata(update.dxPath, file.Name())
			if err != nil {
				log.Warn("cannot calculate the file metadata for %v: %v", update.dxPath, err)
				continue
			}
		} else if file.IsDir() {
			// File type DxDir
			md, err = fs.calculateDxDirMetadata(update.dxPath, file.Name())
			if err != nil {
				log.Warn("cannot calculate the file metadata for %v: %v", update.dxPath, err)
				continue
			}
		} else {
			// Ignore all files other than DxFile and DxDir
			continue
		}
		metadata = applyMetadataForUpdateToMetadata(metadata, md)
	}
	return metadata, nil
}

// calculateDxFileMetadata update, calculate and apply the health related field of a dxfile.
func (fs *FileSystem) calculateDxFileMetadata(path storage.DxPath, filename string) (*metadataForUpdate, error) {
	// Deal with the file names. Input path is the DxPath of the target directory.
	// filename is the system filename of the dxfile.
	filenameNoSuffix := strings.TrimSuffix(filename, storage.DxFileExt)
	fileDxPath, err := path.Join(filenameNoSuffix)
	if err != nil {
		return nil, err
	}
	// Open the DxPath
	file, err := fs.fileSet.Open(fileDxPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open DxPath %v: %v", fileDxPath.Path, err)
	}
	defer file.Close()

	// Get the healthInfoMap, mark all healthy as unstuck, and then calculate the health
	healthInfoTable := fs.contractor.HostHealthMapByID(file.HostIDs())
	if err = file.MarkAllHealthySegmentsAsUnstuck(healthInfoTable); err != nil {
		return nil, fmt.Errorf("cannot calculate DxFile metadata for file %v: %v", fileDxPath.Path, err)
	}
	health, stuckHealth, numStuckSegments := file.Health(healthInfoTable)
	redundancy := file.Redundancy(healthInfoTable)

	// Update TimeLastHealthCheck
	if err := file.SetTimeLastHealthCheck(time.Now()); err != nil {
		return nil, fmt.Errorf("cannot SetTimeLastHealthCheck for file %v: %v", fileDxPath.Path, err)
	}
	cachedMetadata := dxfile.CachedHealthMetadata{
		Health:      health,
		StuckHealth: stuckHealth,
		Redundancy:  redundancy,
	}
	// apply cached metadata and return
	return &metadataForUpdate{
		numFiles:            1,
		totalSize:           file.FileSize(),
		health:              health,
		stuckHealth:         stuckHealth,
		minRedundancy:       redundancy,
		numStuckSegments:    numStuckSegments,
		timeLastHealthCheck: time.Now(),
	}, file.ApplyCachedHealthMetadata(cachedMetadata)
}

// calculateDxDirMetadata calculate and return the metadata from the .dxdir file
func (fs *FileSystem) calculateDxDirMetadata(path storage.DxPath, filename string) (*metadataForUpdate, error) {
	path, err := path.Join(filename)
	if err != nil {
		return nil, err
	}
	d, err := fs.dirSet.Open(path)
	if os.IsNotExist(err) {
		// The .dxdir not exist. Create a new one
		d, err = fs.dirSet.NewDxDir(path)
		if err != nil {
			return nil, fmt.Errorf("cannot create the .dxdir file for file %v: %v", path.Path, err)
		}
	} else if err != nil {
		return nil, err
	}
	// No error, or the dxdir is created.
	rawMetadata := d.Metadata()
	return &metadataForUpdate{
		numFiles:            rawMetadata.NumFiles,
		totalSize:           rawMetadata.TotalSize,
		health:              rawMetadata.Health,
		stuckHealth:         rawMetadata.StuckHealth,
		minRedundancy:       rawMetadata.MinRedundancy,
		numStuckSegments:    rawMetadata.NumStuckSegments,
		timeLastHealthCheck: time.Unix(int64(d.Metadata().TimeLastHealthCheck), 0),
	}, nil
}

// applyMetadataForUpdateToMetadata apply a metadataForUpdate to dxdir.Metadata
func applyMetadataForUpdateToMetadata(md *dxdir.Metadata, update *metadataForUpdate) *dxdir.Metadata {
	md.NumFiles += update.numFiles
	md.TotalSize += update.totalSize
	// If the update health or stuckHealth has higher priority than md.Health, update
	if dxfile.CmpHealthPriority(update.health, md.Health) > 0 {
		md.Health = update.health
	}
	if dxfile.CmpHealthPriority(update.stuckHealth, md.StuckHealth) > 0 {
		md.StuckHealth = update.stuckHealth
	}
	// Update minRedundancy
	if update.minRedundancy < md.MinRedundancy {
		md.MinRedundancy = update.minRedundancy
	}
	md.NumStuckSegments += update.numStuckSegments
	// update timeLastHealthCheck. TimeLastHealthCheck is the oldest time for health check
	if uint64(update.timeLastHealthCheck.Unix()) < md.TimeLastHealthCheck {
		md.TimeLastHealthCheck = uint64(update.timeLastHealthCheck.Unix())
	}
	return md
}
