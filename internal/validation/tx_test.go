package validation

import (
	"context"
	"math/big"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/pallada"
)

func TestValidateMempoolTx_AllowsQueuedNonce(t *testing.T) {
	cfg := &config.Config{
		IN_MEM: true,
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x34435Dd4078275B01D76A456FF9599fc07e61B90"),
		},
	}
	vaultIface, err := storage.NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault: %v", err)
	}
	vault := vaultIface.(*storage.D5Vault)

	key, err := types.GenerateAccount()
	if err != nil {
		t.Fatalf("GenerateAccount: %v", err)
	}
	sender := types.PubkeyToAddress(&key.PublicKey)
	receiver := types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002")
	if err := vault.DropFaucet(sender, types.FloatToBigInt(10), common.BytesToHash([]byte("faucet_queued"))); err != nil {
		t.Fatalf("DropFaucet: %v", err)
	}

	signer := types.NewSimpleSigner(big.NewInt(11))
	gasPrice := types.FloatToBigInt(pallada.MinGasPrice())
	accNonce := vault.Get(sender).Nonce

	for i, nonce := range []uint64{accNonce, accNonce + 1, accNonce + 2} {
		tx, err := types.CreateUnbroadcastTransaction(nonce, receiver, 1, 21000, gasPrice, "queued")
		if err != nil {
			t.Fatalf("create tx %d: %v", i, err)
		}
		signed, err := types.SignTx(tx, signer, key)
		if err != nil {
			t.Fatalf("sign tx %d: %v", i, err)
		}
		if err := ValidateMempoolTx(signer, *signed, vault, nil); err != nil {
			t.Fatalf("queued nonce %d rejected: %v", nonce, err)
		}
	}

	stale, err := types.CreateUnbroadcastTransaction(accNonce-1, receiver, 1, 21000, gasPrice, "stale")
	if err != nil {
		t.Fatalf("create stale: %v", err)
	}
	signedStale, err := types.SignTx(stale, signer, key)
	if err != nil {
		t.Fatalf("sign stale: %v", err)
	}
	if err := ValidateMempoolTx(signer, *signedStale, vault, nil); err == nil {
		t.Fatal("expected stale nonce to be rejected")
	}
}
