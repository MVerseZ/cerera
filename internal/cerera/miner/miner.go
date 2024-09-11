package miner

// PROTOTYPE STRUCTURE
import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SimpleTransaction struct {
	ID     int
	from   string
	to     string
	amount int
}
type SimpleBlock struct {
	ID                int
	previousBlockHash string
	transactions      []SimpleTransaction
}

func blockHash(o interface{}, nonce string) []byte {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v%s", o, nonce)))
	return h.Sum(nil)
}
func mine(o interface{}, difficulty int) {
	maxnonce := 100000000000
	prefixZeros := strings.Repeat("0", difficulty)
	var nonce int
	for nonce <= maxnonce {
		hashResult := fmt.Sprintf("%x", blockHash(o, strconv.Itoa(nonce)))
		nonce++
		if strings.HasPrefix(hashResult, prefixZeros) {
			fmt.Printf("Nouce is : %d\r\n", nonce)
			fmt.Printf("The block Hash  is : %s\r\n", hashResult)
			break
		}
	}

}
func Start() {
	currentTime := time.Now()
	difficulty := 10 // increase number for make mining harder
	transone := SimpleTransaction{
		ID:     1,
		from:   "mahdi",
		to:     "mohammad",
		amount: 25,
	}
	tanstwo := SimpleTransaction{
		ID:     2,
		from:   "roya",
		to:     "negin",
		amount: 99,
	}
	block := SimpleBlock{
		ID:                1,
		previousBlockHash: "00000000001fd09d3cf161db54434c9e518cf80e94811e7762c3aee8a7af39af",
		transactions:      []SimpleTransaction{transone, tanstwo},
	}
	fmt.Println("Start Mining")
	mine(block, difficulty)
	fmt.Println("End of Mining")
	duration := time.Since(currentTime)
	fmt.Printf("Mining took %s\r\n", duration)

}
