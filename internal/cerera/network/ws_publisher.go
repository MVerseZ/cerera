package network

import (
	"net"

	"github.com/btcsuite/websocket"
)

type WebSocketResponse struct {
	Event string
	Data  interface{}
}

var connectionsPool = make(map[net.Addr]*websocket.Conn)

func AddWsClientConnection(wsConnection *websocket.Conn) {
	connectionsPool[wsConnection.RemoteAddr()] = wsConnection
}

func PublishData(dataType string, data interface{}) {
	var wsResp = WebSocketResponse{
		dataType, data,
	}
	for _, w := range connectionsPool {
		w.WriteJSON(wsResp)
	}
}
