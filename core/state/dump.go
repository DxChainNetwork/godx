// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"encoding/json"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"
)

type DumpAccount struct {
	Balance  string            `json:"balance"`
	Nonce    uint64            `json:"nonce"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`
}

type Dump struct {
	Root     string                 `json:"root"`
	Accounts map[string]DumpAccount `json:"accounts"`
}

// RawDump dumps data in all s.trie in format
// addr -> DumpAccount
// where dumpAccount.Storage = map(trie(account.data.root))
func (s *StateDB) RawDump() Dump {
	dump := Dump{
		Root:     fmt.Sprintf("%x", s.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
	}

	it := trie.NewIterator(s.trie.NodeIterator(nil))
	for it.Next() {
		addr := s.trie.GetKey(it.Key)
		var data Account
		if err := rlp.DecodeBytes(it.Value, &data); err != nil {
			panic(err)
		}
		// obj is the stateObject from address and data from trie
		obj := newObject(nil, common.BytesToAddress(addr), data)
		// Define the dump message
		account := DumpAccount{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     common.Bytes2Hex(data.Root[:]),
			CodeHash: common.Bytes2Hex(data.CodeHash),
			Code:     common.Bytes2Hex(obj.Code(s.db)),
			Storage:  make(map[string]string),
		}
		// Iterate on the trie of root obj.data.root. Write the key value pair to account.Storage
		storageIt := trie.NewIterator(obj.getTrie(s.db).NodeIterator(nil))
		for storageIt.Next() {
			account.Storage[common.Bytes2Hex(s.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(storageIt.Value)
		}
		// update dump
		dump.Accounts[common.Bytes2Hex(addr)] = account
	}
	return dump
}

// Dump call RawDump and marshal the result to json
func (s *StateDB) Dump() []byte {
	json, err := json.MarshalIndent(s.RawDump(), "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}

	return json
}
