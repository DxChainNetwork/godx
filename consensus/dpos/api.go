// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/trie"
)

// API is a user facing RPC API to allow controlling the delegate and voting
// mechanisms of the delegated-proof-of-stake
type API struct {
	chain consensus.ChainReader
	dpos  *Dpos
}

// GetValidators retrieves the list of the validators at specified block
func (api *API) GetValidators(number *rpc.BlockNumber) ([]common.Address, error) {
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}

	if header == nil {
		return nil, errUnknownBlock
	}

	dbWithCache := trie.NewDatabaseWithCache(api.dpos.db, 256)
	epochTrie, err := types.NewEpochTrie(header.DposContext.EpochRoot, dbWithCache)
	if err != nil {
		return nil, err
	}

	dposContext := types.DposContext{}
	dposContext.SetEpoch(epochTrie)
	validators, err := dposContext.GetValidators()
	if err != nil {
		return nil, err
	}

	return validators, nil
}

// GetConfirmedBlockNumber retrieves the latest irreversible block
func (api *API) GetConfirmedBlockNumber() (*big.Int, error) {
	var err error
	header := api.dpos.confirmedBlockHeader
	if header == nil {
		header, err = api.dpos.loadConfirmedBlockHeader(api.chain)
		if err != nil {
			return nil, err
		}
	}

	return header.Number, nil
}
