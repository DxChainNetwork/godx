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
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/rlp"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

// String stringify the code
func (c Code) String() string {
	return string(c) //strings.Join(Disassemble(c), " ")
}

// Storage is the content to be stored as account.Root
type Storage map[common.Hash]common.Hash

func (self Storage) String() (str string) {
	for key, value := range self {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (self Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range self {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	originStorage Storage // Storage cache of original entries to dedup rewrites
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
}

// empty returns whether the account is considered empty.
// so.data's fields three of four are all zero meaning the so is empty.
// The three fields include: nonce, balance, and codeHash
func (so *stateObject) empty() bool {
	return so.data.Nonce == 0 && so.data.Balance.Sign() == 0 && bytes.Equal(so.data.CodeHash, emptyCodeHash)
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
// Account can be two types:
// 1. Externally owned accounts (user accounts is controlled by private keys)
// 2. Contracts accounts (controlled by code)
type Account struct {
	Nonce uint64 // If external account, number of transactions sent from this account address.
	// nonce is used to prevent replay attack. If contract account, number of contracts created by this account
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie, hash of storage contents. Empty by default
	CodeHash []byte      // Hash of EVM code of this account.
}

// newObject creates a state object.
// The input parameters include address and data (data.Nonce, data.Balance, data.Root, data.CodeHash).
// so.addrHash is the hash of address.
func newObject(db *StateDB, address common.Address, data Account) *stateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		originStorage: make(Storage),
		dirtyStorage:  make(Storage),
	}
}

// EncodeRLP implements rlp.Encoder.
// Only so.data is rlp encoded and stored.
func (so *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, so.data)
}

// setError remembers the first non-nil error it is called with.
// Only update so.dbError if it is nil
func (so *stateObject) setError(err error) {
	if so.dbErr == nil {
		so.dbErr = err
	}
}

// markSuicided change so.suicided to true.
func (so *stateObject) markSuicided() {
	so.suicided = true
}

// touch append the touchChange to so.db.journal.
// And if address is ripemd, mark the address as dirty in journal.
func (so *stateObject) touch() {
	so.db.journal.append(touchChange{
		account: &so.address,
	})
	if so.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		so.db.journal.dirty(so.address)
	}
}

