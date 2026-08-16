package validator

import "github.com/cerera/core/types"

// Exec DTOs (typed request objects)
type CreateTxParams struct {
	Key    string
	Nonce  uint64
	To     types.Address
	Amount string // decimal string CER, e.g. "1.23"
	Gas    float64
	Msg    string
}

type SendTxParams struct {
	Key    string
	ToHex  string
	Amount string // decimal string CER
	Gas    float64
	Msg    string
}
