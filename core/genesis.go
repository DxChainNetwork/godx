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

package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/rawdb"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
)

//go:generate gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go
//go:generate gencodec -type GenesisAccount -field-override genesisAccountMarshaling -out gen_genesis_account.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

// Genesis specifies the header fields, state of a genesis block. It also defines hard
// fork switch-over blocks through the chain configuration.
type Genesis struct {
	Config     *params.ChainConfig `json:"config"`
	Nonce      uint64              `json:"nonce"`
	Timestamp  uint64              `json:"timestamp"`
	ExtraData  []byte              `json:"extraData"`
	GasLimit   uint64              `json:"gasLimit"   gencodec:"required"`
	Difficulty *big.Int            `json:"difficulty" gencodec:"required"`
	Mixhash    common.Hash         `json:"mixHash"`
	Coinbase   common.Address      `json:"coinbase"`
	Alloc      GenesisAlloc        `json:"alloc"      gencodec:"required"`

	// These fields are used for consensus tests. Please don't use them
	// in actual genesis blocks.
	Number     uint64      `json:"number"`
	GasUsed    uint64      `json:"gasUsed"`
	ParentHash common.Hash `json:"parentHash"`
}

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc map[common.Address]GenesisAccount

func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := make(map[common.UnprefixedAddress]GenesisAccount)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ga = make(GenesisAlloc)
	for addr, a := range m {
		(*ga)[common.Address(addr)] = a
	}
	return nil
}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

// field type overrides for gencodec
type genesisSpecMarshaling struct {
	Nonce      math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
	ExtraData  hexutil.Bytes
	GasLimit   math.HexOrDecimal64
	GasUsed    math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Difficulty *math.HexOrDecimal256
	Alloc      map[common.UnprefixedAddress]GenesisAccount
}

type genesisAccountMarshaling struct {
	Code       hexutil.Bytes
	Balance    *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	Storage    map[storageJSON]storageJSON
	PrivateKey hexutil.Bytes
}

// storageJSON represents a 256 bit byte array, but allows less than 256 bits when
// unmarshaling from hex.
type storageJSON common.Hash

func (h *storageJSON) UnmarshalText(text []byte) error {
	text = bytes.TrimPrefix(text, []byte("0x"))
	if len(text) > 64 {
		return fmt.Errorf("too many hex characters in storage key/value %q", text)
	}
	offset := len(h) - len(text)/2 // pad on the left
	if _, err := hex.Decode(h[offset:], text); err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid hex storage key/value %q", text)
	}
	return nil
}

func (h storageJSON) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The stored chain configuration will be updated if it is compatible (i.e. does not
// specify a fork block below the local head block). In case of a conflict, the
// error is a *params.ConfigCompatError and the new, unwritten config is returned.
//
// The returned chain configuration is never nil.
func SetupGenesisBlock(db ethdb.Database, genesis *Genesis) (*params.ChainConfig, common.Hash, error) {
	return SetupGenesisBlockWithOverride(db, genesis, nil)
}
func SetupGenesisBlockWithOverride(db ethdb.Database, genesis *Genesis, constantinopleOverride *big.Int) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return params.DposChainConfig, common.Hash{}, errGenesisNoConfig
	}

	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		return genesis.Config, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		hash := genesis.ToBlock(nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}

	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	if constantinopleOverride != nil {
		newcfg.ConstantinopleBlock = constantinopleOverride
	}
	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != params.MainnetGenesisHash {
		return storedcfg, stored, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := rawdb.ReadHeaderNumber(db, rawdb.ReadHeadHeaderHash(db))
	if height == nil {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, *height)
	if compatErr != nil && *height != 0 && compatErr.RewindTo != 0 {
		return newcfg, stored, compatErr
	}
	rawdb.WriteChainConfig(db, stored, newcfg)
	return newcfg, stored, nil
}

func (g *Genesis) configOrDefault(ghash common.Hash) *params.ChainConfig {
	switch {
	case g != nil:
		return g.Config
	default:
		return params.DposChainConfig
	}
}

