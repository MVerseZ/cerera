package common

import (
	"math/big"
	"strconv"
)

func FloatToBigInt(val float64) *big.Int {
	// Convert float to decimal string, then use exact decimal -> dust converter
	// to avoid binary FP artifacts like ...0005
	s := strconv.FormatFloat(val, 'f', -1, 64)
	dust, err := DecimalStringToDust(s)
	if err != nil {
		return big.NewInt(0)
	}
	return dust
}
