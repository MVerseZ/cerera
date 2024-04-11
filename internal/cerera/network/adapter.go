package network

import (
	"bufio"
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
)

func (h Host) ServerProtocol(stream network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	for {
		data, _ := rw.ReadBytes('\r')

		if len(data) > 0 {
			fmt.Printf("RECEIVED (h): %d\r\n", data)
			var p = FromBytes(data)
			fmt.Println(p)
			// if p.T == 0xa {
			// 	var snap = storage.Sync()
			// 	var packet = new(Packet)
			// 	packet.T = 0x4
			// 	packet.Data = snap
			// 	packet.T = 0x4
			// 	fmt.Printf("SENDED (h): STORAGE SNAP %x\r\n", snap)
			// 	rw.Write(packet.Bytes())
			// 	rw.Flush()
			// }
		}

		// time.Sleep(time.Second * 2)
		// str, _ := rw.ReadString('\n')
		// if str == "" {
		// 	return
		// }
		// if str != "\n" {
		// 	fmt.Printf("RECEIVED (h): %s\r", str)
		// 	if strings.Contains(str, "OP_SYNC") {
		// 		// send to swarm
		// 		fmt.Printf("d:%d\r\n", storage.S())
		// 		fmt.Printf("SENDED (h): OP_SYNC_CLI\r\n")
		// 		rw.WriteString("OP_SYNC_CLI\n")
		// 		rw.Flush()
		// 	}
		// }
	}
}

func (h Host) ClientProtocol(rw *bufio.ReadWriter) {
	var p = new(Packet)
	p.T = 0x3
	p.Data = []byte("OP_I")
	p.EF = 0x3
	rw.Write(p.Bytes())
	for {
		data, _ := rw.ReadBytes('\r')
		if len(data) > 0 {
			fmt.Printf("RECEIVED (c): %x\r\n", data)
		}

		// str, _ := rw.ReadString('\n')
		// if str == "" {
		// 	return
		// }
		// if str != "\n" {
		// 	fmt.Printf("RECEIVED (с): %s\r", str)
		// 	if strings.Contains(str, "OP_SYNC") {
		// 		// send to swarm
		// 		var snap = storage.Sync()
		// 		var packet = new(Packet)
		// 		packet.T = 0x4
		// 		packet.Data = snap
		// 		packet.T = '\r'
		// 		fmt.Printf("SENDED (с): STORAGE SNAP %x\r\n", snap)
		// 		rw.Write(packet.Bytes())
		// 		rw.Flush()
		// 	}
		// }
	}
}
