// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"math/big"

	"github.com/DxChainNetwork/godx/consensus"
	"github.com/syndtr/goleveldb/leveldb"
)

// API is a user facing RPC API to allow controlling the delegate and voting
// mechanisms of the delegated-proof-of-stake
type API struct {
	chain consensus.ChainReader
	dpos  *Dpos
}

// GetConfirmedBlockNumber retrieves the latest irreversible block
func (api *API) GetConfirmedBlockNumber() (*big.Int, error) {
	var err error
	header := api.dpos.confirmedBlockHeader
	if header == nil {
		header, err = api.dpos.loadConfirmedBlockHeader(api.chain)
		if err != nil {

			// if it's leveldb.ErrNotFound, indicates that only genesis block in local, and return 0
			if err == leveldb.ErrNotFound {
				return new(big.Int).SetInt64(0), nil
			}

			// other errors, return nil
			return nil, err
		}
	}

	return header.Number, nil
}
