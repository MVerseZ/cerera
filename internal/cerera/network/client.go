package network

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/types"
)

type Client struct {
	addr   types.Address
	status byte
}

var client Client
var (
	pollMinutes int = 10
)

func InitClient(cereraAddress types.Address) {
	c, err := net.Dial("tcp", "addr")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer c.Close()

	client = Client{
		addr:   cereraAddress,
		status: 0x1,
	}

	go customHandleConnectionClient(c)

	for {
		//time.Sleep(pollMinutes * time.Minute)
		time.Sleep(time.Duration(3) * time.Second)
	}
}

func customHandleConnectionClient(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Println("error closing connection:", err)
		}
	}()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var resp Response
	var reqParams = []interface{}{client.addr}
	hReq := Request{
		JSONRPC: "2.0",
		Method:  "cerera.consensus.join",
		Params:  reqParams,
		ID:      5422899109,
	}

	if err := enc.Encode(&hReq); err != nil {
		fmt.Println("failed to encode data:", err)
		return
	}

	for {

		if err := dec.Decode(&resp); err != nil {
			fmt.Println("failed to unmarshal request:", err)
			return
		}
		// result
		result := resp.Result

		switch v := result.(type) {
		case map[string]interface{}:

			switch s := client.status; s {
			case 0x1:
				tmpJson, err := json.Marshal(v)
				if err != nil {
					fmt.Println(err)
					continue
				}
				var b block.Block
				if err := json.Unmarshal(tmpJson, &b); err != nil {
					fmt.Println(err)
					return
				}
				//fmt.Printf("block: %s\r\n", b.Hash())
				var currentBlock = chain.GetBlockChain().GetLatestBlock()
				// fmt.Println(currentBlock.GetLatestBlock().Hash())
				// fmt.Println(b.Hash())
				if b.Hash().String() != currentBlock.Hash().String() {
					if b.Head.Number.Cmp(currentBlock.Head.Number) > 0 {
						var diff = big.NewInt(0).Sub(b.Head.Number, currentBlock.Head.Number)
						var reqParams = []interface{}{diff}
						hReq := Request{
							JSONRPC: "2.0",
							Method:  "cerera.consensus.sync",
							Params:  reqParams,
							ID:      5422899110,
						}
						if err := enc.Encode(&hReq); err != nil {
							fmt.Println("failed to encode data:", err)
							return
						}
					}
				}
				client.status = 0x2
			case 0x2:
				tmpJson, err := json.Marshal(v)
				if err != nil {
					fmt.Println(err)
					continue
				}
				var b block.Block
				if err := json.Unmarshal(tmpJson, &b); err != nil {
					fmt.Println(err)
					return
				}
				chain.GetBlockChain().UpdateChain(&b)
			default:

			}

		case string:
			fmt.Printf("block_str: %s\r\n", v)
		case float64:
			fmt.Printf("cons stat: %f\r\n", v)
		default:
			fmt.Println("unknown")
		}
	}
}
