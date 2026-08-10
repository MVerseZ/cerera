package network

import (
	"encoding/json"
	"time"

	"github.com/cerera/core/types"
)

const TIMEOUT = 5 * time.Second

func parseRequest(body []byte) (*types.Request, error) {
	var request types.Request
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, err
	} else {
		return &request, nil
	}
}

func constructResponse(rId int, data any) ([]byte, error) {
	response := types.Response{
		JSONRPC: "2.0",
		ID:      rId,
	}
	if err, ok := data.(error); ok && err != nil {
		response.Error = &types.Error{Code: -32603, Message: err.Error()}
	} else {
		response.Result = data
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	} else {
		return responseData, nil
	}
}
