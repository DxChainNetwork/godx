package storageclient

import (
	"context"
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/internal/ethapi"
)

// PublicTransactionPoolAPI exposes methods for the RPC interface
type StorageContractTxAPI struct {
	b         Backend
	nonceLock *ethapi.AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewStorageContractTxAPI(b Backend) *StorageContractTxAPI {
	nonceLock := new(ethapi.AddrLocker)
	return &StorageContractTxAPI{b, nonceLock}
}

// TODO: 上层storage contract交易调用这个接口发送交易，只需要传送 from、to、input（rlp编码过的）
func (sc *StorageContractTxAPI) SendFormContractTX(ctx context.Context, args SendStorageContractTxArgs) (common.Hash, error) {
	account := accounts.Account{Address: args.From}
	wallet, err := sc.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}

	sc.nonceLock.LockAddr(args.From)
	defer sc.nonceLock.UnlockAddr(args.From)

	tx, err := args.setDefaultsTX(ctx, sc.b)
	if err != nil {
		return common.Hash{}, err
	}

	var chainID *big.Int
	if config := sc.b.ChainConfig(); config.IsEIP155(sc.b.CurrentBlock().Number()) {
		chainID = config.ChainID
	}

	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}

	if err := sc.b.SendTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}

	return signed.Hash(), nil
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendStorageContractTxArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	Input    *hexutil.Bytes  `json:"input"`
}

func (args *SendStorageContractTxArgs) setDefaultsTX(ctx context.Context, b Backend) (*types.Transaction, error) {
	args.Gas = new(hexutil.Uint64)
	*(*uint64)(args.Gas) = 90000

	price, err := b.SuggestPrice(ctx)
	if err != nil {
		return nil, err
	}
	args.GasPrice = (*hexutil.Big)(price)

	nonce, err := b.GetPoolNonce(ctx, args.From)
	if err != nil {
		return nil, err
	}
	args.Nonce = (*hexutil.Uint64)(&nonce)

	if args.To == nil || args.Input == nil {
		return nil, errors.New(`storage contract tx without to or input`)
	}

	return types.NewTransaction(uint64(*args.Nonce), *args.To, nil, uint64(*args.Gas), (*big.Int)(args.GasPrice), *args.Input), nil
}
