// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"fmt"
	"strings"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/log"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rlp"
)

// ParseAndValidateCandidateApplyTxArgs will parse and validate the candidate apply transaction arguments
func ParseAndValidateCandidateApplyTxArgs(to common.Address, gas uint64, fields map[string]string, stateDB *state.StateDB, account *accounts.Manager) (*PrecompiledContractTxArgs, error) {
	// parse the candidateAddress field
	var candidateAddress common.Address
	if fromStr, ok := fields["from"]; ok {
		candidateAddress = common.HexToAddress(fromStr)
	} else {
		candidateAddress = defaultAccount(account)
		log.Info("Candidate account is automatically configured", "candidateAccount", account)
	}

	// form,  validate, and encode candidate tx data
	data, err := formAndValidateAndEncodeCandidateTxData(stateDB, candidateAddress, fields)
	if err != nil {
		return nil, err
	}

	return NewPrecompiledContractTxArgs(candidateAddress, to, data, nil, gas), nil
}

// ParseAndValidateVoteTxArgs will parse and validate the vote transaction arguments
func ParseAndValidateVoteTxArgs(to common.Address, gas uint64, fields map[string]string, stateDB *state.StateDB, account *accounts.Manager) (*PrecompiledContractTxArgs, error) {
	// parse the delegator account address
	var delegatorAddress common.Address
	if fromStr, ok := fields["from"]; ok {
		delegatorAddress = common.HexToAddress(fromStr)
	} else {
		delegatorAddress = defaultAccount(account)
		log.Info("Vote account is automatically configured", "voteAccount", account)
	}

	// form, validate, and encode vote tx data
	data, err := formAndValidateAndEncodeVoteTxData(stateDB, delegatorAddress, fields)
	if err != nil {
		return nil, err
	}

	return NewPrecompiledContractTxArgs(delegatorAddress, to, data, nil, gas), nil
}

// formAndValidateAndEncodeCandidateTxData will form, validate and encode candidate apply transaction data
func formAndValidateAndEncodeCandidateTxData(stateDB *state.StateDB, candidateAddress common.Address, fields map[string]string) ([]byte, error) {
	// form candidate tx data
	addCandidateTxData, err := formAddCandidateTxData(fields)
	if err != nil {
		return nil, err
	}

	// validate candidate tx data
	if err := dpos.CandidateTxDataValidation(stateDB, addCandidateTxData, candidateAddress); err != nil {
		return nil, err
	}

	// candidate transaction data encoding
	return rlp.EncodeToBytes(&addCandidateTxData)
}

// formAndValidateAndEncodeVoteTxData will form, validate, and encode vote transaction data
func formAndValidateAndEncodeVoteTxData(stateDB *state.StateDB, delegatorAddress common.Address, fields map[string]string) ([]byte, error) {
	// form the vote tx data
	voteTxData, err := formVoteTxData(fields)
	if err != nil {
		return nil, err
	}

	// voteTxData validation
	if err := dpos.VoteTxDepositValidation(stateDB, delegatorAddress, voteTxData); err != nil {
		return nil, err
	}

	// encode and return the data
	return rlp.EncodeToBytes(&voteTxData)
}

// formVoteTxData will parse the fields and form vote transaction data
func formVoteTxData(fields map[string]string) (data types.VoteTxData, err error) {
	// get deposit
	depositStr, ok := fields["deposit"]
	if !ok {
		return types.VoteTxData{}, fmt.Errorf("failed to form voteTxData, vote deposit is not provided")
	}

	// get candidates
	candidatesStr, ok := fields["candidates"]
	if !ok {
		return types.VoteTxData{}, fmt.Errorf("failed to form voteTxData, vote candidates is not provided")
	}

	// parse candidates
	if data.Candidates, err = parseCandidates(candidatesStr); err != nil {
		return types.VoteTxData{}, err
	}

	// parse deposit
	if data.Deposit, err = unit.ParseCurrency(depositStr); err != nil {
		return types.VoteTxData{}, err
	}

	return
}

// formAddCandidateTxData will parse the fields and form candidate apply transaction data
func formAddCandidateTxData(fields map[string]string) (data types.AddCandidateTxData, err error) {
	// get reward ratio
	rewardRatioStr, ok := fields["ratio"]
	if !ok {
		return types.AddCandidateTxData{}, fmt.Errorf("failed to form addCandidateTxData, candidate rewardRatio is not provided")
	}

	// get deposit
	depositStr, ok := fields["deposit"]
	if !ok {
		return types.AddCandidateTxData{}, fmt.Errorf("failed to form addCandidateTxData, candidate deposit is not provided")
	}

	// parse reward ratio
	if data.RewardRatio, err = parseRewardRatio(rewardRatioStr); err != nil {
		return types.AddCandidateTxData{}, err
	}

	// parse deposit
	if data.Deposit, err = unit.ParseCurrency(depositStr); err != nil {
		return types.AddCandidateTxData{}, err
	}

	return
}

// parseCandidates will convert the string to a list of candidate address
func parseCandidates(candidates string) ([]common.Address, error) {
	// strip all white spaces
	candidates = strings.Replace(candidates, " ", "", -1)

	// convert it to a list of candidates
	candidatesList := strings.Split(candidates, ",")

	// candidates addresses conversion
	var candidateAddresses []common.Address
	for _, candidate := range candidatesList {
		addr := common.HexToAddress(candidate)
		candidateAddresses = append(candidateAddresses, addr)
	}

	return candidateAddresses, nil
}

// parseRewardRatio is used to convert the ratio from string to uint64
// and to validate the parsed ratio
func parseRewardRatio(ratio string) (uint64, error) {
	// convert the string to uint64
	rewardRatio, err := unit.ParseUint64(ratio, 1, "")
	if err != nil {
		return 0, ErrParseStringToUint
	}

	// validate the rewardRatio and return
	return rewardRatioValidation(rewardRatio)
}

// rewardRatioValidation is used to validate the reward ratio
func rewardRatioValidation(ratio uint64) (uint64, error) {
	// check if the reward ratio
	if ratio > dpos.RewardRatioDenominator {
		return 0, ErrInvalidAwardDistributionRatio
	}
	return ratio, nil
}

// defaultAccount will return the first account address from the first wallet
func defaultAccount(account *accounts.Manager) common.Address {
	// check if there are existing account, if so, use the first one as
	// default account
	if wallets := account.Wallets(); len(wallets) > 0 {
		if walletAccounts := wallets[0].Accounts(); len(walletAccounts) > 0 {
			return walletAccounts[0].Address
		}
	}

	// otherwise, return empty account address
	return common.Address{}
}
