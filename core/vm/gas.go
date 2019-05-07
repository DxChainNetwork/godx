// Copyright 2015 The go-ethereum Authors
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

package vm

import (
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/ethdb"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/params"
)

// Gas costs
const (
	GasQuickStep   uint64 = 2
	GasFastestStep uint64 = 3
	GasFastStep    uint64 = 5
	GasMidStep     uint64 = 8
	GasSlowStep    uint64 = 10
	GasExtStep     uint64 = 20

	GasReturn       uint64 = 0
	GasStop         uint64 = 0
	GasContractByte uint64 = 200
)

var (
	GasCalculationParamsNumberWorng = errors.New("The parameter was wrong.")
	GasCalculationinsufficient      = errors.New("This gas is insufficient")
)

// calcGas returns the actual gas cost of the call.
//
// The cost of gas was changed during the homestead price change HF. To allow for EIP150
// to be implemented. The returned gas is gas - base * 63 / 64.
func callGas(gasTable params.GasTable, availableGas, base uint64, callCost *big.Int) (uint64, error) {
	if gasTable.CreateBySuicide > 0 {
		availableGas = availableGas - base
		gas := availableGas - availableGas/64
		// If the bit length exceeds 64 bit we know that the newly calculated "gas" for EIP150
		// is smaller than the requested amount. Therefor we return the new gas instead
		// of returning an error.
		if callCost.BitLen() > 64 || gas < callCost.Uint64() {
			return gas, nil
		}
	}
	if callCost.BitLen() > 64 {
		return 0, errGasUintOverflow
	}

	return callCost.Uint64(), nil
}

// calculate the gas of storage contract execution
func RemainGas(args ...interface{}) (uint64, []interface{}) {
	result := make([]interface{}, 0)
	gas, ok := args[0].(uint64)
	if len(args) < 2 || !ok {
		result = append(result, GasCalculationParamsNumberWorng)
		return gas, result
	}

	switch i := args[1].(type) {

	// rlp.DecodeBytes
	case func([]byte, interface{}) error:
		if gas < params.DecodeGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 4 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		paramsPre, ok := args[2].([]byte)
		if !ok {
			return gas, result
		}
		gas -= params.DecodeGas
		err := i(paramsPre, args[3])
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result

		//CheckFormContract
	case func(*EVM, types.StorageContract, types.BlockHeight) error:
		if gas < params.CheckFileGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 5 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		evm, _ := args[2].(*EVM)
		fc, _ := args[3].(types.StorageContract)
		bl, _ := args[4].(types.BlockHeight)
		gas -= params.CheckFileGas
		err := i(evm, fc, bl)
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result

		//CheckReversionContract
	case func(*EVM, types.StorageContractRevision, types.BlockHeight) error:
		if gas < params.CheckFileGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 5 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		evm, _ := args[2].(*EVM)
		scr, _ := args[3].(types.StorageContractRevision)
		bl, _ := args[4].(types.BlockHeight)
		gas -= params.CheckFileGas
		err := i(evm, scr, bl)
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result

		//CheckStorageProof
	case func(*EVM, types.StorageProof, types.BlockHeight) error:
		if gas < params.CheckFileGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 5 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		evm, _ := args[2].(*EVM)
		sp, _ := args[3].(types.StorageProof)
		bl, _ := args[4].(types.BlockHeight)
		gas -= params.CheckFileGas
		err := i(evm, sp, bl)
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result

		//StoreStorageContract
	case func(ethdb.Database, types.StorageContractID, types.StorageContract) error:
		if gas < params.SstoreSetGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 5 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		db, _ := args[2].(ethdb.Database)
		scid, _ := args[3].(types.StorageContractID)
		sc, _ := args[4].(types.StorageContract)
		gas -= params.SstoreSetGas
		err := i(db, scid, sc)
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result

		//StoreExpireStorageContract
	case func(ethdb.Database, types.StorageContractID, types.BlockHeight) error:
		if gas < params.SstoreSetGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 5 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		db, _ := args[2].(ethdb.Database)
		scid, _ := args[3].(types.StorageContractID)
		height, _ := args[4].(types.BlockHeight)
		gas -= params.SstoreSetGas
		err := i(db, scid, height)
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result

		//CheckMultiSignatures
	case func(interface{}, types.BlockHeight, []types.Signature) error:
		if gas < params.CheckMultiSignaturesGas {
			result = append(result, GasCalculationinsufficient)
			return gas, result
		}
		if len(args) != 5 {
			result = append(result, GasCalculationParamsNumberWorng)
			return gas, result
		}
		bl, _ := args[4].(types.BlockHeight)
		arrsig, _ := args[4].([]types.Signature)
		gas -= params.CheckMultiSignaturesGas
		err := i(args[2], bl, arrsig)
		if err != nil {
			result = append(result, err)
			return gas, result
		}
		result = append(result, nil)
		return gas, result
	default:
		result = append(result, GasCalculationParamsNumberWorng)
		return gas, result
	}

}
