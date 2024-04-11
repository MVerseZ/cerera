package types

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"

	"math/big"

	"github.com/cerera/internal/cerera/common"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrInvalidMsgLen       = errors.New("invalid message length, need 32 bytes")
	ErrInvalidSignatureLen = errors.New("invalid signature length")
	ErrInvalidRecoveryID   = errors.New("invalid signature recovery id")
	ErrInvalidKey          = errors.New("invalid private key")
	ErrInvalidPubkey       = errors.New("invalid public key")
	ErrSignFailed          = errors.New("signing failed")
	ErrRecoverFailed       = errors.New("recovery failed")
	ErrTxTypeNotSupported  = errors.New("Tx not supported")
)

type Address [common.AddressLength]byte

// events
type Event byte

const (
	EVENT_ADD    Event = 0x1
	EVENT_REMOVE Event = 0x2
	EVENT_STATUS Event = 0x3
)

var (
	addressT   = reflect.TypeOf(Address{})
	MaxAddress = HexToAddress("0xffffffffffffffffffffffffffffffffffffffff")
)

func InitSystemEvents() *[]Event {
	// inline system events
	return &[]Event{EVENT_ADD, EVENT_REMOVE, EVENT_STATUS}
}

func HexToAddress(s string) Address { return BytesToAddress(FromHex(s)) }

func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-common.AddressLength:]
	}
	copy(a[common.AddressLength-len(b):], b)
}

func (a Address) Bytes() []byte {
	dst := make([]byte, 48)
	copy(dst, a[:])
	return dst
}

func (a *Address) checksumHex() []byte {
	buf := a.hex()

	checkHash, _ := blake2b.New512(nil)
	checkHash.Write(buf[:4])
	hash := checkHash.Sum(nil)

	for i := 4; i < len(buf); i++ {
		hashByte := hash[(i-2)/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if buf[i] > '9' && hashByte > 7 {
			buf[i] -= 32
		}
	}
	return buf[:]
}

// type Address struct {
// 	address AddressB
// }

func EmptyAddress() Address {
	return BytesToAddress([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})
}

func (a *Address) FromString(data string) Address {
	fmt.Println(data)
	return Address{}
}

// Hex returns an compliant hex string representation of the address.
func (a Address) Hex() string {
	return string(a.checksumHex())
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return a.Hex()
}

func (a Address) hex() []byte {
	var buf [len(a)*2 + 2]byte
	copy(buf[:2], "0x")
	hex.Encode(buf[2:], a[:])
	return buf[:]
}

func (a Address) IsEmpty() bool {
	bts := a.Bytes()
	var cnt = 0
	for i := 0; i < len(bts); i++ {
		if bts[i] == 0x0 {
			cnt++
		}
	}
	return cnt == len(bts)
}

func (a Address) MarshalText() ([]byte, error) {
	// fmt.Printf("call marshal of address: %s\r\n", a.Hex())
	return common.Bytes(a[:]).MarshalText()
}

// UnmarshalText parses a hash in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	return common.UnmarshalFixedText("Address", input, a[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *Address) UnmarshalJSON(input []byte) error {
	return common.UnmarshalFixedJSON(addressT, input, a[:])
}

// Format implements fmt.Formatter.
// Address supports the %v, %s, %q, %x, %X and %d format verbs.
func (a Address) Format(s fmt.State, c rune) {
	switch c {
	case 'v', 's':
		s.Write(a.checksumHex())
	case 'q':
		q := []byte{'"'}
		s.Write(q)
		s.Write(a.checksumHex())
		s.Write(q)
	case 'x', 'X':
		// %x disables the checksum.
		hex := a.hex()
		if !s.Flag('#') {
			hex = hex[2:]
		}
		if c == 'X' {
			hex = bytes.ToUpper(hex)
		}
		s.Write(hex)
	case 'd':
		fmt.Fprint(s, ([len(a)]byte)(a))
	default:
		fmt.Fprintf(s, "%%!%c(address=%x)", c, a)
	}
}

func IsHexAddress(s string) bool {
	if common.Has0xPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*common.AddressLength && isHex(s)
}

