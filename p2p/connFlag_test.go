// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"github.com/DxChainNetwork/godx/rlp"
	"testing"
)

type poc uint32
type Teste struct {
	M     uint64 // 5 by default
	Flags poc
}

func TestRlpAnyDataStructure(t *testing.T) {
	var a poc = 16
	hs := &Teste{
		M:     18,
		Flags: a,
	}

	size, r, err := rlp.EncodeToReader(hs)
	if err != nil {
		t.Fatal("rlp encode reader err", err)
	}

	res := &Teste{}
	s := rlp.NewStream(r, uint64(size))
	if err := s.Decode(res); err != nil {
		t.Fatal("decode handshake", err)
	}

	t.Log(res.Flags)
	t.Log(res.M)
}

func TestRlpHandshake(t *testing.T) {
	hs := &protoHandshake{
		Version: 64,
		Name:    "eth",
		Flags:   staticDialedConn,
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

	if res.Flags != hs.Flags {
		t.Fatal("this is not result we want")
	}
}
