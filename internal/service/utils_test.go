package service

import (
	"context"
	"testing"
)

func TestParseAndResolve(t *testing.T) {
	svc, method := ParseMethod("cerera.chain.getBlock")
	if svc != "chain" || method != "getBlock" {
		t.Fatalf("%s %s", svc, method)
	}
	svc, method = ParseMethod("pool.submit")
	if svc != "pool" || method != "submit" {
		t.Fatalf("%s %s", svc, method)
	}
	svc, method = ParseMethod("plain")
	if svc != "plain" || method != "plain" {
		t.Fatalf("%s %s", svc, method)
	}
	svc, method = ParseMethod("cerera.only")
	if svc != "cerera" || method != "only" {
		t.Fatalf("%s %s", svc, method)
	}

	if string(ParseMethodBytes("cerera.chain.getBlock")) != "chaingetBlock" {
		t.Fatal(string(ParseMethodBytes("cerera.chain.getBlock")))
	}
	if string(ParseMethodBytes("pool.submit")) != "poolsubmit" {
		t.Fatal(string(ParseMethodBytes("pool.submit")))
	}
	if len(ParseMethodBytes("plain")) != 0 {
		t.Fatal("expected empty bytes")
	}

	if ResolveServiceName("chain") != CHAIN_SERVICE_NAME {
		t.Fatal(ResolveServiceName("chain"))
	}
	if ResolveServiceName("account") != VAULT_SERVICE_NAME || ResolveServiceName("transaction") != VALIDATOR_SERVICE_NAME {
		t.Fatal("namespace map")
	}
	if ResolveServiceName("custom") != "custom" {
		t.Fatal(ResolveServiceName("custom"))
	}
	if ICE_SERVICE_NAME == "" || MINER_SERVICE_NAME == "" || POOL_SERVICE_NAME == "" {
		t.Fatal("service name constants")
	}
}

func TestRegistryNilAndContext(t *testing.T) {
	var registry *Registry
	if _, ok := registry.Lookup("svc", "m"); ok {
		t.Fatal("nil lookup")
	}
	if _, ok := registry.GetService("svc"); ok {
		t.Fatal("nil get")
	}
	if err := registry.Register(NewService("svc", nil)); err == nil {
		t.Fatal("expected nil registry error")
	}

	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(nil); err == nil {
		t.Fatal("expected nil service error")
	}
	if _, ok := registry.Lookup("missing", "m"); ok {
		t.Fatal("missing service")
	}
	if _, ok := registry.GetService("missing"); ok {
		t.Fatal("missing service")
	}
	if err := registry.Register(NewService("svc", map[string]RPCHandler{})); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Lookup("svc", "missing"); ok {
		t.Fatal("missing method")
	}

	svc := NewService("named", nil)
	if svc.ServiceName() != "named" || svc.GetID() != 0 {
		t.Fatalf("%s %d", svc.ServiceName(), svc.GetID())
	}

	if _, ok := GetRegistryFromContext(context.Background()); ok {
		t.Fatal("empty context")
	}
	if _, ok := GetRegistryFromContext(context.WithValue(context.Background(), registryCtxKey, "nope")); ok {
		t.Fatal("wrong type")
	}
	ctx := WithRegistry(context.Background(), registry)
	got, ok := GetRegistryFromContext(ctx)
	if !ok || got != registry {
		t.Fatal("context registry")
	}
}
