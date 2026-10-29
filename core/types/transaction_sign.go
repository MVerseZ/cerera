package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrInvalidSig        = errors.New("invalid signature")
	ErrBadKey            = errors.New("bad key")
	ErrInvalidRecoveryID = errors.New("invalid recovery id")
)

type sigCache struct {
	signer Signer
	from   Address
}

// Sign подписывает сообщение и возвращает 65-байтовую подпись (R,S,V)
func Sign(msg []byte, privKey *ecdsa.PrivateKey) ([]byte, error) {
	// Хешируем сообщение Blake2b-256
	h := blake2b.Sum256(msg)

	// Подписываем
	r, s, err := ecdsa.Sign(rand.Reader, privKey, h[:])
	if err != nil {
		return nil, err
	}

	// Формируем 65-байтовую подпись
	signature := make([]byte, 65)

	// R (32 байта)
	rBytes := r.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)

	// S (32 байта)
	sBytes := s.Bytes()
	copy(signature[64-len(sBytes):64], sBytes)

	// V в формате 27+recoveryID, чтобы IsSigned отличал подпись от нулевого V.
	if verifyWithRecoveryID(privKey, h[:], r, s, 0) {
		signature[64] = 27
	} else if verifyWithRecoveryID(privKey, h[:], r, s, 1) {
		signature[64] = 28
	} else {
		return nil, ErrInvalidRecoveryID
	}

	return signature, nil
}

// verifyWithRecoveryID проверяет, соответствует ли recovery ID ключу
func verifyWithRecoveryID(privKey *ecdsa.PrivateKey, hash []byte, r, s *big.Int, recoveryID int) bool {
	pubKey, err := recoverPublicKey(r, s, recoveryID, hash, privKey.Curve)
	if err != nil {
		return false
	}

	if pubKey == nil {
		return false
	}

	return pubKey.X.Cmp(privKey.PublicKey.X) == 0 && pubKey.Y.Cmp(privKey.PublicKey.Y) == 0
}

// recoverPublicKey восстанавливает публичный ключ из подписи
func recoverPublicKey(r, s *big.Int, recoveryID int, hash []byte, curve elliptic.Curve) (*ecdsa.PublicKey, error) {
	if recoveryID < 0 || recoveryID > 1 {
		return nil, ErrInvalidRecoveryID
	}

	params := curve.Params()

	// Проверяем r и s
	if r.Sign() <= 0 || s.Sign() <= 0 {
		return nil, errors.New("r and s must be positive")
	}
	if r.Cmp(params.N) >= 0 || s.Cmp(params.N) >= 0 {
		return nil, errors.New("r or s out of range")
	}

	// Получаем точку R на кривой
	R, err := recoverPoint(r, recoveryID, curve)
	if err != nil {
		return nil, err
	}

	if R == nil {
		return nil, errors.New("failed to recover point R")
	}

	// Вычисляем публичный ключ: Q = r^(-1) * (s * R - hash * G)
	rInv := new(big.Int).ModInverse(r, params.N)
	if rInv == nil {
		return nil, errors.New("r has no inverse modulo N")
	}

	scalarLen := (params.BitSize + 7) / 8

	// Вычисляем s * R
	sRX, sRY := curve.ScalarMult(R.X, R.Y, padScalar(s, scalarLen))
	if sRX == nil || sRY == nil {
		return nil, errors.New("failed to compute s*R")
	}

	// e = hash интерпретируется как целое по модулю N (как в ecdsa.Sign)
	var eGX, eGY *big.Int
	if len(hash) == 0 {
		eGX = big.NewInt(0)
		eGY = big.NewInt(0)
	} else {
		e := hashToInt(hash, curve)
		e.Mod(e, params.N)
		if e.Sign() == 0 {
			eGX = big.NewInt(0)
			eGY = big.NewInt(0)
		} else {
			eGX, eGY = curve.ScalarBaseMult(padScalar(e, scalarLen))
			if eGX == nil || eGY == nil {
				return nil, errors.New("failed to compute hash*G")
			}
		}
	}

	// Проверяем, что s*R лежит на кривой (если это не точка на бесконечности)
	if !(sRX.Sign() == 0 && sRY.Sign() == 0) && !curve.IsOnCurve(sRX, sRY) {
		return nil, errors.New("s*R is not on curve")
	}

	var pubX, pubY *big.Int

	// Если hash*G = 0, то просто берем s*R
	if eGX.Sign() == 0 && eGY.Sign() == 0 {
		pubX, pubY = sRX, sRY
	} else {
		if !curve.IsOnCurve(eGX, eGY) {
			return nil, errors.New("hash*G is not on curve")
		}

		// Для Weierstrass -(x,y) = (x, -y mod p), не (-x, -y).
		negEGX := new(big.Int).Set(eGX)
		negEGY := new(big.Int).Sub(params.P, eGY)
		negEGY.Mod(negEGY, params.P)
		if !curve.IsOnCurve(negEGX, negEGY) {
			return nil, errors.New("negated hash*G is not on curve")
		}

		pubX, pubY = curve.Add(sRX, sRY, negEGX, negEGY)
		if pubX == nil || pubY == nil {
			return nil, errors.New("failed to add points")
		}
	}

	// Проверяем результат
	if !(pubX.Sign() == 0 && pubY.Sign() == 0) && !curve.IsOnCurve(pubX, pubY) {
		return nil, errors.New("invalid intermediate point")
	}

	// Умножаем на r^(-1)
	if !(pubX.Sign() == 0 && pubY.Sign() == 0) {
		pubX, pubY = curve.ScalarMult(pubX, pubY, padScalar(rInv, scalarLen))
		if pubX == nil || pubY == nil {
			return nil, errors.New("failed to multiply by r^(-1)")
		}
	}

	// Финальная проверка
	if !(pubX.Sign() == 0 && pubY.Sign() == 0) && !curve.IsOnCurve(pubX, pubY) {
		return nil, errors.New("recovered point is not on curve")
	}

	return &ecdsa.PublicKey{Curve: curve, X: pubX, Y: pubY}, nil
}

