package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/btcsuite/websocket"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/types"
)

var handlerLogger = logger.Named("handler")

func HandleRequest(ctx context.Context) http.HandlerFunc { //, poa *dddddpoa.DDDDDPoa, m prometheus.Counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			// Обработка префлайт запроса

			w.Header().Set("Access-Control-Allow-Origin", "*") // Замените на нужный источник
			// w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
			w.Header().Set("Access-Control-Request-Headers", "X-Custom-Header")

			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "1800")
			w.Header().Set("Access-Control-Allow-Headers", "content-type")
			w.Header().Set("Access-Control-Allow-Methods", "PUT, POST, GET, DELETE, PATCH, OPTIONS")

			w.WriteHeader(http.StatusOK)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		var request types.Request
		err = json.Unmarshal(body, &request)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		// reg, err := service.GetRegistry()
		// if err != nil {
		// 	http.Error(w, "Service registry not available", http.StatusInternalServerError)
		// 	return
		// }
		var response = types.Response{
			Result: Execute(request.Method, request.Params),
			// Result: reg.Execute(request.Method, request.Params),
		}

		response.JSONRPC = "2.0"
		response.ID = request.ID

		responseData, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// w.Header().Set("Access-Control-Max-Age", "10")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Request-Headers", "X-Custom-Header")

		_, err = w.Write(responseData)
		// m.Inc()
		if err != nil {
			handlerLogger.Errorw("Failed to write response", "err", err)
		}

		// select {
		// case <-ctx.Done():
		// 	fmt.Println("Context is done.")
		// default:
		// 	fmt.Println("Context is still valid.")
		// }
	}
}

var wsManager = NewWsManager()

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWebSockerRequest(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// w.Header().Set("Access-Control-Allow-Origin", "*") // Замените на нужный источник
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Request-Headers", "X-Custom-Header")

		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "1800")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		w.Header().Set("Access-Control-Allow-Methods", "PUT, POST, GET, DELETE, PATCH, OPTIONS")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			handlerLogger.Errorw("Failed to upgrade WebSocket connection", "err", err)
			return
		}

		wsManager.register <- conn

		go func(conn *websocket.Conn) {
			defer func() {
				wsManager.unregister <- conn
			}()

			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					handlerLogger.Errorw("Failed to read message from WebSocket", "err", err)
					break
				}
				if string(message) == "ping" {
					conn.WriteJSON("pong")
				}
			}
		}(conn)
	}
}

func BroadCastWs(data []byte) {

	// var wst = GetTransport()
	// for i := 0; i < len(wst.wsListeners); i++ {
	// 	wst.wsListeners[i].WriteJSON(data)
	// }
}
