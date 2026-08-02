package commands

import (
	"bytes"
	"testing"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

func TestSetStoresValue(t *testing.T) {
	eng := engine.NewEngine()
	go eng.Run(t.Context())

	value := []byte("value")
	response, err := NewSet(eng).Execute(t.Context(), []resp.Element{
		{Type: resp.BulkString, Value: []byte("key")},
		{Type: resp.BulkString, Value: value},
	})
	if err != nil {
		t.Fatalf("executing SET: %v", err)
	}
	if response.Type != resp.String || !bytes.Equal(response.Value, []byte("OK")) {
		t.Fatalf("unexpected response: %#v", response)
	}
	copy(value, "xxxxx")

	value, found, err := eng.Get(t.Context(), []byte("key"))
	if err != nil {
		t.Fatalf("getting stored value: %v", err)
	}
	if !found || !bytes.Equal(value, []byte("value")) {
		t.Fatalf("unexpected stored value: value %q, found %t", value, found)
	}
}

func TestSetValidate(t *testing.T) {
	bulkString := func(value string) resp.Element {
		return resp.Element{Type: resp.BulkString, Value: []byte(value)}
	}

	tests := []struct {
		name      string
		arguments []resp.Element
		wantError bool
	}{
		{name: "key and value", arguments: []resp.Element{bulkString("key"), bulkString("value")}},
		{name: "no arguments", wantError: true},
		{name: "only key", arguments: []resp.Element{bulkString("key")}, wantError: true},
		{
			name:      "too many arguments",
			arguments: []resp.Element{bulkString("key"), bulkString("value"), bulkString("extra")},
			wantError: true,
		},
		{
			name:      "non-bulk key",
			arguments: []resp.Element{{Type: resp.String, Value: []byte("key")}, bulkString("value")},
			wantError: true,
		},
		{
			name:      "null key",
			arguments: []resp.Element{{Type: resp.BulkString, Null: true}, bulkString("value")},
			wantError: true,
		},
		{
			name:      "non-bulk value",
			arguments: []resp.Element{bulkString("key"), {Type: resp.String, Value: []byte("value")}},
			wantError: true,
		},
		{
			name:      "null value",
			arguments: []resp.Element{bulkString("key"), {Type: resp.BulkString, Null: true}},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (Set{}).Validate(tt.arguments)
			if tt.wantError && err == nil {
				t.Fatal("expected arguments to be rejected")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected arguments to be accepted: %v", err)
			}
		})
	}
}
