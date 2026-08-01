package resp

import (
	"bufio"
	"bytes"
	"testing"
)

func TestWriterWritesError(t *testing.T) {
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	respWriter := NewWriter(writer)

	err := respWriter.Write(Element{
		Type:  Error,
		Value: []byte("ERR unknown command"),
	})
	if err != nil {
		t.Fatalf("expected error element to be written: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("expected writer to flush: %v", err)
	}

	if got, want := output.String(), "-ERR unknown command\r\n"; got != want {
		t.Fatalf("unexpected encoding: got %q, want %q", got, want)
	}
}

func TestWriterWritesString(t *testing.T) {
	if got, want := writeElement(t, Element{Type: String, Value: []byte("OK")}), "+OK\r\n"; got != want {
		t.Fatalf("unexpected encoding: got %q, want %q", got, want)
	}
}

func TestWriterWritesInteger(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "positive", value: "42", want: ":42\r\n"},
		{name: "negative", value: "-42", want: ":-42\r\n"},
		{name: "zero", value: "0", want: ":0\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := writeElement(t, Element{Type: Integer, Value: []byte(tt.value)})
			if got != tt.want {
				t.Fatalf("unexpected encoding: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriterWritesBulkString(t *testing.T) {
	tests := []struct {
		name    string
		element Element
		want    string
	}{
		{
			name:    "value",
			element: Element{Type: BulkString, Value: []byte("hello")},
			want:    "$5\r\nhello\r\n",
		},
		{
			name:    "empty",
			element: Element{Type: BulkString, Value: []byte{}},
			want:    "$0\r\n\r\n",
		},
		{
			name:    "binary safe",
			element: Element{Type: BulkString, Value: []byte{'a', '\r', '\n', 0, 'b'}},
			want:    "$5\r\na\r\n\x00b\r\n",
		},
		{
			name:    "null",
			element: Element{Type: BulkString, Null: true},
			want:    "$-1\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := writeElement(t, tt.element); got != tt.want {
				t.Fatalf("unexpected encoding: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriterWritesArray(t *testing.T) {
	tests := []struct {
		name    string
		element Element
		want    string
	}{
		{
			name: "mixed elements",
			element: Element{
				Type: Array,
				Elements: []Element{
					{Type: String, Value: []byte("OK")},
					{Type: Integer, Value: []byte("42")},
					{Type: BulkString, Value: []byte("hello")},
				},
			},
			want: "*3\r\n+OK\r\n:42\r\n$5\r\nhello\r\n",
		},
		{
			name:    "empty",
			element: Element{Type: Array},
			want:    "*0\r\n",
		},
		{
			name:    "null",
			element: Element{Type: Array, Null: true},
			want:    "*-1\r\n",
		},
		{
			name: "nested",
			element: Element{
				Type: Array,
				Elements: []Element{
					{
						Type: Array,
						Elements: []Element{
							{Type: Integer, Value: []byte("1")},
						},
					},
				},
			},
			want: "*1\r\n*1\r\n:1\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := writeElement(t, tt.element); got != tt.want {
				t.Fatalf("unexpected encoding: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriterRejectsInvalidError(t *testing.T) {
	tests := []struct {
		name    string
		element Element
	}{
		{name: "carriage return", element: Element{Type: Error, Value: []byte("ERR bad\rvalue")}},
		{name: "line feed", element: Element{Type: Error, Value: []byte("ERR bad\nvalue")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := NewWriter(bufio.NewWriter(&bytes.Buffer{}))
			if err := writer.Write(tt.element); err == nil {
				t.Fatal("expected invalid error element to be rejected")
			}
		})
	}
}

func TestWriterRejectsInvalidString(t *testing.T) {
	for _, value := range []string{"bad\rvalue", "bad\nvalue"} {
		writer := NewWriter(bufio.NewWriter(&bytes.Buffer{}))
		if err := writer.Write(Element{Type: String, Value: []byte(value)}); err == nil {
			t.Fatalf("expected invalid string %q to be rejected", value)
		}
	}
}

func TestWriterRejectsInvalidInteger(t *testing.T) {
	for _, value := range []string{"", "abc", "1.5", "9223372036854775808"} {
		writer := NewWriter(bufio.NewWriter(&bytes.Buffer{}))
		if err := writer.Write(Element{Type: Integer, Value: []byte(value)}); err == nil {
			t.Fatalf("expected invalid integer %q to be rejected", value)
		}
	}
}

func TestWriterRejectsExcessiveArrayNesting(t *testing.T) {
	element := Element{Type: Integer, Value: []byte("1")}
	for range maxNestingDepth + 1 {
		element = Element{Type: Array, Elements: []Element{element}}
	}

	writer := NewWriter(bufio.NewWriter(&bytes.Buffer{}))
	if err := writer.Write(element); err == nil {
		t.Fatal("expected excessive array nesting to be rejected")
	}
}

func TestWriterRejectsUnsupportedType(t *testing.T) {
	writer := NewWriter(bufio.NewWriter(&bytes.Buffer{}))
	if err := writer.Write(Element{Type: dataType('?')}); err == nil {
		t.Fatal("expected unsupported type to be rejected")
	}
}

func writeElement(t *testing.T, element Element) string {
	t.Helper()

	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if err := NewWriter(writer).Write(element); err != nil {
		t.Fatalf("writing element: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flushing element: %v", err)
	}

	return output.String()
}