func isHex(str string) bool {
	if len(str)%2 != 0 {
		return false
	}
	for _, c := range []byte(str) {
		if !isHexCharacter(c) {
			return false
		}
	}
	return true
}

func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func FromHex(s string) []byte {
	if common.Has0xPrefix(s) {
		s = s[2:]
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return Hex2Bytes(s)
}

// Hex2Bytes returns the bytes represented by the hexadecimal string str.
func Hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)
	return h
}

func GetServiceName(srv byte) string {
	if srv == 1 {
		return "RPC_SERVICE"
	}
	if srv == 2 {
		return "TX_POOL_SERVICE"
	}
	return "UNKNOWN_SERVICE"
}

type StateAccount struct {
	Address  Address
	Balance  *big.Int
	Bloom    []byte
	CodeHash []byte
	Name     string
	Nonce    uint64
	Root     common.Hash // merkle root of the storage trie
	Status   string
	// Treasury []*coinbase.CoinBase
	Inputs     []common.Hash
	Passphrase common.Hash
}

func (sa *StateAccount) BloomUp() {
	var tmpBloom = sa.Bloom[1]
	if sa.Bloom[1] < 0xf {
		sa.Bloom[1] = tmpBloom + 0x1
	} else {
		sa.Bloom[2] = 0xf
	}
}

func (sa *StateAccount) BloomDown() {
	var tmpBloom = sa.Bloom[1]
	if sa.Bloom[1] > 0x1 {
		sa.Bloom[1] = tmpBloom - 0x1
	} else {
		sa.Bloom[2] = 0xf
	}
}

func (sa *StateAccount) Bytes() []byte {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(sa)
	if err != nil {
		panic(err)
	}
	var saBytes = append(buf.Bytes(), sa.Bloom...)
	return saBytes
}

func BytesToStateAccount(data []byte) StateAccount {
	p := StateAccount{}
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&p)
	if err != nil {
		panic(err)
	}
	return p
}

type GTransactionData interface {
	txType() byte
	copy() GTransactionData
	setSignatureValues(chainID, r, s, x, y *big.Int)
	rawSignatureValues() (r, s, x, y *big.Int)
	nonce() uint64
	gas() uint64
	gasPrice() *big.Int
	value() *big.Int
	data() []byte
	dna() []byte
	to() *Address
	from() Address
	payload() []byte
}

type BridleStatus int

func GetFloat(unk interface{}) (float64, error) {
	switch i := unk.(type) {
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		return strconv.ParseFloat(i, 64)
	case *big.Int:
		return BigIntToFloat(i), nil
	default:
		v := reflect.ValueOf(unk)
		v = reflect.Indirect(v)
		// if v.Type().ConvertibleTo(fl) {
		// 	fv := v.Convert(floatType)
		// 	return fv.Float(), nil
		// } else if v.Type().ConvertibleTo(stringType) {
		// 	sv := v.Convert(stringType)
		// 	s := sv.String()
		// 	return strconv.ParseFloat(s, 64)
		// } else {
		return math.NaN(), fmt.Errorf("can't convert %v to float64", v.Type())
		// }
	}
}

func FloatToBigInt(val float64) *big.Int {
	bigval := new(big.Float)
	bigval.SetFloat64(val)
	// Set precision if required.
	// bigval.SetPrec(64)

	coin := new(big.Float)
	coin.SetInt(big.NewInt(10000000))

	bigval.Mul(bigval, coin)

	result := new(big.Int)
	bigval.Int(result) // store converted number in result

	return result
}

func BigIntToFloat(bi *big.Int) float64 {
	bigval := new(big.Float)
	bigval.SetInt(bi)

	coin := new(big.Float)
	coin.SetInt(big.NewInt(10000000))

	bigval.Quo(bigval, coin)

	result, acc := bigval.Float64()
	fmt.Printf("Accuracy converter:%d\r\n", acc)

	return result
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func IntToBytes(num int) []byte {
	bytes := make([]byte, 4) // Assuming you want a 4-byte representation for a 32-bit integer.
	binary.BigEndian.PutUint32(bytes, uint32(num))
	return bytes
}

func IsFileNotEmpty(filename string) (bool, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return false, err
	}

	return fileInfo.Size() > 0, nil
}