// recoverPlain восстанавливает адрес из подписи.
// hash — тот же digest, что передавался в ecdsa.Sign (blake2b-256 сообщения).
func recoverPlain(hash []byte, r, s, v *big.Int) (Address, error) {
	recoveryID := int(v.Uint64())
	if recoveryID >= 27 {
		recoveryID -= 27
	}
	if recoveryID < 0 || recoveryID > 1 {
		return Address{}, ErrInvalidRecoveryID
	}

	curve := crypto.ChainElliptic()
	pubKey, err := recoverPublicKey(r, s, recoveryID, hash, curve)
	if err != nil {
		return Address{}, err
	}

	return crypto.PubkeyToAddress(pubKey), nil
}

func padScalar(k *big.Int, size int) []byte {
	out := make([]byte, size)
	b := k.Bytes()
	if len(b) > size {
		copy(out, b[len(b)-size:])
		return out
	}
	copy(out[size-len(b):], b)
	return out
}

func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}
	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}

// recoverPoint восстанавливает точку R на кривой
func recoverPoint(r *big.Int, recoveryID int, curve elliptic.Curve) (*ecdsa.PublicKey, error) {
	if recoveryID < 0 || recoveryID > 1 {
		return nil, ErrInvalidRecoveryID
	}

	params := curve.Params()

	// r должен быть в диапазоне [1, n-1]
	if r.Cmp(big.NewInt(1)) < 0 || r.Cmp(params.N) >= 0 {
		return nil, errors.New("r out of range")
	}

	// Вычисляем x = r
	x := new(big.Int).Set(r)

	// Вычисляем y^2 = x^3 - 3x + b (для P-256)
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	threeX := new(big.Int).Mul(big.NewInt(3), x)

	y2 := new(big.Int).Sub(x3, threeX)
	y2.Add(y2, params.B)
	y2.Mod(y2, params.P)

	// Находим квадратный корень
	y := modSqrt(y2, params.P)
	if y == nil {
		return nil, errors.New("cannot compute square root")
	}

	// Корректируем y в зависимости от recoveryID
	if (y.Bit(0) == 0 && recoveryID == 1) || (y.Bit(0) == 1 && recoveryID == 0) {
		y.Sub(params.P, y)
	}

	// Проверяем, что точка лежит на кривой
	if !curve.IsOnCurve(x, y) {
		return nil, errors.New("point is not on curve")
	}

	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

// modSqrt вычисляет квадратный корень по модулю p (для p ≡ 3 mod 4)
func modSqrt(a, p *big.Int) *big.Int {
	// Проверяем, что a не 0
	if a.Sign() == 0 {
		return big.NewInt(0)
	}

	// Для p ≡ 3 (mod 4): sqrt = a^((p+1)/4) mod p
	exp := new(big.Int).Add(p, big.NewInt(1))
	exp.Div(exp, big.NewInt(4))
	result := new(big.Int).Exp(a, exp, p)

	// Проверяем результат
	check := new(big.Int).Mul(result, result)
	check.Mod(check, p)

	if check.Cmp(a) == 0 {
		return result
	}
	return nil
}

// decodeSignature декодирует 65-байтовую подпись
func decodeSignature(sig []byte) (r, s, v *big.Int) {
	if len(sig) != 65 {
		return new(big.Int), new(big.Int), new(big.Int)
	}

	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetUint64(uint64(sig[64]))

	return r, s, v
}

// // recoverPlain восстанавливает адрес из подписи
// func recoverPlain(r, s, v *big.Int) (Address, error) {
// 	recoveryID := int(v.Uint64())
// 	if recoveryID >= 27 {
// 		recoveryID -= 27
// 	}
// 	if recoveryID < 0 || recoveryID > 1 {
// 		return Address{}, ErrInvalidRecoveryID
// 	}

// 	curve := crypto.ChainElliptic()
// 	pubKey, err := recoverPublicKey(r, s, recoveryID, nil, curve)
// 	if err != nil {
// 		return Address{}, err
// 	}

// 	return crypto.PubkeyToAddress(pubKey), nil
// }

// SignTx подписывает транзакцию
func SignTx(tx *GTransaction, s Signer, prv *ecdsa.PrivateKey) (*GTransaction, error) {
	h := s.Hash(tx)
	sig, err := Sign(h[:], prv)
	if err != nil {
		return nil, ErrBadKey
	}

	from := crypto.PrivKeyToAddress(*prv)
	tx.from.Store(sigCache{
		signer: s,
		from:   from,
	})

	signTx, errSign := tx.WithSignature(s, sig)
	if errSign != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", errSign)
	}

	return signTx, nil
}

