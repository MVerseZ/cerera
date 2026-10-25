package network

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cerera/internal/service"
)

func TestParseRequestIDsAndValidation(t *testing.T) {
	for _, body := range []string{
		`{"jsonrpc":"2.0","method":"svc.ok","id":1}`,
		`{"jsonrpc":"2.0","method":"svc.ok","id":"client-1"}`,
		`{"jsonrpc":"2.0","method":"svc.ok","id":null}`,
		`{"jsonrpc":"2.0","method":"svc.ok"}`,
	} {
		if _, err := parseRequest([]byte(body)); err != nil {
			t.Errorf("parseRequest(%s): %v", body, err)
		}
	}
	for _, body := range []string{
		`{"jsonrpc":"1.0","method":"svc.ok","id":1}`,
		`{"jsonrpc":"2.0","method":"svc.ok","params":true,"id":1}`,
		`{"jsonrpc":"2.0","method":"svc.ok","id":false}`,
		`{"jsonrpc":"2.0","method":"svc.ok","id":1} {}`,
	} {
		if _, err := parseRequest([]byte(body)); err == nil {
			t.Errorf("parseRequest(%s) unexpectedly succeeded", body)
		}
	}
}

func TestHandleRequestJSONRPCAndNotification(t *testing.T) {
	registry, err := service.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(service.NewService("svc", map[string]service.RPCHandler{
		"value": func(context.Context, []any) (any, error) { return "ok", nil },
	})); err != nil {
		t.Fatal(err)
	}
	ctx := service.WithRegistry(context.Background(), registry)
	handler := HandleRequest(ctx)

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"svc.value","params":[],"id":"x"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var decoded map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != http.StatusOK || decoded["result"] != "ok" || decoded["id"] != "x" {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"svc.missing","id":2}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":-32601`) {
		t.Fatalf("expected method-not-found JSON-RPC error, got %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"svc.value"}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("notification must have empty response, got %d %s", response.Code, response.Body.String())
	}
}

func TestHandleRequestLimitsAndOrigins(t *testing.T) {
	handler := HandleRequest(context.Background(), []string{"https://app.example"})
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden origin, got %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://app.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("expected allowed origin preflight, got %d with origin %q", response.Code, response.Header().Get("Access-Control-Allow-Origin"))
	}

	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", maxRPCRequestBytes+1)))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected body limit response, got %d", response.Code)
	}
}

func TestHandleRequestPropagatesClientCancellation(t *testing.T) {
	registry, err := service.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	canceled := make(chan struct{})
	if err := registry.Register(service.NewService("svc", map[string]service.RPCHandler{
		"wait": func(ctx context.Context, _ []any) (any, error) {
			close(entered)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		},
	})); err != nil {
		t.Fatal(err)
	}
	handler := HandleRequest(service.WithRegistry(context.Background(), registry))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"svc.wait","id":1}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { handler.ServeHTTP(response, request); close(done) }()
	<-entered
	cancel()
	<-canceled
	<-done
}
