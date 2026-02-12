package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/common"
)

// DecimalBig marshals/unmarshals as a JSON string in decimal format (not hex)
type DecimalBig big.Int

// MarshalJSON implements json.Marshaler
func (b DecimalBig) MarshalJSON() ([]byte, error) {
	bi := (*big.Int)(&b)
	return json.Marshal(bi.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (b *DecimalBig) UnmarshalJSON(input []byte) error {
	var s string
	if err := json.Unmarshal(input, &s); err != nil {
		return err
	}
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return fmt.Errorf("invalid decimal number: %s", s)
	}
	*b = DecimalBig(*bi)
	return nil
}

// DecimalUint64 marshals/unmarshals as a JSON number (not hex string)
type DecimalUint64 uint64

// MarshalJSON implements json.Marshaler
func (u DecimalUint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint64(u))
}

// UnmarshalJSON implements json.Unmarshaler
func (u *DecimalUint64) UnmarshalJSON(input []byte) error {
	var num uint64
	if err := json.Unmarshal(input, &num); err != nil {
		return err
	}
	*u = DecimalUint64(num)
	return nil
}

// txJSONOutput is used for serialization with unified format
type txJSONOutput struct {
	Hash     common.Hash    `json:"hash"`
	Type     uint64         `json:"type,omitempty"`
	To       *Address       `json:"to,omitempty"`
	From     *Address       `json:"from,omitempty"`
	Nonce    *DecimalUint64 `json:"nonce,omitempty"`
	Gas      *DecimalUint64 `json:"gas,omitempty"`
	GasPrice *DecimalBig    `json:"gasPrice,omitempty"`
	Value    *DecimalBig    `json:"value,omitempty"`
	Data     *common.Bytes  `json:"input,omitempty"`
	Dna      *common.Bytes  `json:"dna,omitempty"`
	Payload  *common.Bytes  `json:"payload,omitempty"`
	Time     time.Time      `json:"time,omitempty"`
	R        *Big           `json:"r,omitempty"`
	S        *Big           `json:"s,omitempty"`
	V        *Big           `json:"v,omitempty"`
}

// MarshalJSON marshals as JSON with unified format (decimal strings for value/gasPrice, numbers for gas/nonce)
func (tx *GTransaction) MarshalJSON() ([]byte, error) {
	var output txJSONOutput
	
	// Common fields for all transaction types
	output.Hash = tx.Hash()
	output.Type = uint64(tx.Type())
	output.To = tx.To()
	from := tx.From()
	if !from.IsEmpty() {
		output.From = &from
	}
	
	// Type-specific fields
	switch itx := tx.inner.(type) {
	case *PGTransaction:
		nonce := DecimalUint64(itx.Nonce)
		output.Nonce = &nonce
		gas := DecimalUint64(uint64(itx.Gas))
		output.Gas = &gas
		gasPrice := DecimalBig(*itx.GasPrice)
		output.GasPrice = &gasPrice
		value := DecimalBig(*itx.Value)
		output.Value = &value
		output.Data = (*common.Bytes)(&itx.Data)
		output.Dna = (*common.Bytes)(&itx.Dna)
		output.Time = tx.GetTime()
		output.Payload = (*common.Bytes)(&itx.Payload)
		var r, s, v = tx.RawSignatureValues()
		output.R = (*Big)(r)
		output.S = (*Big)(s)
		output.V = (*Big)(v)
	case *CBTransaction:
		nonce := DecimalUint64(itx.Nonce)
		output.Nonce = &nonce
		gas := DecimalUint64(uint64(itx.Gas))
		output.Gas = &gas
		gasPrice := DecimalBig(*itx.GasPrice)
		output.GasPrice = &gasPrice
		value := DecimalBig(*itx.Value)
		output.Value = &value
		output.Data = (*common.Bytes)(&itx.Data)
		output.Dna = (*common.Bytes)(&itx.Dna)
		output.Time = tx.GetTime()
		output.Payload = (*common.Bytes)(&itx.Payload)
		// Coinbase transactions may not have signature fields
	case *FaucetTransaction:
		nonce := DecimalUint64(itx.Nonce)
		output.Nonce = &nonce
		gas := DecimalUint64(uint64(itx.Gas))
		output.Gas = &gas
		gasPrice := DecimalBig(*itx.GasPrice)
		output.GasPrice = &gasPrice
		value := DecimalBig(*itx.Value)
		output.Value = &value
		output.Time = tx.GetTime()
	default:
		fmt.Printf("Unknown transaction type: %T\r\n", itx)
	}

	return json.Marshal(&output)
}

// txJSONInput is used for deserialization with unified format support
type txJSONInput struct {
	Hash     *common.Hash   `json:"hash,omitempty"`
	Type     *uint64        `json:"type,omitempty"`
	To       *Address       `json:"to,omitempty"`
	From     *Address       `json:"from,omitempty"`
	Nonce    *DecimalUint64 `json:"nonce,omitempty"`
	Gas      *DecimalUint64 `json:"gas,omitempty"`
	GasPrice *DecimalBig    `json:"gasPrice,omitempty"`
	Value    *DecimalBig    `json:"value,omitempty"`
	Data     *common.Bytes  `json:"input,omitempty"`
	Dna      *common.Bytes  `json:"dna,omitempty"`
	Payload  *common.Bytes  `json:"payload,omitempty"`
	Time     *time.Time     `json:"time,omitempty"`
	R        *Big           `json:"r,omitempty"`
	S        *Big           `json:"s,omitempty"`
	V        *Big           `json:"v,omitempty"`
}

// unmarshalFromNewFormat handles deserialization from new unified format
func (tx *GTransaction) unmarshalFromNewFormat(dec txJSONInput) error {
	if dec.Type == nil {
		return errors.New("missing required field 'type' in transaction")
	}

	var inner TxData
	switch *dec.Type {
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
		itx.Dna = *dec.Dna

		if dec.Time != nil {
			itx.Time = *dec.Time
		} else {
			itx.Time = time.Now()
		}

		if dec.R != nil {
			itx.R = (*big.Int)(dec.R)
		}
		if dec.S != nil {
			itx.S = (*big.Int)(dec.S)
		}
		if dec.V != nil {
			itx.V = (*big.Int)(dec.V)
		}
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
		itx.Gas = float64(*dec.Gas)

		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)

		if dec.Time != nil {
			itx.Time = *dec.Time
		} else {
			itx.Time = time.Now()
		}
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
		itx.Gas = float64(*dec.Gas)

		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)

		if dec.Data != nil {
			itx.Data = *dec.Data
		} else {
			itx.Data = []byte{}
		}

		if dec.Payload != nil {
			itx.Payload = *dec.Payload
		} else {
			itx.Payload = []byte{}
		}

		if dec.Dna != nil {
			itx.Dna = *dec.Dna
		} else {
			itx.Dna = []byte{}
		}

		if dec.Time != nil {
			itx.Time = *dec.Time
		} else {
			itx.Time = time.Now()
		}

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

	tx.setDecoded(inner, 0)
	return nil
}

// UnmarshalJSON unmarshals from JSON with support for new unified format
func (tx *GTransaction) UnmarshalJSON(input []byte) error {
	// Try new format first (with DecimalUint64 and DecimalBig)
	var decNew txJSONInput
	if err := json.Unmarshal(input, &decNew); err == nil && decNew.Type != nil {
		return tx.unmarshalFromNewFormat(decNew)
	}

	// Fallback to old format (with common.Uint64 and common.Big for backward compatibility)
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
