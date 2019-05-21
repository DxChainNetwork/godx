package storageclient

import (
	"crypto/ecdsa"
	"errors"
	"io"
	"math/big"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
)

// LoopFormContractRequest contains the request parameters for RPCLoopFormContract.
type FormContractRequest struct {
	StorageContract types.StorageContract
	RenterKey       ecdsa.PublicKey
}

func (fcr *FormContractRequest) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	if err := s.Decode(&fcr); err != nil {
		return err
	}
	log.Debug("rlp decode form contract request", "encode_size", rlp.ListSize(size))
	return nil
}

func (fcr *FormContractRequest) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, fcr)
}

// calculate renter and host collateral
func RenterPayoutsPreTax(host HostDBEntry, funding, basePrice, baseCollateral *big.Int, period, expectedStorage uint64) (renterPayout, hostPayout, hostCollateral *big.Int, err error) {

	// Divide by zero check.
	if host.StoragePrice.Sign() == 0 {
		host.StoragePrice.SetInt64(1)
	}

	// Underflow check.
	if funding.Cmp(host.ContractPrice) <= 0 {
		err = errors.New("underflow detected, funding < contractPrice")
		return
	}

	// Calculate renterPayout.
	renterPayout = new(big.Int).Sub(funding, host.ContractPrice)
	renterPayout = renterPayout.Sub(renterPayout, basePrice)

	// Calculate hostCollateral by calculating the maximum amount of storage
	// the renter can afford with 'funding' and calculating how much collateral
	// the host wouldl have to put into the contract for that. We also add a
	// potential baseCollateral.
	maxStorageSizeTime := new(big.Int).Div(renterPayout, host.StoragePrice)
	maxStorageSizeTime = maxStorageSizeTime.Mul(maxStorageSizeTime, host.Collateral)
	hostCollateral = maxStorageSizeTime.Add(maxStorageSizeTime, baseCollateral)

	// Don't add more collateral than 10x the collateral for the expected
	// storage to save on fees.
	host.Collateral = host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(period))
	host.Collateral = host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(expectedStorage))
	maxRenterCollateral := host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(5))
	if hostCollateral.Cmp(maxRenterCollateral) > 0 {
		hostCollateral = maxRenterCollateral
	}

	// Don't add more collateral than the host is willing to put into a single
	// contract.
	if hostCollateral.Cmp(host.MaxCollateral) > 0 {
		hostCollateral = host.MaxCollateral
	}

	// Calculate hostPayout.
	hostCollateral.Add(hostCollateral, host.ContractPrice)
	hostPayout = hostCollateral.Add(hostCollateral, basePrice)
	return
}

func NewRevision(current types.StorageContractRevision, cost *big.Int) types.StorageContractRevision {
	rev := current

	rev.NewValidProofOutputs = make([]types.DxcoinCharge, 2)
	rev.NewMissedProofOutputs = make([]types.DxcoinCharge, 3)
	copy(rev.NewValidProofOutputs, current.NewValidProofOutputs)
	copy(rev.NewMissedProofOutputs, current.NewMissedProofOutputs)

	// move valid payout from renter to host
	rev.NewValidProofOutputs[0].Value = current.NewValidProofOutputs[0].Value.Sub(current.NewValidProofOutputs[0].Value, cost)
	rev.NewValidProofOutputs[1].Value = current.NewValidProofOutputs[1].Value.Add(current.NewValidProofOutputs[1].Value, cost)

	// move missed payout from renter to void
	rev.NewMissedProofOutputs[0].Value = current.NewMissedProofOutputs[0].Value.Sub(current.NewMissedProofOutputs[0].Value, cost)
	rev.NewMissedProofOutputs[2].Value = current.NewMissedProofOutputs[2].Value.Add(current.NewMissedProofOutputs[2].Value, cost)

	// increment revision number
	rev.NewRevisionNumber++

	return rev
}
