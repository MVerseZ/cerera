package network

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/cerera/internal/cerera/types"
)

type Client struct {
	addr types.Address
}

var client Client
var (
	pollMinutes int = 10
)

func InitClient(cereraAddress types.Address) {
	c, err := net.Dial("tcp", "addr")

	if err != nil {
		panic(err)
	}
	defer c.Close()

	client = Client{
		addr: cereraAddress,
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
		ID:      1,
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
		fmt.Println(result)
	}
}
