package ethapi

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"

	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/firehose"
)

type StateOverrides map[libcommon.Address]Account

func (overrides *StateOverrides) Override(state *state.IntraBlockState) error {

	for addr, account := range *overrides {
		// Override account nonce.
		if account.Nonce != nil {
			state.SetNonce(addr, uint64(*account.Nonce), firehose.NoOpContext)
		}
		// Override account(contract) code.
		if account.Code != nil {
			state.SetCode(addr, *account.Code, firehose.NoOpContext)
		}
		// Override account balance.
		if account.Balance != nil {
			balance, overflow := uint256.FromBig((*big.Int)(*account.Balance))
			if overflow {
				return fmt.Errorf("account.Balance higher than 2^256-1")
			}
			state.SetBalance(addr, balance, firehose.NoOpContext, firehose.IgnoredBalanceChangeReason)
		}
		if account.State != nil && account.StateDiff != nil {
			return fmt.Errorf("account %s has both 'state' and 'stateDiff'", addr.Hex())
		}
		// Replace entire state if caller requires.
		if account.State != nil {
			state.SetStorage(addr, *account.State, firehose.NoOpContext)
		}
		// Apply state diff into specified accounts.
		if account.StateDiff != nil {
			for key, value := range *account.StateDiff {
				key := key
				state.SetState(addr, &key, value, firehose.NoOpContext)
			}
		}
	}

	return nil
}
