// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/pkg/errors"
	"math/rand"
	"time"
)

var errDisrupted = errors.New("disrupted")

type (
	// disrupter is the structure used for test cases which insert disrupt point
	// in the code. It has a mapping from keyword to the disrupt function
	disrupter map[string]disruptFunc

	// disruptFunc is the function to be called when disrupt
	disruptFunc func() bool
)

// disrupt is the disrupt function to be executed during the code execution
func (d disrupter) disrupt(s string) bool {
	f, exist := d[s]
	if !exist {
		return false
	}
	return f()
}

// newRandomDisrupter creates a disrupt that disrupt at keyword at a probability
// of disruptProb [0, 1]
func newRandomDisrupter(keyword string, disruptProb float32) disrupter {
	d := make(disrupter)
	d.registerDisruptFunc(keyword, makeRandomDisruptFunc(disruptProb))
	return d
}

// newNormalDisrupter creates a disrupt that always disrupt
func newNormalDisrupter(keyword string) disrupter {
	d := make(disrupter)
	d.registerDisruptFunc(keyword, makeNormalDisruptFunc())
	return d
}

// newBlockDisrupter creates a disrupt that blocks on input channel, and
// alway return true after unblock
func newBlockDisrupter(keyword string, c <-chan struct{}) disrupter {
	d := make(disrupter)
	d.registerDisruptFunc(keyword, makeBlockDisruptFunc(c, makeNormalDisruptFunc()))
	return d
}

// registerDisruptFunc register the disrupt function to the disrupter
func (d disrupter) registerDisruptFunc(keyword string, df disruptFunc) disrupter {
	d[keyword] = df
	return d
}

// makeRandomDisruptFunc makes a random disrupt function that will disrupt
// at the rate of disruptProb
func makeRandomDisruptFunc(disruptProb float32) disruptFunc {
	return func() bool {
		rand.Seed(time.Now().UnixNano())
		num := rand.Float32()
		if num < disruptProb {
			return true
		}
		return false
	}
}

// makeNormalDisruptFunc creates a disruptFunc that always return true
func makeNormalDisruptFunc() disruptFunc {
	return func() bool {
		return true
	}
}

// makeBlockDisruptFunc creates a disruptFunc that will block on the input channel.
// After receiving the value from input channel, it will execute the second input func
func makeBlockDisruptFunc(c <-chan struct{}, f disruptFunc) disruptFunc {
	return func() bool {
		<-c
		return f()
	}
}
