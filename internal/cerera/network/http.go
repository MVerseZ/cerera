package network

import "github.com/cerera/internal/cerera/service"

var Result interface{}

func Execute(method string, params []interface{}) interface{} {
	// Result = icenet.Execute(method, params)
	Result = service.Exec(method, params)
	return Result
}
