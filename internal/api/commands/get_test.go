package commands

import (
	"testing"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

func TestGetReturnsNullBulkStringForMissingKey(t *testing.T) {
	eng := engine.NewEngine()
	go eng.Run(t.Context())

	response, err := NewGet(eng).Execute(t.Context(), []resp.Element{
		{Type: resp.BulkString, Value: []byte("missing")},
	})
	if err != nil {
		t.Fatalf("executing GET: %v", err)
	}
	if response.Type != resp.BulkString || !response.Null {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestGetValidate(t *testing.T) {
	tests := []struct {
		name      string
		arguments []resp.Element
		wantError bool
	}{
		{
			name:      "one key",
			arguments: []resp.Element{{Type: resp.BulkString, Value: []byte("key")}},
		},
		{name: "no key", wantError: true},
		{
			name: "too many keys",
			arguments: []resp.Element{
				{Type: resp.BulkString, Value: []byte("one")},
				{Type: resp.BulkString, Value: []byte("two")},
			},
			wantError: true,
		},
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
			err := (Get{}).Validate(tt.arguments)
			if tt.wantError && err == nil {
				t.Fatal("expected arguments to be rejected")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected arguments to be accepted: %v", err)
			}
		})
	}
}
