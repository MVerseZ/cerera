package types

import (
	"github.com/cerera/core/common"
	"golang.org/x/crypto/blake2b"
)

// var hasherPool = sync.Pool{
// 	New: func() interface{} { return sha3.NewLegacyKeccak256() },
// }

func grlpHash(x interface{}) (h common.Hash) {
	return blake2b.Sum256(h[:])

	// sha := hasherPool.Get().(KessakState)
	// defer hasherPool.Put(sha)
	// sha.Reset()
	// rlp.Encode(sha, x)
	// sha.Read(h[:])
	// return h
}