// Sender возвращает отправителя транзакции
func Sender(signer Signer, tx *GTransaction) (Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	r, s, v := tx.RawSignatureValues()
	if r == nil || s == nil || v == nil {
		return Address{}, ErrInvalidSig
	}

	digest := blake2b.Sum256(signer.Hash(tx).Bytes())
	addr, err := recoverPlain(digest[:], r, s, v)
	if err != nil {
		return Address{}, err
	}

	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// SimpleSigner - простой подписыватель
type SimpleSigner struct {
	chainId, chainIdMul *big.Int
}

func NewSimpleSigner(chainId *big.Int) Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return SimpleSigner{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
}

func (ss SimpleSigner) ChainID() *big.Int {
	return ss.chainId
}

func (s1 SimpleSigner) Equal(s2 Signer) bool {
	ss, ok := s2.(SimpleSigner)
	return ok && ss.chainId.Cmp(s1.chainId) == 0
}

func (fs SimpleSigner) Hash(tx *GTransaction) common.Hash {
	return crvTxHash(tx.inner)
}

func (fs SimpleSigner) Sender(tx *GTransaction) (Address, error) {
	if tx.Type() != LegacyTxType {
		return Address{}, ErrTxTypeNotSupported
	}

	r, s, v := tx.RawSignatureValues()
	if r == nil || s == nil || v == nil {
		return Address{}, ErrInvalidSig
	}

	digest := blake2b.Sum256(fs.Hash(tx).Bytes())
	return recoverPlain(digest[:], r, s, v)
}

func (fs SimpleSigner) SignatureValues(tx *GTransaction, sig []byte) (R, S, V *big.Int, err error) {
	R, S, V = decodeSignature(sig)
	return R, S, V, nil
}

func (fs SimpleSigner) SignTransaction(tx *GTransaction, k *ecdsa.PrivateKey) (common.Hash, error) {
	sTx, err := SignTx(tx, fs, k)
	if err != nil {
		return common.EmptyHash(), err
	}
	return sTx.Hash(), nil
}

// VerifySignature проверяет подпись
func VerifySignature(pubKey *ecdsa.PublicKey, msg []byte, sig []byte) bool {
	if len(sig) != 65 {
		return false
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])

	h := blake2b.Sum256(msg)
	return ecdsa.Verify(pubKey, h[:], r, s)
}
