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
		// PGTransaction.Gas is float64; encode as uint64 for JSON schema
		{
			u := common.Uint64(uint64(itx.Gas))
			enc.Gas = &u
		}
		enc.GasPrice = (*common.Big)(itx.GasPrice)
		enc.Value = (*common.Big)(itx.Value)
		enc.Data = (*common.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.Dna = (*common.Bytes)(&itx.Dna)
		enc.Time = (time.Time)(tx.GetTime())
		enc.Type = LegacyTxType
		enc.Hash = tx.Hash()
		enc.Payload = (*common.Bytes)(&itx.Payload)
		var r, s, v = tx.RawSignatureValues()
		enc.R = (*Big)(r)
		enc.S = (*Big)(s)
		enc.V = (*Big)(v)
	case *CBTransaction:
		enc.Nonce = (*common.Uint64)(&itx.Nonce)
		// CBTransaction.Gas is float64; encode as uint64 for JSON schema
		{
			u := common.Uint64(uint64(itx.Gas))
			enc.Gas = &u
		}
		enc.GasPrice = (*common.Big)(itx.GasPrice)
		enc.Value = (*common.Big)(itx.Value)
		enc.Data = (*common.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.Dna = (*common.Bytes)(&itx.Dna)
		enc.Time = (time.Time)(tx.GetTime())
		enc.Type = CoinbaseTxType
		enc.Hash = tx.Hash()
		enc.Payload = (*common.Bytes)(&itx.Payload)
		// var r, s, v = tx.RawSignatureValues()
		// enc.R = (*Big)(r)
		// enc.S = (*Big)(s)
		// enc.V = (*Big)(v)
	case *FaucetTransaction:
		enc.Nonce = (*common.Uint64)(&itx.Nonce)
		// CBTransaction.Gas is float64; encode as uint64 for JSON schema
		{
			u := common.Uint64(uint64(itx.Gas))
			enc.Gas = &u
		}
		enc.GasPrice = (*common.Big)(itx.GasPrice)
		enc.Value = (*common.Big)(itx.Value)
		enc.To = tx.To()
		enc.Time = (time.Time)(tx.GetTime())
		enc.Type = FaucetTxType
		enc.Hash = tx.Hash()
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
		// JSON carries gas as uint64; convert to float64 for PGTransaction
		itx.Gas = float64(*dec.Gas)

		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)

		if dec.Data == nil {
			return errors.New("missing required field 'data' in transaction")
		}
		itx.Data = *dec.Data

		if dec.Payload == nil {
			return errors.New("missing required field 'payload' in transaction")
		}
		itx.Payload = *dec.Payload

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
	case FaucetTxType:
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
		// JSON carries gas as uint64; convert to float64 for PGTransaction
		itx.Gas = float64(*dec.Gas)

		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)

		itx.Time = dec.Time
	case CoinbaseTxType:
		var itx CBTransaction
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
		// JSON carries gas as uint64; convert to float64 for CBTransaction
		itx.Gas = float64(*dec.Gas)

		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)

		if dec.Data == nil {
			itx.Data = []byte{}
		} else {
			itx.Data = *dec.Data
		}

		if dec.Payload == nil {
			itx.Payload = []byte{}
		} else {
			itx.Payload = *dec.Payload
		}

		if dec.Dna == nil {
			itx.Dna = []byte{}
		} else {
			itx.Dna = *dec.Dna
		}

		itx.Time = dec.Time
		// Coinbase transactions may not have signature fields
		if dec.R != nil {
			itx.R = (*big.Int)(dec.R)
		}
		if dec.S != nil {
			itx.S = (*big.Int)(dec.S)
		}
		if dec.V != nil {
			itx.V = (*big.Int)(dec.V)
		}
	default:
		return ErrTxTypeNotSupported
	}

	// set inner tx with size 0 to trigger recalculation
	tx.setDecoded(inner, 0)

	return nil
}
