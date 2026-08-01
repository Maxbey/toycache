package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func newTestReader(stream string) *Reader {
	reader := NewReader(bufio.NewReader(strings.NewReader(stream)))
	return &reader
}

func mustRead(t *testing.T, reader *Reader) Element {
	t.Helper()

	element, err := reader.Read()
	if err != nil {
		t.Fatalf("expected element to parse: %v", err)
	}

	return element
}

func TestParseScalarElements(t *testing.T) {
	tests := []struct {
		name   string
		stream string
		typeID dataType
		value  string
	}{
		{name: "simple string", stream: "+OK\r\n", typeID: String, value: "OK"},
		{name: "empty simple string", stream: "+\r\n", typeID: String, value: ""},
		{name: "error", stream: "-ERR unknown command\r\n", typeID: Error, value: "ERR unknown command"},
		{name: "positive integer", stream: ":42\r\n", typeID: Integer, value: "42"},
		{name: "negative integer", stream: ":-42\r\n", typeID: Integer, value: "-42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			element := mustRead(t, newTestReader(tt.stream))
			if element.Type != tt.typeID {
				t.Fatalf("unexpected type: got %q, want %q", element.Type, tt.typeID)
			}
			if string(element.Value) != tt.value {
				t.Fatalf("unexpected value: got %q, want %q", element.Value, tt.value)
			}
		})
	}
}

func TestParseBulkStrings(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		element := mustRead(t, newTestReader("$5\r\nhello\r\n"))
		if element.Type != BulkString || element.Null || string(element.Value) != "hello" {
			t.Fatalf("unexpected bulk string: %+v", element)
		}
	})

	t.Run("empty", func(t *testing.T) {
		element := mustRead(t, newTestReader("$0\r\n\r\n"))
		if element.Null || len(element.Value) != 0 {
			t.Fatalf("expected non-null empty bulk string: %+v", element)
		}
	})

	t.Run("null", func(t *testing.T) {
		element := mustRead(t, newTestReader("$-1\r\n"))
		if !element.Null || element.Type != BulkString {
			t.Fatalf("expected null bulk string: %+v", element)
		}
	})

	t.Run("embedded CRLF", func(t *testing.T) {
		element := mustRead(t, newTestReader("$4\r\na\r\nb\r\n"))
		if !bytes.Equal(element.Value, []byte{'a', '\r', '\n', 'b'}) {
			t.Fatalf("unexpected payload: %q", element.Value)
		}
	})
}

func TestParseArrays(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		reader := newTestReader("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n")
		element := mustRead(t, reader)
		if element.Type != Array || element.Null || len(element.Elements) != 3 {
			t.Fatalf("unexpected array: %+v", element)
		}
		for i, want := range []string{"SET", "key", "value"} {
			if string(element.Elements[i].Value) != want {
				t.Fatalf("element %d: got %q, want %q", i, element.Elements[i].Value, want)
			}
		}
	})

	t.Run("empty", func(t *testing.T) {
		element := mustRead(t, newTestReader("*0\r\n"))
		if element.Null || len(element.Elements) != 0 {
			t.Fatalf("expected non-null empty array: %+v", element)
		}
	})

	t.Run("null", func(t *testing.T) {
		element := mustRead(t, newTestReader("*-1\r\n"))
		if !element.Null || element.Type != Array {
			t.Fatalf("expected null array: %+v", element)
		}
	})

	t.Run("mixed and nested", func(t *testing.T) {
		reader := newTestReader("*3\r\n+OK\r\n:2\r\n*1\r\n$3\r\nhey\r\n")
		element := mustRead(t, reader)
		if len(element.Elements) != 3 || len(element.Elements[2].Elements) != 1 {
			t.Fatalf("unexpected nested array: %+v", element)
		}
		if string(element.Elements[2].Elements[0].Value) != "hey" {
			t.Fatalf("unexpected nested value: %q", element.Elements[2].Elements[0].Value)
		}
	})
}

