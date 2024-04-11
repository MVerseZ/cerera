package types

import "math/big"

type BlockChainInfo struct {
	Chain        string   `json:"chain"`
	Blocks       int      `json:"blocks"`
	Headers      int      `json:"headers"`
	Difficulty   *big.Int `json:"difficulty"`
	Chainwork    *big.Int `json:"chainwork"`
	Size_on_disk int      `json:"size_on_disk"`
	Warnings     string   `json:"warnings"`
}
