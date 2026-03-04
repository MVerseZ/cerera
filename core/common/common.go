package common

import (
	"encoding/hex"
	"errors"
	"math/big"
	"math/rand"
	"time"
	"unsafe"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 73 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringBytesMaskImprSrcUnsafe(n int) string {
	var src = rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

func FromHex(s string) []byte {
	if Has0xPrefix(s) {
		s = s[2:]
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return Hex2Bytes(s)
}

func Hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)
	return h
}

// DecimalStringToDust converts exact decimal string (e.g. "1.23") into dust (10^6) without binary float errors.
// 1 CER = 1,000,000 DUST.
// Returns error for malformed input. Accepts optional leading +/-, and up to 6 fractional digits.
func DecimalStringToDust(s string) (*big.Int, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return nil, errors.New("bad decimal")
	}
	scale := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(6), nil))
	r.Mul(r, scale)
	// Round to nearest integer towards zero; adjust policy if needed
	if !r.IsInt() {
		r = r.SetFrac(r.Num(), r.Denom())
	}
	return new(big.Int).Set(r.Num()), nil
}

// DecimalStringToWei is an alias for DecimalStringToDust kept for backward compatibility.
func DecimalStringToWei(s string) (*big.Int, error) { return DecimalStringToDust(s) }

func BigIntToFloat(bi *big.Int) float64 {
	bigval := new(big.Float)
	bigval.SetInt(bi)
	bigval.SetPrec(128)

	// 1 CER = 1,000,000 DUST
	coin := new(big.Float)
	coin.SetInt(big.NewInt(1_000_000)) // 10^6

	bigval.Quo(bigval, coin)

	result, _ := bigval.Float64()

	return result
}

func Uint64ToBigInt(u uint64) *big.Int {
	return new(big.Int).SetUint64(u)
}
