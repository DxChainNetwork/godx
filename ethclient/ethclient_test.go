package ethclient

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	ethereum "github.com/DxChainNetwork/godx"
	"github.com/DxChainNetwork/godx/common"
)

// Verify that Client implements the ethereum interfaces.
var (
	_ = ethereum.ChainReader(&Client{})
	_ = ethereum.TransactionReader(&Client{})
	_ = ethereum.ChainStateReader(&Client{})
	_ = ethereum.ChainSyncReader(&Client{})
	_ = ethereum.ContractCaller(&Client{})
	_ = ethereum.GasEstimator(&Client{})
	_ = ethereum.GasPricer(&Client{})
	_ = ethereum.LogFilterer(&Client{})
	_ = ethereum.PendingStateReader(&Client{})
	// _ = ethereum.PendingStateEventer(&Client{})
	_ = ethereum.PendingContractCaller(&Client{})
)

func TestToFilterArg(t *testing.T) {
	blockHashErr := fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock")
	addresses := []common.Address{
		common.HexToAddress("0xD36722ADeC3EdCB29c8e7b5a47f352D701393462"),
	}
	blockHash := common.HexToHash(
		"0xeb94bb7d78b73657a9d7a99792413f50c0a45c51fc62bdcb08a53f18e9a2b4eb",
	)

	for _, testCase := range []struct {
		name   string
		input  ethereum.FilterQuery
		output interface{}
		err    error
	}{
		{
			"without BlockHash",
			ethereum.FilterQuery{
				Addresses: addresses,
				FromBlock: big.NewInt(1),
				ToBlock:   big.NewInt(2),
				Topics:    [][]common.Hash{},
			},
			map[string]interface{}{
				"address":   addresses,
				"fromBlock": "0x1",
				"toBlock":   "0x2",
				"topics":    [][]common.Hash{},
			},
			nil,
		},
		{
			"with nil fromBlock and nil toBlock",
			ethereum.FilterQuery{
				Addresses: addresses,
				Topics:    [][]common.Hash{},
			},
			map[string]interface{}{
				"address":   addresses,
				"fromBlock": "0x0",
				"toBlock":   "latest",
				"topics":    [][]common.Hash{},
			},
			nil,
		},
		{
			"with blockhash",
			ethereum.FilterQuery{
				Addresses: addresses,
				BlockHash: &blockHash,
				Topics:    [][]common.Hash{},
			},
			map[string]interface{}{
				"address":   addresses,
				"blockHash": blockHash,
				"topics":    [][]common.Hash{},
			},
			nil,
		},
		{
			"with blockhash and from block",
			ethereum.FilterQuery{
				Addresses: addresses,
				BlockHash: &blockHash,
				FromBlock: big.NewInt(1),
				Topics:    [][]common.Hash{},
			},
			nil,
			blockHashErr,
		},
		{
			"with blockhash and to block",
			ethereum.FilterQuery{
				Addresses: addresses,
				BlockHash: &blockHash,
				ToBlock:   big.NewInt(1),
				Topics:    [][]common.Hash{},
			},
			nil,
			blockHashErr,
		},
		{
			"with blockhash and both from / to block",
			ethereum.FilterQuery{
				Addresses: addresses,
				BlockHash: &blockHash,
				FromBlock: big.NewInt(1),
				ToBlock:   big.NewInt(2),
				Topics:    [][]common.Hash{},
			},
			nil,
			blockHashErr,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := toFilterArg(testCase.input)
			if (testCase.err == nil) != (err == nil) {
				t.Fatalf("expected error %v but got %v", testCase.err, err)
			}
			if testCase.err != nil {
				if testCase.err.Error() != err.Error() {
					t.Fatalf("expected error %v but got %v", testCase.err, err)
				}
			} else if !reflect.DeepEqual(testCase.output, output) {
				t.Fatalf("expected filter arg %v but got %v", testCase.output, output)
			}
		})
	}
}

// Note: Generally, Ethereum rpc service maybe not open to public ! please connect your ethereum node !

// example to use ethclient
func exampleToUseClient() {

	// create a client
	example_rpc_server := "http://localhost:8545" // Note: please set the rpc address of your ethereum node !
	example_client, err := Dial(example_rpc_server)
	if err != nil {
		fmt.Printf("\n failed to dial rpc server: %s ,err: %v \n", example_rpc_server, err)
	} else if example_client == nil {
		fmt.Printf("\n create a nil rpc client \n")
	}

	// Blockchain Access
	ctx := context.Background()
	blockhash := common.HexToHash("0x127187f6d5e6204935c9df22d86a65ab6038b863497c0417aaee424c9c1ef58f") // Note: please set the example block hash in your private blockchain !
	block, err := example_client.BlockByHash(ctx, blockhash)
	if err != nil {
		fmt.Printf("\n failed to get block by hash: %s ,err: %v \n", blockhash.Hex(), err)
	} else {
		fmt.Printf("\n succcess to get block: %v \n", block.Number().String())
	}

	// filter log query
	filterQuery := ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress("0x2376e58503058db30e2a91352470636a07cb8926"), // Note: please set the example user account address in your private blockchain !
		},
		BlockHash: &blockhash,
		Topics:    [][]common.Hash{},
	}
	logs, err := example_client.FilterLogs(ctx, filterQuery)
	if err != nil {
		fmt.Printf("\n failed to query logs by filter,err: %v \n", err)
	} else {
		fmt.Printf("\n succcess to query logs: %v \n", logs)
	}

	// call contract
	contractAddr := common.HexToAddress("0x02157a7e46d32c4df73b9f2dda7de4e89e26b22b") // Note: please set the example contract account address in your private blockchain !
	msg := ethereum.CallMsg{
		From: common.HexToAddress("0x2376e58503058db30e2a91352470636a07cb8926"), // Note: please set the example user account address in your private blockchain !
		To:   &contractAddr,
		Gas:  uint64(1000000),
		Data: []byte("0xcdcd77c000000000000000000000000000000000000000000000000000000000000000450000000000000000000000000000000000000000000000000000000000000001"), // Note: please set the example contract call args in your private blockchain !
	}
	result, err := example_client.CallContract(ctx, msg, nil)
	if err != nil {
		fmt.Printf("\n failed to call contract from latest block,err: %v \n", err)
	} else {
		fmt.Printf("\n succcess to call contract from latest block,result: %s \n", string(result))
	}

}
