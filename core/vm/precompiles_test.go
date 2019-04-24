package vm

import (
	"encoding/hex"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
)

var PrecompiledContractsTest = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{9}): &testContact{},
}

type testContact struct {
	address common.Address
	name    string
}

//调用该预留合约需要消耗的gas
func (c *testContact) RequiredGas(input []byte) uint64 {
	return 20000
}

//预留合约的具体实现函数
func (c *testContact) Run(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("Invalid parameter")
	}
	return input[:len(input)/2], nil
}

func ExamplePrecompiles() {

	precompiledContract := testContact{
		address: common.HexToAddress("9"),
		name:    "test",
	}

	contract := &Contract{
		Gas: 500000,
	}

	input, _ := hex.DecodeString("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2e")

	if p := PrecompiledContractsTest[precompiledContract.address]; p != nil {
		ret, err := RunPrecompiledContract(p, input, contract)
		if err != nil {
			fmt.Println("error")
		} else {
			fmt.Println(hex.EncodeToString(ret))
		}

	}

	// Output:
	//ffffffffffffffffffffffffffffffff
}
