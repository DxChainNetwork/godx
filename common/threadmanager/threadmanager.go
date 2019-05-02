package threadmanager

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"sync"
)

// ErrStopped would be throw when the thread manager no longer working
var ErrStopped = errors.New("ThreadManager already stopped")

// ThreadManager manage the thread and close the thread before
// system shutting down
type ThreadManager struct {
	stopFn  []func() error
	deferFn []func() error

	once     sync.Once
	stopChan chan struct{}
	wgLock   sync.Mutex
	fnLock   sync.Mutex
	wg       sync.WaitGroup
}

// init the off chanel
func (tg *ThreadManager) init() {
	tg.stopChan = make(chan struct{})
}

// isOff check is the thread manager is alive
func (tg *ThreadManager) isOff() bool {
	tg.once.Do(tg.init)
	select {
	case <-tg.stopChan:
		return true
	default:
		return false
	}
}

// Add a new task to wait group
func (tg *ThreadManager) Add() error {
	tg.wgLock.Lock()
	defer tg.wgLock.Unlock()

	if tg.isOff() {
		return ErrStopped
	}
	tg.wg.Add(1)
	return nil
}

// PushDeferFn push the function defer to stack, when Stop is called, the
// defer closing function would be execute at the end of Stop
func (tg *ThreadManager) PushDeferFn(fn func() error) error {
	tg.fnLock.Lock()
	if tg.isOff() {
		tg.fnLock.Unlock()
		return common.ErrCompose(ErrStopped, fn())
	}
	tg.deferFn = append(tg.deferFn, fn)
	tg.fnLock.Unlock()
	return nil
}

// PushStopFn push the function to stack, when Stop is called, the closing function
// would be execute when Stop is called
func (tg *ThreadManager) PushStopFn(fn func() error) error {
	tg.fnLock.Lock()
	if tg.isOff() {
		tg.fnLock.Unlock()
		return common.ErrCompose(ErrStopped, fn())
	}
	tg.stopFn = append(tg.stopFn, fn)
	tg.fnLock.Unlock()
	return nil
}

// Done manage one of the wait function is done
func (tg *ThreadManager) Done() {
	tg.wg.Done()
}

// Stop turn off the thread manager, it would first pop from stack
// and execute the function in stack, the pop and execute the
// function in defer stack
func (tg *ThreadManager) Stop() error {
	if tg.isOff() {
		return ErrStopped
	}
	tg.wgLock.Lock()
	close(tg.stopChan)
	tg.wgLock.Unlock()

	tg.fnLock.Lock()
	tg.fnLock.Unlock()

	var err error
	for i := len(tg.stopFn) - 1; i >= 0; i-- {
		err = common.ErrExtend(err, tg.stopFn[i]())
	}

	tg.wg.Wait()

	for i := len(tg.deferFn) - 1; i >= 0; i-- {
		err = common.ErrExtend(err, tg.deferFn[i]())
	}
	return err
}

// GetStopChan get the read-only chanel from the thread group for caller
func (tg *ThreadManager) GetStopChan() <-chan struct{} {
	tg.once.Do(tg.init)
	return tg.stopChan
}
