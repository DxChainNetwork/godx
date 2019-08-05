// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package erasurecode

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"
)

func ExampleShardErasureCode() {
	rand.Seed(time.Now().UnixNano())
	minSectors := uint32(10)
	numSectors := uint32(30)
	ec, err := New(ECTypeStandard, minSectors, numSectors)
	if err != nil {
		fmt.Println(err)
	}
	data := []byte("Some thing I want to encode")
	encoded, err := ec.Encode(data)
	if err != nil {
		fmt.Println(err)
	}
	removeIndex := rand.Perm(int(numSectors))[:numSectors-minSectors]
	for _, j := range removeIndex {
		encoded[j] = nil
	}
	recovered := new(bytes.Buffer)
	err = ec.Recover(encoded, len(data), recovered)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("recovered data: %v\n", string(recovered.Bytes()))
	// Output:
	// recovered data: Some thing I want to encode
}
