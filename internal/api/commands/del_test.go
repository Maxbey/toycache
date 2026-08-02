package commands

import (
	"bytes"
	"testing"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

func TestDelDeletesKeys(t *testing.T) {
	eng := engine.NewEngine()
	go eng.Run(t.Context())

	if err := eng.Set(t.Context(), []byte("one"), []byte("value")); err != nil {
		t.Fatalf("setting key: %v", err)
	}

	response, err := NewDel(eng).Execute(t.Context(), []resp.Element{
		{Type: resp.BulkString, Value: []byte("one")},
		{Type: resp.BulkString, Value: []byte("missing")},
	})
	if err != nil {
		t.Fatalf("executing DEL: %v", err)
	}
	if response.Type != resp.Integer || !bytes.Equal(response.Value, []byte("1")) {
		t.Fatalf("unexpected response: %#v", response)
	}

	value, found, err := eng.Get(t.Context(), []byte("one"))
	if err != nil {
		t.Fatalf("getting deleted key: %v", err)
	}
	if found || value != nil {
		t.Fatalf("deleted key remains: value %q, found %t", value, found)
	}
}

func TestDelValidate(t *testing.T) {
	tests := []struct {
		name      string
		arguments []resp.Element
		wantError bool
	}{
		{
			name:      "one key",
			arguments: []resp.Element{{Type: resp.BulkString, Value: []byte("key")}},
		},
		{
			name: "multiple keys",
			arguments: []resp.Element{
				{Type: resp.BulkString, Value: []byte("one")},
				{Type: resp.BulkString, Value: []byte("two")},
			},
		},
		{name: "no keys", wantError: true},
		{
			name:      "non-bulk key",
			arguments: []resp.Element{{Type: resp.String, Value: []byte("key")}},
			wantError: true,
		},
		{
			name:      "null key",
			arguments: []resp.Element{{Type: resp.BulkString, Null: true}},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (Del{}).Validate(tt.arguments)
			if tt.wantError && err == nil {
				t.Fatal("expected arguments to be rejected")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected arguments to be accepted: %v", err)
			}
		})
	}
}
