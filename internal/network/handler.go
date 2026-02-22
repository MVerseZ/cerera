package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/btcsuite/websocket"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var handlerLogger = logger.Named("handler")

var (
	rpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of RPC requests by method",
		},
		[]string{"method"},
	)
	rpcRequestsDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_requests_duration_seconds",
			Help:    "RPC request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		},
		[]string{"method"},
	)
	rpcRequestsErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_errors_total",
			Help: "Total number of RPC request errors by method",
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(
		rpcRequestsTotal,
		rpcRequestsDurationSeconds,
		rpcRequestsErrorsTotal,
	)
}

func HandleRequest(ctx context.Context) http.HandlerFunc { //, poa *dddddpoa.DDDDDPoa, m prometheus.Counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		method := "unknown"

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
			rpcRequestsErrorsTotal.WithLabelValues("read_body").Inc()
			return
		}

		var request types.Request
		err = json.Unmarshal(body, &request)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			rpcRequestsErrorsTotal.WithLabelValues("parse_body").Inc()
			return
		}

		method = request.Method

		result := Execute(request.Method, request.Params)
		response := types.Response{
			JSONRPC: "2.0",
			ID:      request.ID,
		}
		if err, ok := result.(error); ok && err != nil {
			response.Error = &types.Error{Code: -32603, Message: err.Error()}
			// JSON-RPC 2.0: omit result when error is present
		} else {
			response.Result = result
		}

		responseData, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
			rpcRequestsErrorsTotal.WithLabelValues(method).Inc()
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
			rpcRequestsErrorsTotal.WithLabelValues(method).Inc()
		} else {
			// Record successful request
			duration := time.Since(startTime).Seconds()
			rpcRequestsTotal.WithLabelValues(method).Inc()
			rpcRequestsDurationSeconds.WithLabelValues(method).Observe(duration)
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
		// Для development: observer на :8080 подключается к нодам на :1337+
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
