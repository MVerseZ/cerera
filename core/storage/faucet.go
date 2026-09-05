package storage

import (
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

// DropFaucet mints new coins directly to the destination account with faucet metrics.
func (v *D5Vault) DropFaucet(to types.Address, cnt *big.Int, txHash common.Hash) error {
	if cnt == nil || cnt.Sign() <= 0 {
		return ErrInvalidMintAmount
	}
	if cnt.Cmp(coinbase.MinFaucetAmount) < 0 {
		return fmt.Errorf("faucet amount %s is below minimum %s", cnt.String(), coinbase.MinFaucetAmount.String())
	}
	if cnt.Cmp(coinbase.MaxFaucetAmount) > 0 {
		return fmt.Errorf("faucet amount %s exceeds maximum %s", cnt.String(), coinbase.MaxFaucetAmount.String())
	}

	v.faucetMu.Lock()
	if v.faucetLastRequest == nil {
		v.faucetLastRequest = make(map[types.Address]time.Time)
	}
	lastRequest, exists := v.faucetLastRequest[to]
	if exists {
		cooldownDuration := time.Duration(coinbase.FaucetCooldownHours) * time.Hour
		timeSinceLastRequest := time.Since(lastRequest)
		if timeSinceLastRequest < cooldownDuration {
			remainingTime := cooldownDuration - timeSinceLastRequest
			v.faucetMu.Unlock()
			return fmt.Errorf("faucet cooldown period not expired: please wait %v before next request", remainingTime.Round(time.Minute))
		}
	}
	v.faucetMu.Unlock()

	if _, err := v.creditMintedAmount(to, cnt, txHash); err != nil {
		return err
	}

	v.faucetMu.Lock()
	v.faucetLastRequest[to] = time.Now()
	v.faucetMu.Unlock()

	vaultFaucetAmountTotal.Add(types.BigIntToFloat(cnt))
	return nil
}