func TestReaderReadsPipelinedElements(t *testing.T) {
	reader := newTestReader("+OK\r\n:42\r\n")
	first := mustRead(t, reader)
	second := mustRead(t, reader)

	if string(first.Value) != "OK" || string(second.Value) != "42" {
		t.Fatalf("unexpected elements: first=%q second=%q", first.Value, second.Value)
	}
	if _, err := reader.Read(); err == nil {
		t.Fatal("expected EOF after both pipelined elements")
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name   string
		stream string
	}{
		{name: "empty input", stream: ""},
		{name: "unknown marker", stream: "?wat\r\n"},
		{name: "unterminated simple string", stream: "+hello"},
		{name: "invalid bulk length", stream: "$wat\r\n"},
		{name: "bulk length below null", stream: "$-2\r\n"},
		{name: "truncated bulk payload", stream: "$5\r\nhey\r\n"},
		{name: "missing bulk terminator", stream: "$3\r\nhey"},
		{name: "invalid bulk terminator", stream: "$3\r\nheyXX"},
		{name: "array length below null", stream: "*-2\r\n"},
		{name: "incomplete array", stream: "*2\r\n+OK\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newTestReader(tt.stream).Read(); err == nil {
				t.Fatalf("expected malformed input to be rejected: %q", tt.stream)
			}
		})
	}
}

func TestParseRejectsOversizedBulkString(t *testing.T) {
	stream := fmt.Sprintf("$%d\r\n", maxBulkStringLength+1)
	if _, err := newTestReader(stream).Read(); err == nil {
		t.Fatal("expected oversized bulk string to be rejected")
	}
}

func TestParseRejectsOversizedArray(t *testing.T) {
	stream := fmt.Sprintf("*%d\r\n", maxArrayElements+1)
	if _, err := newTestReader(stream).Read(); err == nil {
		t.Fatal("expected oversized array to be rejected")
	}
}

func TestParseNestingDepth(t *testing.T) {
	valid := strings.Repeat("*1\r\n", maxNestingDepth) + "+OK\r\n"
	if _, err := newTestReader(valid).Read(); err != nil {
		t.Fatalf("expected maximum nesting depth to be accepted: %v", err)
	}

	tooDeep := strings.Repeat("*1\r\n", maxNestingDepth+1) + "+OK\r\n"
	if _, err := newTestReader(tooDeep).Read(); err == nil {
		t.Fatal("expected excessive nesting depth to be rejected")
	}
}

func TestParseSimpleStringRequiresCRLF(t *testing.T) {
	if _, err := newTestReader("+hello\r\n").Read(); err != nil {
		t.Fatalf("expected CRLF-terminated simple string to be accepted: %v", err)
	}

	for _, stream := range []string{
		"+hello\n",
		"+hello\nworld\r\n",
		"+hello\rworld\r\n",
	} {
		if _, err := newTestReader(stream).Read(); err == nil {
			t.Fatalf("expected invalid line ending to be rejected: %q", stream)
		}
	}
}

func TestParseIntegerValidation(t *testing.T) {
	for _, stream := range []string{
		":0\r\n",
		":+42\r\n",
		":-42\r\n",
		":9223372036854775807\r\n",
		":-9223372036854775808\r\n",
	} {
		if _, err := newTestReader(stream).Read(); err != nil {
			t.Fatalf("expected valid integer to be accepted: %q: %v", stream, err)
		}
	}

	for _, stream := range []string{
		":\r\n",
		":banana\r\n",
		":1.5\r\n",
		":9223372036854775808\r\n",
		":-9223372036854775809\r\n",
	} {
		if _, err := newTestReader(stream).Read(); err == nil {
			t.Fatalf("expected invalid integer to be rejected: %q", stream)
		}
	}
}

func TestParseLineLengthLimit(t *testing.T) {
	atLimit := "+" + strings.Repeat("a", maxLineLength) + "\r\n"
	if _, err := newTestReader(atLimit).Read(); err != nil {
		t.Fatalf("expected line at maximum length to be accepted: %v", err)
	}

	overLimit := "+" + strings.Repeat("a", maxLineLength+1) + "\r\n"
	if _, err := newTestReader(overLimit).Read(); err == nil {
		t.Fatal("expected line over maximum length to be rejected")
	}
}
