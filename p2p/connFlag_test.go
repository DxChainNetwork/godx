// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"github.com/DxChainNetwork/godx/rlp"
	"testing"
)

func TestRlpHandshake(t *testing.T) {
	hs := &protoHandshake{
		Version:64,
		Name:"eth",
		flags:staticDialedConn,
	}
	size, r, err := rlp.EncodeToReader(hs)
	if err != nil {
		t.Fatal("rlp encode reader err", err)
	}

	res := &protoHandshake{}
	s := rlp.NewStream(r, uint64(size))
	if err := s.Decode(res); err != nil {
		t.Fatal("decode handshake", err)
	}

	if res.flags == hs.flags {
		t.Fatal("this is result we want")
	}
}