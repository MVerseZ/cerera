package network

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/websocket"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/prometheus/client_golang/prometheus"
)

var handlerLogger = logger.Named("handler")

var (
	rpcRequestsTotal           = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "rpc_requests_total", Help: "Total number of RPC requests"}, []string{"method", "status"})
	rpcRequestsDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "rpc_requests_duration_seconds", Help: "RPC request duration in seconds", Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5}}, []string{"method", "status"})
	rpcRequestsErrorsTotal     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "rpc_requests_errors_total", Help: "Total number of RPC request errors"}, []string{"method", "status"})
)

func init() {
	prometheus.MustRegister(rpcRequestsTotal, rpcRequestsDurationSeconds, rpcRequestsErrorsTotal)
}

const maxRPCRequestBytes = 1 << 20

type rpcError struct {
	code int
	msg  string
}

func (e *rpcError) Error() string { return e.msg }

func HandleRequest(ctx context.Context, allowedOrigins ...[]string) http.HandlerFunc {
	origins := []string(nil)
	if len(allowedOrigins) > 0 {
		origins = allowedOrigins[0]
	}
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		metricMethod, metricStatus := "unknown", "invalid_request"
		defer func() {
			rpcRequestsTotal.WithLabelValues(metricMethod, metricStatus).Inc()
			rpcRequestsDurationSeconds.WithLabelValues(metricMethod, metricStatus).Observe(time.Since(started).Seconds())
			if metricStatus != "success" {
				rpcRequestsErrorsTotal.WithLabelValues(metricMethod, metricStatus).Inc()
			}
		}()

		if r.Method == http.MethodOptions {
			if !setCORSHeaders(w, r, origins) {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "1800")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !setCORSHeaders(w, r, origins) {
			http.Error(w, "origin is not allowed", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mediaType := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])
		if !strings.EqualFold(mediaType, "application/json") {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRPCRequestBytes))
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "request body exceeds 1 MiB", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
			}
			return
		}
		request, err := parseRequest(body)
		if err != nil {
			metricStatus = "invalid_request"
			if errors.Is(err, errInvalidRequest) {
				writeRPCError(w, nil, -32600, "Invalid Request")
			} else {
				writeRPCError(w, nil, -32700, "Parse error")
			}
			return
		}
		metricMethod = knownMetricMethod(ctx, request.Method)
		if request.Params == nil {
			request.Params = json.RawMessage("[]")
		}

		requestCtx, cancel := context.WithTimeout(r.Context(), TIMEOUT)
		stopServerCancel := context.AfterFunc(ctx, cancel)
		defer func() { stopServerCancel(); cancel() }()
		if registry, ok := service.GetRegistryFromContext(ctx); ok {
			requestCtx = service.WithRegistry(requestCtx, registry)
		}
		result, callErr := Execute(requestCtx, request.Method, request.Params)
		if callErr != nil {
			var coded *rpcError
			if errors.As(callErr, &coded) {
				metricStatus = "method_error"
			} else {
				metricStatus = "internal_error"
			}
			if len(request.ID) == 0 {
				return
			}
			if errors.As(callErr, &coded) {
				writeRPCError(w, request.ID, coded.code, coded.msg)
			} else {
				writeRPCError(w, request.ID, -32603, "Internal error")
			}
			return
		}
		if len(request.ID) == 0 {
			return
		}
		response, err := constructResponse(request.ID, result)
		if err != nil {
			metricStatus = "internal_error"
			writeRPCError(w, request.ID, -32603, "Internal error")
			return
		}
		metricStatus = "success"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(response); err != nil {
			handlerLogger.Warnw("Failed to write RPC response", "err", err)
		}
	}
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if id == nil {
		id = json.RawMessage("null")
	}
	body, err := constructErrorResponse(id, code, message)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func knownMetricMethod(ctx context.Context, method string) string {
	registry, ok := service.GetRegistryFromContext(ctx)
	if !ok {
		return "unknown"
	}
	cmp, name := service.ParseMethod(method)
	cmp = service.ResolveServiceName(cmp)
	if _, ok := registry.Lookup(cmp, name); !ok {
		return "unknown"
	}
	return method
}

func setCORSHeaders(w http.ResponseWriter, r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if isSameHostOrigin(origin, r.Host, r.TLS != nil) {
		return true
	}
	for _, candidate := range allowed {
		if candidate == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			return true
		}
	}
	return false
}

func isSameHostOrigin(origin, host string, secure bool) bool {
	separator := strings.Index(origin, "://")
	if separator < 0 {
		return false
	}
	scheme := strings.ToLower(origin[:separator])
	if (secure && scheme != "https") || (!secure && scheme != "http") {
		return false
	}
	authority := origin[separator+3:]
	if authority == "" || strings.ContainsAny(authority, "/?#@") {
		return false
	}
	return strings.EqualFold(authority, host)
}

var wsManager = NewWsManager()

func HandleWebSockerRequest(ctx context.Context, allowedOrigins ...[]string) http.HandlerFunc {
	origins := []string(nil)
	if len(allowedOrigins) > 0 {
		origins = allowedOrigins[0]
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			if isSameHostOrigin(origin, r.Host, r.TLS != nil) {
				return true
			}
			for _, candidate := range origins {
				if candidate == origin {
					return true
				}
			}
			return false
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			handlerLogger.Errorw("Failed to upgrade WebSocket connection", "err", err)
			return
		}
		wsManager.register <- conn
		go func(conn *websocket.Conn) {
			defer func() { wsManager.unregister <- conn; _ = conn.Close() }()
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if string(message) == "ping" {
					_ = conn.WriteJSON("pong")
				}
			}
		}(conn)
	}
}

func BroadCastWs(data []byte) {}
