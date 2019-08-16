// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/rlp"
)

// NewFakeResponseMsg will fake the storage host response message
func NewFakeResponseMsg(msgCode uint64, data interface{}) (p2p.Msg, error) {
	size, r, err := rlp.EncodeToReader(data)
	if err != nil {
		return p2p.Msg{}, fmt.Errorf("failed to create fake p2p message: %s", err.Error())
	}

	return p2p.Msg{
		Code:    msgCode,
		Size:    uint32(size),
		Payload: r,
	}, nil
}
