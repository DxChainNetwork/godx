// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

type (
	// disrupter is the disrupter
	disrupter map[string]disruptFunc

	disruptFunc func() bool
)

func (d *disrupter) disrupt(key string) bool {
	f, exist := (*d)[key]
	if !exist {
		return false
	}
	return f()
}

func newDisrupter() *disrupter {
	d := make(disrupter)
	return &d
}

func (d *disrupter) register(key string, f disruptFunc) *disrupter {
	(*d)[key] = f
	return d
}
