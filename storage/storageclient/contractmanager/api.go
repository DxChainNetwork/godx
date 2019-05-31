// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import "fmt"

// PublicContractManagerAPI defines the object used to call eligible public
// APIs that are used to acquire contract information that client signed
type PublicContractManagerAPI struct {
	cm *ContractManager
}

// NewPublicContractManagerAPI initialize PublicContractManagerAPI object
// which implemented a bunch of API methods
func NewPublicContractManagerAPI(cm *ContractManager) *PublicContractManagerAPI {
	return &PublicContractManagerAPI{
		cm: cm,
	}
}

// AllContracts will return all storage contracts that the storage client has signed
func (api *PublicContractManagerAPI) AllContracts() string {
	return "ALL CONTRACT IS CURRENTLY WIP"
}

type PrivateContractManagerAPI struct {
	cm *ContractManager
}

func NewPrivateContractManagerAPI(cm *ContractManager) *PrivateContractManagerAPI {
	return &PrivateContractManagerAPI{
		cm: cm,
	}
}

// CancelContracts will cancel the contract based on the contract ID
func (api *PrivateContractManagerAPI) CancelContract(contractID string) string {
	return fmt.Sprintf("WIP: the contract id you entered is %s", contractID)

}

func (api *PrivateContractManagerAPI) CancelAllContracts() string {
	return fmt.Sprintf("WIP: all contracts are canceled")
}

type PublicContractManagerDebugAPI struct {
	cm *ContractManager
}

func NewPublicContractManagerDebugAPI(cm *ContractManager) *PublicContractManagerDebugAPI {
	return &PublicContractManagerDebugAPI{
		cm: cm,
	}
}

func (api *PublicContractManagerDebugAPI) InsertContract(amount int) string {
	return fmt.Sprintf("Successfully inserted %v contracts", amount)
}
