// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

type (
	// disruptor is the disruptor
	disruptor map[string]disruptFunc

	disruptFunc func() bool
)

func (d *disruptor) disrupt(key string) bool {
	f, exist := (*d)[key]
	if !exist {
		return false
	}
	return f()
}

func newDisrupter() *disruptor {
	d := make(disruptor)
	return &d
}

func (d *disruptor) register(key string, f disruptFunc) *disruptor {
	(*d)[key] = f
	return d
}
