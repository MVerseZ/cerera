package service

import (
	"context"
	"testing"
)

func TestRegistryRejectsInvalidAndDuplicateServices(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(NewService(" ", nil)); err == nil {
		t.Fatal("expected empty name error")
	}
	if err := registry.Register(NewService("svc", nil)); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(NewService("svc", nil)); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRegistryDoesNotExposeRegisteredMethods(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]RPCHandler{"ok": func(context.Context, []any) (any, error) { return nil, nil }}
	if err := registry.Register(NewService("svc", methods)); err != nil {
		t.Fatal(err)
	}
	delete(methods, "ok")
	if _, ok := registry.Lookup("svc", "ok"); !ok {
		t.Fatal("registration retained caller-owned method map")
	}
	serviceCopy, ok := registry.GetService("svc")
	if !ok {
		t.Fatal("service not found")
	}
	delete(serviceCopy.Methods, "ok")
	if _, ok := registry.Lookup("svc", "ok"); !ok {
		t.Fatal("GetService exposed registry method map")
	}
}
