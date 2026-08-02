package resp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func mustParse(t *testing.T, stream string, cursor int) (Element, int) {
	t.Helper()

	element, nextCursor, err := Parse([]byte(stream), cursor)
	if err != nil {
		t.Fatalf("expected %q to parse: %v", stream, err)
	}

	return element, nextCursor
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
			element, cursor := mustParse(t, tt.stream, 0)
			if element.Type != tt.typeID {
				t.Fatalf("unexpected type: got %q, want %q", element.Type, tt.typeID)
			}
			if string(element.Value) != tt.value {
				t.Fatalf("unexpected value: got %q, want %q", element.Value, tt.value)
			}
			if cursor != len(tt.stream) {
				t.Fatalf("unexpected cursor: got %d, want %d", cursor, len(tt.stream))
			}
		})
	}
}

func TestParseBulkStrings(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		stream := "$5\r\nhello\r\n"
		element, cursor := mustParse(t, stream, 0)
		if element.Type != BulkString || element.Null || string(element.Value) != "hello" {
			t.Fatalf("unexpected bulk string: %+v", element)
		}
		if cursor != len(stream) {
			t.Fatalf("unexpected cursor: got %d, want %d", cursor, len(stream))
		}
	})

	t.Run("empty", func(t *testing.T) {
		element, cursor := mustParse(t, "$0\r\n\r\n", 0)
		if element.Null || len(element.Value) != 0 {
			t.Fatalf("expected non-null empty bulk string: %+v", element)
		}
		if cursor != len("$0\r\n\r\n") {
			t.Fatalf("unexpected cursor: %d", cursor)
		}
	})

	t.Run("null", func(t *testing.T) {
		element, cursor := mustParse(t, "$-1\r\n", 0)
		if !element.Null || element.Type != BulkString {
			t.Fatalf("expected null bulk string: %+v", element)
		}
		if cursor != len("$-1\r\n") {
			t.Fatalf("unexpected cursor: %d", cursor)
		}
	})

	t.Run("embedded CRLF", func(t *testing.T) {
		stream := "$4\r\na\r\nb\r\n"
		element, cursor := mustParse(t, stream, 0)
		if !bytes.Equal(element.Value, []byte{'a', '\r', '\n', 'b'}) {
			t.Fatalf("unexpected payload: %q", element.Value)
		}
		if cursor != len(stream) {
			t.Fatalf("unexpected cursor: got %d, want %d", cursor, len(stream))
		}
	})
}

func TestParseArrays(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		stream := "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
		element, cursor := mustParse(t, stream, 0)
		if element.Type != Array || element.Null || len(element.Elements) != 3 {
			t.Fatalf("unexpected array: %+v", element)
		}
		for i, want := range []string{"SET", "key", "value"} {
			if string(element.Elements[i].Value) != want {
				t.Fatalf("element %d: got %q, want %q", i, element.Elements[i].Value, want)
			}
		}
		if cursor != len(stream) {
			t.Fatalf("unexpected cursor: got %d, want %d", cursor, len(stream))
		}
	})

	t.Run("empty", func(t *testing.T) {
		element, _ := mustParse(t, "*0\r\n", 0)
		if element.Null || len(element.Elements) != 0 {
			t.Fatalf("expected non-null empty array: %+v", element)
		}
	})

	t.Run("null", func(t *testing.T) {
		element, _ := mustParse(t, "*-1\r\n", 0)
		if !element.Null || element.Type != Array {
			t.Fatalf("expected null array: %+v", element)
		}
	})

	t.Run("mixed and nested", func(t *testing.T) {
		stream := "*3\r\n+OK\r\n:2\r\n*1\r\n$3\r\nhey\r\n"
		element, cursor := mustParse(t, stream, 0)
		if len(element.Elements) != 3 || len(element.Elements[2].Elements) != 1 {
			t.Fatalf("unexpected nested array: %+v", element)
		}
		if string(element.Elements[2].Elements[0].Value) != "hey" {
			t.Fatalf("unexpected nested value: %q", element.Elements[2].Elements[0].Value)
		}
		if cursor != len(stream) {
			t.Fatalf("unexpected cursor: got %d, want %d", cursor, len(stream))
		}
	})
}

