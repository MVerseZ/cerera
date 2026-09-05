package storage

import (
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

// faucetMint is an RPC faucet credit that lives outside the chain.
// State-root replay starts from baseline and must re-apply these mints
// so transfers funded by cerera.account.faucet see a real balance.
type faucetMint struct {
	To     types.Address
	Amount *big.Int
	Hash   common.Hash
}

// ReplayFaucetMints credits recorded RPC faucet drops onto the current account set.
func (v *D5Vault) ReplayFaucetMints() error {
	if v == nil {
		return nil
	}
	v.faucetMu.RLock()
	mints := make([]faucetMint, len(v.faucetMints))
	copy(mints, v.faucetMints)
	v.faucetMu.RUnlock()
	for _, m := range mints {
		if m.Amount == nil {
			continue
		}
		if _, err := v.creditMintedAmount(m.To, m.Amount, m.Hash); err != nil {
			return err
		}
	}
	return nil
}

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
		cooldownDuration := time.Duration(coinbase.FaucetCooldownHours * float64(time.Hour))
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
	v.faucetMints = append(v.faucetMints, faucetMint{
		To:     to,
		Amount: new(big.Int).Set(cnt),
		Hash:   txHash,
	})
	v.faucetMu.Unlock()

	vaultFaucetAmountTotal.Add(types.BigIntToFloat(cnt))
	return nil
}
