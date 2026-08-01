package api

import (
	"testing"

	"github.com/Maxbey/toycache/internal/resp"
)

func TestDispatcherDispatchesCaseInsensitively(t *testing.T) {
	got, err := NewDispatcher().Dispatch(commandRequest("ping"))
	if err != nil {
		t.Fatalf("dispatching PING: %v", err)
	}
	want := resp.Element{Type: resp.String, Value: []byte("PONG")}
	if got.Type != want.Type || string(got.Value) != string(want.Value) {
		t.Fatalf("unexpected response: got %#v, want %#v", got, want)
	}
}

func TestDispatcherRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name    string
		request resp.Element
	}{
		{name: "not an array", request: resp.Element{Type: resp.BulkString, Value: []byte("PING")}},
		{name: "null array", request: resp.Element{Type: resp.Array, Null: true}},
		{name: "empty array", request: resp.Element{Type: resp.Array}},
		{name: "unknown command", request: commandRequest("NOPE")},
		{name: "too many PING arguments", request: commandRequest("PING", "one", "two")},
		{
			name: "non-bulk argument",
			request: resp.Element{
				Type: resp.Array,
				Elements: []resp.Element{
					{Type: resp.BulkString, Value: []byte("PING")},
					{Type: resp.String, Value: []byte("hello")},
				},
			},
		},
	}

	dispatcher := NewDispatcher()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := dispatcher.Dispatch(tt.request); err == nil {
				t.Fatal("expected request to be rejected")
			}
		})
	}
}

func commandRequest(parts ...string) resp.Element {
	elements := make([]resp.Element, 0, len(parts))
	for _, part := range parts {
		elements = append(elements, resp.Element{Type: resp.BulkString, Value: []byte(part)})
	}

	return resp.Element{Type: resp.Array, Elements: elements}
}