// ToBlock creates the genesis block and writes state of a genesis specification
// to the given database (or discards it if nil).
func (g *Genesis) ToBlock(db ethdb.Database) *types.Block {
	if db == nil {
		db = ethdb.NewMemDatabase()
	}
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}

	// init genesis block dpos context
	dposContext, err := initGenesisDposContext(statedb, g, db)
	if err != nil {
		panic(err)
	}

	root := statedb.IntermediateRoot(false)
	dcProto := dposContext.ToRoot()
	head := &types.Header{
		Number:      new(big.Int).SetUint64(g.Number),
		Nonce:       types.EncodeNonce(g.Nonce),
		Time:        new(big.Int).SetUint64(g.Timestamp),
		ParentHash:  g.ParentHash,
		Extra:       g.ExtraData,
		GasLimit:    g.GasLimit,
		GasUsed:     g.GasUsed,
		Difficulty:  g.Difficulty,
		MixDigest:   g.Mixhash,
		Coinbase:    g.Coinbase,
		Root:        root,
		DposContext: dcProto,
	}
	if g.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}
	_, err = statedb.Commit(false)
	if err != nil {
		panic(err)
	}
	if _, err = dposContext.Commit(); err != nil {
		panic(err)
	}

	err = statedb.Database().TrieDB().Commit(root, true)
	if err != nil {
		panic(err)
	}

	block := types.NewBlock(head, nil, nil, nil)
	block.SetDposCtx(dposContext)

	return block
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) Commit(db ethdb.Database) (*types.Block, error) {
	block := g.ToBlock(db)
	if block.Number().Sign() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with number > 0")
	}

	rawdb.WriteTd(db, block.Hash(), block.NumberU64(), g.Difficulty)
	rawdb.WriteBlock(db, block)
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())

	config := g.Config
	if config == nil {
		config = params.DposChainConfig
	}
	rawdb.WriteChainConfig(db, block.Hash(), config)
	return block, nil
}

// MustCommit writes the genesis block and state to db, panicking on error.
// The block is committed as the canonical head block.
func (g *Genesis) MustCommit(db ethdb.Database) *types.Block {
	block, err := g.Commit(db)
	if err != nil {
		panic(err)
	}
	return block
}

// GenesisBlockForTesting creates and writes a block in which addr has the given wei balance.
func GenesisBlockForTesting(db ethdb.Database, addr common.Address, balance *big.Int) *types.Block {
	g := DefaultGenesisBlock()
	g.Alloc[addr] = GenesisAccount{Balance: balance}
	return g.MustCommit(db)
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func DefaultGenesisBlock() *Genesis {
	g := &Genesis{
		Config:     params.DposChainConfig,
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0x4478436861696e204e6574776f726b20546573746e657420302e372e32"),
		GasLimit:   3141592,
		Difficulty: big.NewInt(1048576),
		Alloc: map[common.Address]GenesisAccount{
			// faucet
			common.HexToAddress("0x5747595e5fe0ff31df1c4feb7ae6110a34f4714a"): {Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 192), big.NewInt(9))},

			// validators
			common.HexToAddress("0xccdfb5a54db1d805ca24a33b8b15f49d8945bb4b"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x116204ed3e5749ab6c1318314300dbabf5aa972b"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xa60e0361cffc636da87ea8cb246e7926870621c9"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xf6a0da1f0b9a8d4e6b4392fe60a5e1f99c6aa873"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x08edc1328ba5236b151d273f7ad4703c1585def1"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x11720ac932723d5df4221dad1420ea6695acc68a"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x4a83c1c000ac1d1a8c06e8d18dc641c8530e0625"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x9374268a703851b2302b35d4de62c2f498099514"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x7d038901709e9d8cdb040e52e54c493bc726a05d"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x944eb166879a0b9a5e3c7a0512836a4d22a0bb47"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xf7925a26ebf873cea35fbe2f8278a8cc94fad801"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x31423da09afc3202844131da5888f9acd5593c6f"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x8fd7c0503e27b35ee6ef79c21559b4195536b780"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x9c0d5b2713ebebfa9ec0819186809cbd510554e8"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x18cead672df01dbd808fcc7e0e988bdc67551de5"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x2f03b5b4416e7ce86773f1908939291787cb8086"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xe61aa6815e667f1dff7d257ad2c1f30d02bcf3da"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xdfb1cca8d7299b0210086745dc7dad14a03ce2d3"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xc8c85cae16d18076741e2f94528d6ffaa5960fa1"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0xbbb1244fd311481e68253f9995a02715ac22ece6"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
			common.HexToAddress("0x816f4378aca62e72d75ead38495bb896f72eefce"): {Balance: common.NewBigIntUint64(1e18).MultInt64(20000).BigIntPtr()},
		},
	}

	return g
}

// DefaultTestnetGenesisBlock returns the Ropsten network genesis block.
func DefaultTestnetGenesisBlock() *Genesis {
	g := &Genesis{
		Config:     params.TestnetChainConfig,
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0x3535353535353535353535353535353535353535353535353535353535353535"),
		GasLimit:   16777216,
		Difficulty: big.NewInt(1048576),
		Alloc:      decodePrealloc(testnetAllocData),
	}
	for _, vc := range params.DefaultValidators {
		g.Alloc[vc.Address] = GenesisAccount{
			Balance: vc.Deposit.BigIntPtr(),
		}
	}
	return g
}

