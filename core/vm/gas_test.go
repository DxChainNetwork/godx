// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"testing"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rlp"
)

func TestRemainGas(t *testing.T) {
	HostInfoTest := types.HostAnnouncement{
		NetAddress: "127.0.0.1:8080",
		Signature:  []byte("0101010101010"),
	}

	data, errEnc := rlp.EncodeToBytes(HostInfoTest)
	if errEnc != nil {
		t.Log("errEnc:", errEnc)
	}

	HostInfo := types.HostAnnouncement{}

	gasRemain, resultDecode := RemainGas(uint64(20000), rlp.DecodeBytes, data, &HostInfo)
	errDec, _ := resultDecode[0].(error)
	if errDec != nil {
		t.Log("errDec:", errDec)
	} else {
		t.Log("gasRemain:", gasRemain)
		t.Log("HostInfo:", HostInfo)
	}

}