func TestParseUsesReturnedCursorForNextElement(t *testing.T) {
	stream := "+OK\r\n:42\r\n"
	first, cursor := mustParse(t, stream, 0)
	second, cursor := mustParse(t, stream, cursor)

	if string(first.Value) != "OK" || string(second.Value) != "42" {
		t.Fatalf("unexpected elements: first=%q second=%q", first.Value, second.Value)
	}
	if cursor != len(stream) {
		t.Fatalf("unexpected final cursor: got %d, want %d", cursor, len(stream))
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
			if _, _, err := Parse([]byte(tt.stream), 0); err == nil {
				t.Fatalf("expected malformed input to be rejected: %q", tt.stream)
			}
		})
	}
}

func TestParseRejectsOversizedBulkString(t *testing.T) {
	stream := []byte(fmt.Sprintf("$%d\r\n", maxBulkStringLength+1))

	if _, _, err := Parse(stream, 0); err == nil {
		t.Fatal("expected oversized bulk string to be rejected")
	}
}

func TestParseRejectsOversizedArray(t *testing.T) {
	stream := []byte(fmt.Sprintf("*%d\r\n", maxArrayElements+1))

	if _, _, err := Parse(stream, 0); err == nil {
		t.Fatal("expected oversized array to be rejected")
	}
}

func TestParseNestingDepth(t *testing.T) {
	valid := []byte(strings.Repeat("*1\r\n", maxNestingDepth) + "+OK\r\n")
	if _, _, err := Parse(valid, 0); err != nil {
		t.Fatalf("expected maximum nesting depth to be accepted: %v", err)
	}

	tooDeep := []byte(strings.Repeat("*1\r\n", maxNestingDepth+1) + "+OK\r\n")
	if _, _, err := Parse(tooDeep, 0); err == nil {
		t.Fatal("expected excessive nesting depth to be rejected")
	}
}

func TestParseSimpleStringRequiresCRLF(t *testing.T) {
	if _, _, err := Parse([]byte("+hello\r\n"), 0); err != nil {
		t.Fatalf("expected CRLF-terminated simple string to be accepted: %v", err)
	}

	for _, stream := range []string{
		"+hello\n",
		"+hello\nworld\r\n",
		"+hello\rworld\r\n",
	} {
		if _, _, err := Parse([]byte(stream), 0); err == nil {
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
		if _, _, err := Parse([]byte(stream), 0); err != nil {
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
		if _, _, err := Parse([]byte(stream), 0); err == nil {
			t.Fatalf("expected invalid integer to be rejected: %q", stream)
		}
	}
}

func TestParseRejectsNegativeCursor(t *testing.T) {
	if _, _, err := Parse([]byte("+OK\r\n"), -1); err == nil {
		t.Fatal("expected negative cursor to be rejected")
	}
}

func TestParseReportsEveryTruncatedPrefixAsIncomplete(t *testing.T) {
	stream := []byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n")
	for length := range len(stream) {
		_, cursor, err := Parse(stream[:length], 0)
		if !errors.Is(err, ErrIncomplete) {
			t.Fatalf("prefix length %d: expected incomplete error, got %v", length, err)
		}
		if cursor != 0 {
			t.Fatalf("prefix length %d: consumed %d bytes", length, cursor)
		}
	}
}

func TestParseValuesBorrowStream(t *testing.T) {
	stream := []byte("$5\r\nvalue\r\n")
	element, _, err := Parse(stream, 0)
	if err != nil {
		t.Fatalf("parsing bulk string: %v", err)
	}

	copy(stream[4:9], "xxxxx")
	if string(element.Value) != "xxxxx" {
		t.Fatalf("parsed value does not borrow input stream: %q", element.Value)
	}
}

func TestParseLineLengthLimit(t *testing.T) {
	atLimit := []byte("+" + strings.Repeat("a", maxLineLength) + "\r\n")
	if _, _, err := Parse(atLimit, 0); err != nil {
		t.Fatalf("expected line at maximum length to be accepted: %v", err)
	}

	overLimit := []byte("+" + strings.Repeat("a", maxLineLength+1) + "\r\n")
	if _, _, err := Parse(overLimit, 0); err == nil {
		t.Fatal("expected line over maximum length to be rejected")
	}
}
