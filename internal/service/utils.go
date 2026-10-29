package service

import "strings"

const NAMESPACE_METHOD_PREFIX = "cerera"

func ParseMethod(method string) (string, string) {
	parts := strings.Split(method, ".")
	if parts[0] == NAMESPACE_METHOD_PREFIX && len(parts) == 3 {
		return parts[1], parts[2]
	}
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return method, method
}

func ParseMethodBytes(method string) []byte {
	parts := strings.Split(method, ".")
	if parts[0] == NAMESPACE_METHOD_PREFIX && len(parts) == 3 {
		return []byte(parts[1] + parts[2])
	}
	if len(parts) == 2 {
		return []byte(parts[0] + parts[1])
	}
	return []byte{}
}

var NamespaceToService = map[string]string{
	"account":     VAULT_SERVICE_NAME,
	"chain":       CHAIN_SERVICE_NAME,
	"pool":        POOL_SERVICE_NAME,
	"validator":   VALIDATOR_SERVICE_NAME,
	"ice":         ICE_SERVICE_NAME,
	"miner":       MINER_SERVICE_NAME,
	"transaction": VALIDATOR_SERVICE_NAME,
}

func ResolveServiceName(ns string) string {
	if name, ok := NamespaceToService[ns]; ok {
		return name
	}
	return ns
}