// getTrie get the storageTrie of the object.
// If so.trie has been previously loaded, directly return so.trie.
// Open storage trie and assign it to so.trie.
// If cannot find a the storage trie, create a new trie.
func (so *stateObject) getTrie(db Database) Trie {
	if so.trie == nil {
		var err error
		so.trie, err = db.OpenStorageTrie(so.addrHash, so.data.Root)
		if err != nil {
			// if db does not have so.data.root, return an empty new secure trie
			so.trie, _ = db.OpenStorageTrie(so.addrHash, common.Hash{})
			so.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return so.trie
}

// GetState retrieves a value from the account storage trie.
// If key in so.dirtyStorage, return it from so.dirtyStorage
// Else return committedState from originStorage or from db.
func (so *stateObject) GetState(db Database, key common.Hash) common.Hash {
	// If we have a dirty value for this state entry, return it
	value, dirty := so.dirtyStorage[key]
	if dirty {
		return value
	}
	// Otherwise return the entry's original value
	return so.GetCommittedState(db, key)
}

// GetCommittedState retrieves a value from the committed account storage trie.
// If key is cached in so.originStorage, return immediately.
// Else open the trie of root so.data.root, return the value to that key and update original storage.
func (so *stateObject) GetCommittedState(db Database, key common.Hash) common.Hash {
	// If we have the original value cached, return that value
	value, cached := so.originStorage[key]
	if cached {
		return value
	}
	// Otherwise load the storage from the database and query for key
	enc, err := so.getTrie(db).TryGet(key[:])
	if err != nil {
		so.setError(err)
		return common.Hash{}
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			so.setError(err)
		}
		value.SetBytes(content)
	}
	so.originStorage[key] = value
	return value
}

// SetState updates a value in account storage.
// 1. Get the storage of key from the object.
// 2. If update value does not change, simply return
// 3. Else add the record to db.journey.
// 4. Set the storage state.
// Note the so.data is not changed. The only change field is so.dirtyStorage.
func (so *stateObject) SetState(db Database, key, value common.Hash) {
	// If the new value is the same as old, don't set
	prev := so.GetState(db, key)
	if prev == value {
		return
	}
	// New value is different, update and journal the change
	so.db.journal.append(storageChange{
		account:  &so.address,
		key:      key,
		prevalue: prev,
	})
	so.setState(key, value)
}

// Set key, value pair in so.dirtyStorage.
// Note this does not change so.data.Root as well as so.Trie, since it has not been
// instructed to be written to database.
func (so *stateObject) setState(key, value common.Hash) {
	so.dirtyStorage[key] = value
}

// updateTrie writes cached storage modifications into the object's storage trie.
// a.k.a, write so.dirtyStorage to so.trie and update so.originStorage
// If the updated value is 0, delete the key in so.trie. Else so.trie is updated.
// Finally return the updated trie.
func (so *stateObject) updateTrie(db Database) Trie {
	tr := so.getTrie(db)
	for key, value := range so.dirtyStorage {
		delete(so.dirtyStorage, key)

		// Skip noop changes, persist actual changes
		if value == so.originStorage[key] {
			continue
		}
		so.originStorage[key] = value

		if (value == common.Hash{}) {
			so.setError(tr.TryDelete(key[:]))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		// TrimLeft remove all leading zeros in value
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		so.setError(tr.TryUpdate(key[:], v))
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
// updateRoot will update the so.trie, and write the hash of updated trie to so.data.Root
func (so *stateObject) updateRoot(db Database) {
	so.updateTrie(db)
	so.data.Root = so.trie.Hash()
}

// CommitTrie the storage trie of the object to db.
// This updates the trie root.
// 1. Write so.dirtyStorage to so.trie and so.originStorage
// 2. Commit so.trie
// 3. Update so.data.Root
func (so *stateObject) CommitTrie(db Database) error {
	so.updateTrie(db)
	if so.dbErr != nil {
		return so.dbErr
	}
	root, err := so.trie.Commit(nil)
	if err == nil {
		so.data.Root = root
	}
	return err
}

// AddBalance removes amount from so's balance.
// It is used to add funds to the destination account of a transfer.
func (so *stateObject) AddBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if so.empty() {
			so.touch()
		}

		return
	}
	so.SetBalance(new(big.Int).Add(so.Balance(), amount))
}

// SubBalance removes amount from so's balance.
// It is used to remove funds from the origin account of a transfer.
func (so *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	so.SetBalance(new(big.Int).Sub(so.Balance(), amount))
}

// SetBalance set so.data.Balance to corresponding amount.
// The action will also be recorded in db.journal
func (so *stateObject) SetBalance(amount *big.Int) {
	so.db.journal.append(balanceChange{
		account: &so.address,
		prev:    new(big.Int).Set(so.data.Balance),
	})
	so.setBalance(amount)
}

// setBalance set so.data.Balance to amount specified.
func (so *stateObject) setBalance(amount *big.Int) {
	so.data.Balance = amount
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
// This function does nothing
func (so *stateObject) ReturnGas(gas *big.Int) {}

// deepCopy deep copies a stateObject instance
// Except for dbError, every field is copied.
func (so *stateObject) deepCopy(db *StateDB) *stateObject {
	stateObject := newObject(db, so.address, so.data)
	if so.trie != nil {
		stateObject.trie = db.db.CopyTrie(so.trie)
	}
	stateObject.code = so.code
	stateObject.dirtyStorage = so.dirtyStorage.Copy()
	stateObject.originStorage = so.originStorage.Copy()
	stateObject.suicided = so.suicided
	stateObject.dirtyCode = so.dirtyCode
	stateObject.deleted = so.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (so *stateObject) Address() common.Address {
	return so.address
}

// Code returns the contract code associated with this object, if any.
// If so.code exist, directly return
// If so does not have any code (so.data.CodeHash == emptyCodeHash), return nil
// If so.code is not loaded (by examining so.data.CodeHash), code can be loaded from db. Write it to so.code
func (so *stateObject) Code(db Database) []byte {
	if so.code != nil {
		return so.code
	}
	if bytes.Equal(so.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.ContractCode(so.addrHash, common.BytesToHash(so.CodeHash()))
	if err != nil {
		so.setError(fmt.Errorf("can't load code hash %x: %v", so.CodeHash(), err))
	}
	so.code = code
	return code
}

// SetCode add the code chnge to journal and set the corresponding code fields, including code and codeHash.
func (so *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := so.Code(so.db.db)
	so.db.journal.append(codeChange{
		account:  &so.address,
		prevhash: so.CodeHash(),
		prevcode: prevcode,
	})
	so.setCode(codeHash, code)
}

// setCode set so.code, so.data.CodeHash, set so.dirtyCode to true
func (so *stateObject) setCode(codeHash common.Hash, code []byte) {
	so.code = code
	so.data.CodeHash = codeHash[:]
	so.dirtyCode = true
}

// SetNonce set nonce and add to db.journal
func (so *stateObject) SetNonce(nonce uint64) {
	so.db.journal.append(nonceChange{
		account: &so.address,
		prev:    so.data.Nonce,
	})
	so.setNonce(nonce)
}

// setNonce is setter for so.data.nonce
func (so *stateObject) setNonce(nonce uint64) {
	so.data.Nonce = nonce
}

// CodeHash is Attribute field. return so.data.CodeHash
func (so *stateObject) CodeHash() []byte {
	return so.data.CodeHash
}

// Balance is attribute field. Return so.data.Balance
func (so *stateObject) Balance() *big.Int {
	return so.data.Balance
}

// Nonce is attribute field. Return so.data.Nonce
func (so *stateObject) Nonce() uint64 {
	return so.data.Nonce
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (so *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}
