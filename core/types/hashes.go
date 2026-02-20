package types

import "github.com/cerera/internal/cerera/common"

const (
	// HashLength is the expected length of the hash
	HashLength = 32
	// AddressLength is the expected length of the address
	AddressLenUgth = 20
)

var (
	// // EmptyRootHash is the known root hash of an empty trie.
	// EmptyRootHash = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	EmptyRootHash = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// EmptyCodeHash is the known hash of the empty bytecode.
	EmptyCodeHash = INRISeqHash(nil)

	EmptyCodeRootHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
)
