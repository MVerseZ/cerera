package common

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
)

const (
	HashLength    = 32
	GnuHashLength = 48
	AddressLength = 48 // up from 24 to 48 is secure move
)

var (
	hashT = reflect.TypeOf(Hash{})
)

type Hash [HashLength]byte

func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}
	copy(h[HashLength-len(b):], b)
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func (h Hash) Hex() string {
	enc := make([]byte, len(h)*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], h[:])
	return string(enc)
}

func (h Hash) String() string {
	return h.Hex()
}

// BigToHash sets byte representation of b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BigToHash(b *big.Int) Hash { return BytesToHash(b.Bytes()) }

// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash(s string) Hash { return BytesToHash(FromHex(s)) }

// Big converts a hash to a big integer.
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// TODO r&d later
func (h Hash) TerminalString() string {
	// return fmt.Sprintf("%x..%x", h[:3], h[29:])
	return fmt.Sprintf("%x", h[:])
}

// Format implements fmt.Formatter.
// Hash supports the %v, %s, %q, %x, %X and %d format verbs.
func (h Hash) Format(s fmt.State, c rune) {
	hexb := make([]byte, 2+len(h)*2)
	copy(hexb, "0x")
	hex.Encode(hexb[2:], h[:])

	switch c {
	case 'x', 'X':
		if !s.Flag('#') {
			hexb = hexb[2:]
		}
		if c == 'X' {
			hexb = bytes.ToUpper(hexb)
		}
		fallthrough
	case 'v', 's':
		s.Write(hexb)
	case 'q':
		q := []byte{'"'}
		s.Write(q)
		s.Write(hexb)
		s.Write(q)
	case 'd':
		fmt.Fprint(s, ([len(h)]byte)(h))
	default:
		fmt.Fprintf(s, "%%!%c(hash=%x)", c, h)
	}
}

// UnmarshalText parses a hash in hex syntax.
func (h *Hash) UnmarshalText(input []byte) error {
	return UnmarshalFixedText("Hash", input, h[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (h *Hash) UnmarshalJSON(input []byte) error {
	return UnmarshalFixedJSON(hashT, input, h[:])
}

// MarshalText returns the hex representation of h.
func (h Hash) MarshalText() ([]byte, error) {
	return Bytes(h[:]).MarshalText()
}

// Generate implements testing/quick.Generator.
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

// Scan implements Scanner for database/sql.
func (h *Hash) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Hash", src)
	}
	if len(srcB) != HashLength {
		return fmt.Errorf("can't scan []byte of len %d into Hash, want %d", len(srcB), HashLength)
	}
	copy(h[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (h Hash) Value() (driver.Value, error) {
	return h[:], nil
}

func EmptyHash() Hash {
	var h Hash
	h.SetBytes(FromHex("0x0000000000000000000000000000000000000000000000000000000000000000"))
	return h
}

// Compare compares callable and y and returns:
//
//	-1 if callable <  y
//	 0 if callable == y
//	+1 if callable >  y
func (h Hash) Compare(other Hash) int {
	var b1 = h.Bytes()
	var b2 = other.Bytes()
	var res = 0
	for i := 0; i < len(b1); i++ {
		if b1[i] == b2[i] {
			continue
		}
		if b1[i] > b2[i] {
			res = 1
			break
		}
		if b1[i] < b2[i] {
			res = -1
			break
		}
	}
	return res
}
