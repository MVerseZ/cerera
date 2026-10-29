package main

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
	"github.com/cerera/icenet"
)

func main() {
	key, _ := types.GenerateAccount()
	tx, _ := types.CreateUnbroadcastTransaction(5, types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002"), 0.1, 1000, big.NewInt(1), "TEST MESSAGE 5")
	signer := types.NewSimpleSigner(big.NewInt(11))
	signed, _ := types.SignTx(tx, signer, key)
	signed.SetFrom(crypto.PrivKeyToAddress(*key))
	txMsg := icenet.TxMessage{Tx: signed, Hash: signed.Hash()}
	payload, _ := json.Marshal(txMsg)
	var tm icenet.TxMessage
	json.Unmarshal(payload, &tm)
	fmt.Printf("match=%v h0=%s h1=%s\n", tm.Tx.Hash() == tm.Hash, signed.Hash(), tm.Tx.Hash())
}
