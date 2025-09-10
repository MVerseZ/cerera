package network

import (
	"log"
	"sync"

	"github.com/btcsuite/websocket"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/pool"
)

// WsManager manages WebSocket connections
type WsManager struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.Mutex
}

var WsMgr *WsManager

// NewWsManager creates a new WebSocket manager instance
func NewWsManager() *WsManager {
	if WsMgr == nil {
		WsMgr = &WsManager{
			clients:    make(map[*websocket.Conn]bool),
			broadcast:  make(chan []byte),
			register:   make(chan *websocket.Conn),
			unregister: make(chan *websocket.Conn),
		}
	}
	return WsMgr
}

// Start runs the WebSocket manager
func (manager *WsManager) Start() {
	var bc = chain.GetBlockChain()
	var pul = pool.Get()
	for {
		select {
		case conn := <-manager.register:
			manager.mutex.Lock()
			manager.clients[conn] = true
			manager.mutex.Unlock()
			log.Printf("New client connected. Total clients: %d", len(manager.clients))

		case conn := <-manager.unregister:
			manager.mutex.Lock()
			if _, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				conn.Close()
			}
			manager.mutex.Unlock()
			log.Printf("Client disconnected. Total clients: %d", len(manager.clients))

		case message := <-manager.broadcast:
			manager.mutex.Lock()
			for conn := range manager.clients {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Println("Error writing message:", err)
					conn.Close()
					delete(manager.clients, conn)
				}
			}
			manager.mutex.Unlock()
		case message := <-bc.DataChannel:
			manager.mutex.Lock()
			for conn := range manager.clients {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Println("Error writing message:", err)
					conn.Close()
					delete(manager.clients, conn)
				}
			}
			manager.mutex.Unlock()
		case message := <-pul.DataChannel:
			manager.mutex.Lock()
			for conn := range manager.clients {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Println("Error writing message:", err)
					conn.Close()
					delete(manager.clients, conn)
				}
			}
			manager.mutex.Unlock()
			// case message := <-gigea.C.MetricChannel:
			// 	manager.mutex.Lock()
			// 	for conn := range manager.clients {
			// 		err := conn.WriteMessage(websocket.TextMessage, message)
			// 		if err != nil {
			// 			log.Println("Error writing message:", err)
			// 			conn.Close()
			// 			delete(manager.clients, conn)
			// 		}
			// 	}
			// 	manager.mutex.Unlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (manager *WsManager) Broadcast(message []byte) {
	manager.broadcast <- message
}
