package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/common"
)

// MarshalJSON marshals as JSON with a hash.
func (tx *GTransaction) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// fmt.Printf("Marshall tx with hash: %s send to: %s\r\n", tx.Hash(), tx.To())
	if tx.dna != nil {
		// 	return []byte{}, nil
		// }
		// for all txs types
		enc.Hash = tx.Hash()
		enc.Type = common.Uint64(tx.Type())
		enc.To = tx.To()
	}
	// other fields
	switch itx := tx.inner.(type) {
	case *PGTransaction:
		enc.Nonce = (*common.Uint64)(&itx.Nonce)
		enc.Gas = (*common.Uint64)(&itx.Gas)
		enc.GasPrice = (*common.Big)(itx.GasPrice)
		enc.Value = (*common.Big)(itx.Value)
		enc.Data = (*common.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.Dna = (*common.Bytes)(&itx.Dna)
		enc.Time = (time.Time)(tx.GetTime())
		enc.Type = 4
		enc.Hash = tx.Hash()
		enc.Payload = (*common.Bytes)(&itx.Payload)
		var r, s, v = tx.RawSignatureValues()
		enc.R = (*Big)(r)
		enc.S = (*Big)(s)
		enc.V = (*Big)(v)
	case *GSTransaction:
		enc.Nonce = (*common.Uint64)(&itx.Nonce)
		enc.Gas = (*common.Uint64)(&itx.Gas)
		enc.GasPrice = (*common.Big)(itx.GasPrice)
		enc.Value = (*common.Big)(itx.Value)
		enc.Data = (*common.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.Dna = (*common.Bytes)(&itx.Dna)
		enc.Time = (time.Time)(tx.GetTime())
		enc.Type = 4
		enc.Hash = tx.Hash()
		enc.Payload = (*common.Bytes)(&itx.Payload)
		var r, s, v = tx.RawSignatureValues()
		enc.R = (*Big)(r)
		enc.S = (*Big)(s)
		enc.V = (*Big)(v)
		panic("NOT IMPLEMENTED YET")
	default:
		fmt.Printf("%T\r\n", itx)
	}

	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (tx *GTransaction) UnmarshalJSON(input []byte) error {
	var dec txJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	// handle fields by tx type
	var inner TxData
	// fmt.Printf("TX TYPE: %d\r\n", dec.Type)
	// fmt.Printf("TX TYPE: %s\r\n", dec.Type)
	// fmt.Printf("TX TO: %s\r\n", dec.To)
	switch dec.Type {
	case LegacyTxType:
		var itx PGTransaction
		inner = &itx
		if dec.To == nil {
			return errors.New("missing required field 'to' in transaction")
		}
		itx.To = dec.To

		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)

		if dec.GasPrice == nil {
			return errors.New("missing required field 'gasPrice' in transaction")
		}
		itx.GasPrice = (*big.Int)(dec.GasPrice)

		if dec.Gas == nil {
			return errors.New("missing required field 'gas' in transaction")
		}
		itx.Gas = uint64(*dec.Gas)

		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)

		if dec.Data == nil {
			return errors.New("missing required field 'data' in transaction")
		}
		itx.Data = *dec.Data

		if dec.Dna == nil {
			return errors.New("missing required field 'dna' in transaction")
		}

		if dec.R == nil {
			return errors.New("missing required field 'R' in transaction")
		}
		itx.R = (*big.Int)(dec.R)

		if dec.S == nil {
			return errors.New("missing required field 'S' in transaction")
		}
		itx.S = (*big.Int)(dec.S)

		if dec.V == nil {
			return errors.New("missing required field 'V' in transaction")
		}
		itx.V = (*big.Int)(dec.V)

		itx.Dna = *dec.Dna

		itx.Time = dec.Time
	default:
		return ErrTxTypeNotSupported
	}

	// set inner tx
	tx.setDecoded(inner, 0)

	// TODO: check hash here?
	return nil
}
