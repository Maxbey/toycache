package engine

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestGet(t *testing.T) {
	eng := NewEngine()
	key := []byte{'k', 0, 'y'}
	eng.keyspace[string(key)] = []byte("value")
	go eng.Run(t.Context())

	value, found, err := eng.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("getting key: %v", err)
	}
	if !found || !bytes.Equal(value, []byte("value")) {
		t.Fatalf("unexpected result: value %q, found %t", value, found)
	}

	value[0] = 'X'
	value, found, err = eng.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("getting key again: %v", err)
	}
	if !found || !bytes.Equal(value, []byte("value")) {
		t.Fatalf("caller mutated engine-owned value: value %q, found %t", value, found)
	}
}

func TestGetReturnsMissing(t *testing.T) {
	eng := NewEngine()
	go eng.Run(t.Context())

	value, found, err := eng.Get(t.Context(), []byte("missing"))
	if err != nil {
		t.Fatalf("getting missing key: %v", err)
	}
	if found || value != nil {
		t.Fatalf("unexpected result: value %q, found %t", value, found)
	}
}

func TestGetHonorsCancellationBeforeSubmission(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, _, err := NewEngine().Get(ctx, []byte("key"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestSet(t *testing.T) {
	eng := NewEngine()
	go eng.Run(t.Context())

	key := []byte{'k', 0, 'y'}
	value := []byte{'v', 0, 'l'}
	if err := eng.Set(t.Context(), key, value); err != nil {
		t.Fatalf("setting key: %v", err)
	}

	key[0] = 'X'
	value[0] = 'X'
	got, found, err := eng.Get(t.Context(), []byte{'k', 0, 'y'})
	if err != nil {
		t.Fatalf("getting stored value: %v", err)
	}
	if !found || !bytes.Equal(got, []byte{'v', 0, 'l'}) {
		t.Fatalf("unexpected stored value: value %q, found %t", got, found)
	}

	if err := eng.Set(t.Context(), []byte{'k', 0, 'y'}, []byte("new")); err != nil {
		t.Fatalf("overwriting key: %v", err)
	}
	got, found, err = eng.Get(t.Context(), []byte{'k', 0, 'y'})
	if err != nil {
		t.Fatalf("getting overwritten value: %v", err)
	}
	if !found || !bytes.Equal(got, []byte("new")) {
		t.Fatalf("unexpected overwritten value: value %q, found %t", got, found)
	}
}

func TestSetHonorsCancellationBeforeSubmission(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	eng := NewEngine()
	if err := eng.Set(ctx, []byte("key"), []byte("value")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}

	go eng.Run(t.Context())
	value, found, err := eng.Get(t.Context(), []byte("key"))
	if err != nil {
		t.Fatalf("getting key after cancelled SET: %v", err)
	}
	if found || value != nil {
		t.Fatalf("cancelled SET modified keyspace: value %q, found %t", value, found)
	}
}
