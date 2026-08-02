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
