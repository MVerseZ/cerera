package icenet

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
)

func TestTxMessagePubSubRoundtrip_SignedLegacy(t *testing.T) {
	key, err := types.GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := types.CreateUnbroadcastTransaction(
		5,
		types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002"),
		0.1,
		1000,
		big.NewInt(1),
		"TEST MESSAGE 5",
	)
	if err != nil {
		t.Fatal(err)
	}
	signer := types.NewSimpleSigner(big.NewInt(11))
	signed, err := types.SignTx(tx, signer, key)
	if err != nil {
		t.Fatal(err)
	}
	signed.SetFrom(crypto.PrivKeyToAddress(*key))

	txMsg := TxMessage{Tx: signed, Hash: signed.Hash()}
	payload, err := json.Marshal(txMsg)
	if err != nil {
		t.Fatal(err)
	}
	outer := PubSubMessage{Type: PubSubMsgTypeTx, Payload: payload}
	data, err := json.Marshal(outer)
	if err != nil {
		t.Fatal(err)
	}

	var ps PubSubMessage
	if err := json.Unmarshal(data, &ps); err != nil {
		t.Fatal(err)
	}
	var got TxMessage
	if err := json.Unmarshal(ps.Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Tx == nil {
		t.Fatal("nil tx after unmarshal")
	}
	if got.Tx.Hash() != got.Hash {
		t.Fatalf("hash mismatch: payload tx %s envelope %s", got.Tx.Hash(), got.Hash)
	}
	if got.Hash != signed.Hash() {
		t.Fatalf("hash drift: want %s got %s", signed.Hash(), got.Hash)
	}
}

func TestTxMessagePubSubRoundtrip_AfterValidatorStyleSign(t *testing.T) {
	key, err := types.GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	spk := types.EncodePrivateKeyToToString(key)
	tx, err := types.CreateUnbroadcastTransaction(
		1,
		types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002"),
		0.1,
		21000,
		big.NewInt(1),
		"TEST MESSAGE 0",
	)
	if err != nil {
		t.Fatal(err)
	}
	signer := types.NewSimpleSigner(big.NewInt(11))
	signTx, err := types.SignTx(tx, signer, key)
	if err != nil {
		t.Fatal(err)
	}
	*tx = *signTx
	_ = spk

	txMsg := TxMessage{Tx: tx, Hash: tx.Hash()}
	payload, err := json.Marshal(txMsg)
	if err != nil {
		t.Fatal(err)
	}
	var got TxMessage
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Tx.Hash() != got.Hash {
		t.Fatalf("hash mismatch: payload tx %s envelope %s", got.Tx.Hash(), got.Hash)
	}
}
