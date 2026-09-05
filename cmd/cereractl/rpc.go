package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
	ID      int             `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func rpcURL(httpPort int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/", httpPort)
}

func rpcCall(httpPort int, method string, params []any) (json.RawMessage, error) {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(rpcURL(httpPort), "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var envelope rpcResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", envelope.Error.Message)
	}
	return envelope.Result, nil
}

func queryNodeHealth(httpPort int) (height int, info json.RawMessage, err error) {
	heightRaw, err := rpcCall(httpPort, "cerera.chain.height", []any{})
	if err != nil {
		return 0, nil, err
	}
	if err := json.Unmarshal(heightRaw, &height); err != nil {
		return 0, nil, err
	}
	info, err = rpcCall(httpPort, "cerera.chain.getInfo", []any{})
	if err != nil {
		return height, nil, nil
	}
	return height, info, nil
}
