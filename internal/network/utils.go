package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cerera/core/types"
)

const TIMEOUT = 5 * time.Second

var errInvalidRequest = fmt.Errorf("invalid JSON-RPC request")

func parseRequest(body []byte) (*types.Request, error) {
	var request types.Request
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&request); err != nil {
		var validJSON any
		if json.Unmarshal(body, &validJSON) == nil {
			return nil, fmt.Errorf("request fields have invalid types: %w", errInvalidRequest)
		}
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("request must contain a single JSON value: %w", errInvalidRequest)
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		return nil, fmt.Errorf("jsonrpc must be 2.0 and method must be a non-empty string: %w", errInvalidRequest)
	}
	if len(request.Params) > 0 {
		trimmed := bytes.TrimSpace(request.Params)
		if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
			return nil, fmt.Errorf("params must be an array or object: %w", errInvalidRequest)
		}
	}
	if len(request.ID) > 0 && !bytes.Equal(bytes.TrimSpace(request.ID), []byte("null")) {
		var id any
		if err := json.Unmarshal(request.ID, &id); err != nil {
			return nil, fmt.Errorf("invalid request id: %w", errInvalidRequest)
		}
		switch id.(type) {
		case string, float64:
		default:
			return nil, fmt.Errorf("request id must be a string, number, or null: %w", errInvalidRequest)
		}
	}
	return &request, nil
}

func constructResponse(rID json.RawMessage, data any) ([]byte, error) {
	result, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	response := types.Response{
		JSONRPC: "2.0",
		ID:      rID,
		Result:  result,
	}
	return json.Marshal(response)
}

func constructErrorResponse(rID json.RawMessage, code int, message string) ([]byte, error) {
	return json.Marshal(types.Response{
		JSONRPC: "2.0",
		ID:      rID,
		Error:   &types.Error{Code: code, Message: message},
	})
}