// DefaultRinkebyGenesisBlock returns the Rinkeby network genesis block.
func DefaultRinkebyGenesisBlock() *Genesis {
	g := &Genesis{
		Config:     params.RinkebyChainConfig,
		Timestamp:  1492009146,
		ExtraData:  hexutil.MustDecode("0x52657370656374206d7920617574686f7269746168207e452e436172746d616e42eb768f2244c8811c63729a21a3569731535f067ffc57839b00206d1ad20c69a1981b489f772031b279182d99e65703f0076e4812653aab85fca0f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		GasLimit:   4700000,
		Difficulty: big.NewInt(1),
		Alloc:      decodePrealloc(rinkebyAllocData),
	}
	for _, vc := range params.DefaultValidators {
		g.Alloc[vc.Address] = GenesisAccount{
			Balance: vc.Deposit.BigIntPtr(),
		}
	}
	return g
}

// DeveloperGenesisBlock returns the 'geth --dev' genesis block. Note, this must
// be seeded with the
func DeveloperGenesisBlock(period uint64, faucet common.Address) *Genesis {
	// Override the default period to the user requested one
	config := *params.AllCliqueProtocolChanges
	config.Clique.Period = period

	// Assemble and return the genesis with the precompiles and faucet pre-funded
	return &Genesis{
		Config:     &config,
		ExtraData:  append(append(make([]byte, 32), faucet[:]...), make([]byte, 65)...),
		GasLimit:   6283185,
		Difficulty: big.NewInt(1),
		Alloc: map[common.Address]GenesisAccount{
			common.BytesToAddress([]byte{1}): {Balance: big.NewInt(1)}, // ECRecover
			common.BytesToAddress([]byte{2}): {Balance: big.NewInt(1)}, // SHA256
			common.BytesToAddress([]byte{3}): {Balance: big.NewInt(1)}, // RIPEMD
			common.BytesToAddress([]byte{4}): {Balance: big.NewInt(1)}, // Identity
			common.BytesToAddress([]byte{5}): {Balance: big.NewInt(1)}, // ModExp
			common.BytesToAddress([]byte{6}): {Balance: big.NewInt(1)}, // ECAdd
			common.BytesToAddress([]byte{7}): {Balance: big.NewInt(1)}, // ECScalarMul
			common.BytesToAddress([]byte{8}): {Balance: big.NewInt(1)}, // ECPairing
			faucet:                           {Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(9))},
		},
	}
}

func decodePrealloc(data string) GenesisAlloc {
	var p []struct{ Addr, Balance *big.Int }
	if err := rlp.NewStream(strings.NewReader(data), 0).Decode(&p); err != nil {
		panic(err)
	}
	ga := make(GenesisAlloc, len(p))
	for _, account := range p {
		ga[common.BigToAddress(account.Addr)] = GenesisAccount{Balance: account.Balance}
	}
	return ga
}

// initGenesisDposContext returns the dpos context of given genesis block
func initGenesisDposContext(stateDB *state.StateDB, g *Genesis, db ethdb.Database) (*types.DposContext, error) {
	dc, err := types.NewDposContextFromProto(types.NewFullDposDatabase(db), &types.DposContextRoot{})
	if err != nil {
		return nil, err
	}

	if g.Config == nil || g.Config.Dpos == nil || g.Config.Dpos.Validators == nil {
		return nil, errors.New("invalid dpos config for genesis")
	}

	// get validators from the genesis DPOS config
	validators := g.Config.Dpos.ParseValidators()

	// set initial genesis epoch validators
	err = dc.SetValidators(validators)
	if err != nil {
		return nil, err
	}

	// just let genesis initial validator voted themselves
	// vMap is the structure to check the duplication of the validators
	vMap := make(map[common.Address]struct{})
	for _, validator := range g.Config.Dpos.Validators {
		validatorAddr := validator.Address
		if _, exist := vMap[validatorAddr]; exist {
			return nil, fmt.Errorf("duplicate validator address %x", validatorAddr)
		}
		vMap[validatorAddr] = struct{}{}

		err = dc.CandidateTrie().TryUpdate(validatorAddr.Bytes(), validatorAddr.Bytes())
		if err != nil {
			return nil, err
		}

		// set deposit and frozen assets
		if balance := common.PtrBigInt(stateDB.GetBalance(validatorAddr)); balance.Cmp(validator.Deposit) < 0 {
			return nil, fmt.Errorf("during initializing for genesis, validator %x has not enough balance for deposit", validatorAddr)
		}
		dpos.SetCandidateDeposit(stateDB, validatorAddr, validator.Deposit)
		dpos.SetFrozenAssets(stateDB, validatorAddr, validator.Deposit)

		// set reward ratio
		dpos.SetRewardRatioNumerator(stateDB, validatorAddr, validator.RewardRatio)
		dpos.SetRewardRatioNumeratorLastEpoch(stateDB, validatorAddr, validator.RewardRatio)
	}

	// init the KeyValueCommonAddress account and set its nonce 1 to avoid deleting empty state object
	stateDB.SetNonce(dpos.KeyValueCommonAddress, 1)
	stateDB.SetState(dpos.KeyValueCommonAddress, dpos.KeyPreEpochSnapshotDelegateTrieRoot, dc.DelegateTrie().Hash())

	return dc, nil
}
