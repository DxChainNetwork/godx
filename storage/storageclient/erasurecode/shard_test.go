// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package erasurecode

import (
	"bytes"
	"errors"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"
)

func TestNewShardErasureCode(t *testing.T) {
	tests := []struct {
		minSectors uint32
		numSectors uint32
		shardSize  int
		err        error
	}{
		{1, 2, EncodedShardUnit, nil},
		{10, 20, EncodedShardUnit * 1024, nil},
		{1, 1, EncodedShardUnit, errors.New("params error")},
		{10, 20, 20, errors.New("shard size error")},
	}
	for i, test := range tests {
		_, err := newShardErasureCode(test.minSectors, test.numSectors, test.shardSize)
		if (err == nil) != (test.err == nil) {
			t.Errorf("Test %d: expect error: %v, have error %v", i, test.err, err)
		}
	}
}

func TestShardErasureCode_Encode_Recover(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	tests := []struct {
		minSectors uint32
		numSectors uint32
		shardSize  int
		data       []byte
	}{
		{1, 2, EncodedShardUnit, randomBytes(1)},
		{1, 10, EncodedShardUnit, randomBytes(10)},
		{10, 11, EncodedShardUnit, randomBytes(10)},
		{2, 3, EncodedShardUnit * 2, randomBytes(400)},
		{10, 30, EncodedShardUnit * 16, randomBytes(4096)},
		{10, 30, EncodedShardUnit * 16, randomBytes(1892378)},
	}
	for i, test := range tests {
		sec, err := newShardErasureCode(test.minSectors, test.numSectors, test.shardSize)
		if err != nil {
			t.Fatalf("Test %d: cannot new sec: %v", i, err)
		}
		encoded, err := sec.Encode(test.data)
		if err != nil {
			t.Fatalf("Test %d: cannot encode: %v", i, err)
		}
		// remove some of the encoded string
		removeIndex := rand.Perm(int(test.numSectors))[:test.numSectors-test.minSectors]
		for _, j := range removeIndex {
			encoded[j] = nil
		}
		recovered := new(bytes.Buffer)
		err = sec.Recover(encoded, len(test.data), recovered)
		if err != nil {
			t.Errorf("cannot recover data: %v", err)
		}
		if !bytes.Equal(recovered.Bytes()[:len(test.data)], test.data) {
			t.Errorf("Test %d: data not equal:\n\tExpect %x\n\tGot %x", i, test.data, recovered)
		}
	}
}

func BenchmarkShardErasureCode_Encode(b *testing.B) {
	rsc, err := newShardErasureCode(80, 100, 64)
	if err != nil {
		b.Fatal(err)
	}
	data := randomBytes(1 << 20)

	b.SetBytes(1 << 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rsc.Encode(data)
	}
}

func BenchmarkShardErasureCode_Recover(b *testing.B) {
	rsc, err := newShardErasureCode(50, 200, 64)
	if err != nil {
		b.Fatal(err)
	}
	data := randomBytes(1 << 20)
	pieces, err := rsc.Encode(data)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(1 << 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(pieces)/2; j += 2 {
			pieces[j] = nil
		}
		rsc.Recover(pieces, 1<<20, ioutil.Discard)
	}
}
