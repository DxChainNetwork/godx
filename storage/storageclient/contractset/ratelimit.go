// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"sync/atomic"
)

// RateLimit is the data structure that defines the read and write speed limit in terms
// of bytes per second. It also defines the packet size per transfer.
type RateLimit struct {
	atomicReadBPS    int64
	atomicWriteBPS   int64
	atomicPacketSize uint64
}

// NewRateLimit will initialize the RateLimit object, where readBPS specifies the
// read bytes per second, writeBPS specify the write bytes per second, and the packet
// size specifies the size of data can be uploaded each time.
// readBPS -> 0: unlimited read
// writeBPS -> 0: unlimited write
// packetSize -> 0: 4 MiB by default
func NewRateLimit(readBPS, writeBPS int64, packetSize uint64) (rt *RateLimit) {
	return &RateLimit{
		atomicReadBPS:    readBPS,
		atomicWriteBPS:   writeBPS,
		atomicPacketSize: packetSize,
	}
}

// SetRateLimit will set the rate limit based on the value user provided
func (rl *RateLimit) SetRateLimit(readBPS, writeBPS int64, packetSize uint64) {
	atomic.StoreInt64(&rl.atomicReadBPS, readBPS)
	atomic.StoreInt64(&rl.atomicWriteBPS, writeBPS)
	atomic.StoreUint64(&rl.atomicPacketSize, packetSize)
}

// RetrieveRateLimit will return the current rate limit settings
func (rl *RateLimit) RetrieveRateLimit() (readBPS, writeBPS int64, packetSize uint64) {
	readBPS = atomic.LoadInt64(&rl.atomicReadBPS)
	writeBPS = atomic.LoadInt64(&rl.atomicWriteBPS)
	packetSize = atomic.LoadUint64(&rl.atomicPacketSize)

	return
}
