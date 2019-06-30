// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"net"
	"testing"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
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

	_, resultDecode := RemainGas(uint64(20000), rlp.DecodeBytes, data, &HostInfo)
	errDec, _ := resultDecode[0].(error)
	if errDec != nil {
		t.Error("errDec:", errDec)
	}

}

func TestHostAnnounceRemainGas(t *testing.T) {
	prvKeyHost, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs for storage host: %v", err)
	}
	hostNode := enode.NewV4(&prvKeyHost.PublicKey, net.IP{127, 0, 0, 1}, int(8888), int(8888))

	if err != nil {
		t.Errorf("failed to generate public/private key pairs for storage client: %v", err)
	}

	// test host announce signature(only one signature)
	ha := types.HostAnnouncement{
		NetAddress: hostNode.String(),
	}

	sigHa, err := crypto.Sign(ha.RLPHash().Bytes(), prvKeyHost)
	if err != nil {
		t.Errorf("failed to sign host announce: %v", err)
	}
	ha.Signature = sigHa

	_, resultDecode := RemainGas(uint64(20000), CheckMultiSignatures, ha, [][]byte{ha.Signature})
	errDec, _ := resultDecode[0].(error)
	if errDec != nil {
		t.Error("errDec:", errDec)
	}
}
