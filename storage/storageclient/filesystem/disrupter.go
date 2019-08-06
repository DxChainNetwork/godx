// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"errors"
	"sync"
)

// errDisrupted is the error that happens when disrupted
var errDisrupted = errors.New("disrupted")

// disrupter is the interface for disrupt
type disrupter interface {
	disrupt(s string) bool
	registerDisruptFunc(keyword string, df disruptFunc) disrupter
	registered(keyword string) bool
}

type (
	// standardDisrupter is the structure used for test cases which insert disrupt point
	// in the code. It has a mapping from keyword to the disrupt function.
	// Note the standardDisrupter does not support runtime multi-thread disruptFunc
	// registering
	standardDisrupter map[string]disruptFunc

	// disruptFunc is the function to be called when disrupt
	disruptFunc func() bool

	// counterDisrupter is the disrupter that disrupt also return the counts of the disrupter
	counterDisrupter struct {
		disrupter
		counter map[string]int
		lock    sync.Mutex
	}
)

// newStandardDisrupter creates an empty disrupter
func newStandardDisrupter() *standardDisrupter {
	d := make(standardDisrupter)
	return &d
}

// disrupt is the disrupt function to be executed during the code execution
func (d *standardDisrupter) disrupt(s string) bool {
	f, exist := (*d)[s]
	if !exist {
		return false
	}
	return f()
}

// registerDisruptFunc register the disrupt function to the standardDisrupter
func (d *standardDisrupter) registerDisruptFunc(keyword string, df disruptFunc) disrupter {
	(*d)[keyword] = df
	return d
}

// registered return whether the input keyword is registered
func (d *standardDisrupter) registered(s string) bool {
	_, exist := (*d)[s]
	return exist
}

// newCounterDisrupter makes a new CounterDisrupter
func newCounterDisrupter(sd disrupter) *counterDisrupter {
	return &counterDisrupter{
		disrupter: sd,
		counter:   make(map[string]int),
	}
}

// disrupt for counterDisrupter also increment the count of the string
func (cd *counterDisrupter) disrupt(s string) bool {
	cd.lock.Lock()
	defer cd.lock.Unlock()
	if cd.disrupter.registered(s) {
		c, exist := cd.counter[s]
		if !exist {
			cd.counter[s] = 1
		} else {
			cd.counter[s] = c + 1
		}
	}
	return cd.disrupter.disrupt(s)
}

// count return how many times a specified string has been accessed
func (cd *counterDisrupter) count(s string) int {
	cd.lock.Lock()
	defer cd.lock.Unlock()

	if !cd.disrupter.registered(s) {
		return 0
	}
	return cd.counter[s]
}

// makeBlockDisruptFunc creates a disruptFunc that will block on the input channel.
// After receiving the value from input channel, it will execute the second input func
func makeBlockDisruptFunc(c <-chan struct{}, f disruptFunc) disruptFunc {
	return func() bool {
		<-c
		return f()
	}
}
