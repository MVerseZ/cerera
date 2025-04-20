package types

type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	ID      int         `json:"id"`
	Error   *Error      `json:"error,omitempty"`
}
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
