package commands

import (
	"testing"

	"github.com/Maxbey/toycache/internal/resp"
)

func TestPing(t *testing.T) {
	tests := []struct {
		name      string
		arguments []resp.Element
		want      resp.Element
	}{
		{
			name: "without message",
			want: resp.Element{Type: resp.String, Value: []byte("PONG")},
		},
		{
			name: "with message",
			arguments: []resp.Element{
				{Type: resp.BulkString, Value: []byte("hello")},
			},
			want: resp.Element{Type: resp.BulkString, Value: []byte("hello")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (Ping{}).Execute(t.Context(), tt.arguments)
			if err != nil {
				t.Fatalf("executing PING: %v", err)
			}
			if got.Type != tt.want.Type || got.Null != tt.want.Null || string(got.Value) != string(tt.want.Value) {
				t.Fatalf("unexpected response: got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestPingValidate(t *testing.T) {
	tests := []struct {
		name      string
		arguments []resp.Element
		wantError bool
	}{
		{name: "no arguments"},
		{name: "one argument", arguments: []resp.Element{{Type: resp.BulkString, Value: []byte("hello")}}},
		{
			name: "too many arguments",
			arguments: []resp.Element{
				{Type: resp.BulkString, Value: []byte("one")},
				{Type: resp.BulkString, Value: []byte("two")},
			},
			wantError: true,
		},
		{
			name:      "non-bulk argument",
			arguments: []resp.Element{{Type: resp.String, Value: []byte("hello")}},
			wantError: true,
		},
		{
			name:      "null argument",
			arguments: []resp.Element{{Type: resp.BulkString, Null: true}},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (Ping{}).Validate(tt.arguments)
			if tt.wantError && err == nil {
				t.Fatal("expected arguments to be rejected")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected arguments to be accepted: %v", err)
			}
		})
	}
}
