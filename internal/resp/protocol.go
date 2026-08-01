package resp

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	terminatorLength    = 2
	maxBulkStringLength = 1 << 20
	maxArrayElements    = 1024
	maxNestingDepth     = 32
)

type dataType byte

const (
	String     dataType = '+'
	Error      dataType = '-'
	Integer    dataType = ':'
	BulkString dataType = '$'
	Array      dataType = '*'
)

type Element struct {
	Type     dataType
	Value    []byte
	Null     bool
	Elements []Element
}

func Parse(stream []byte, cursor int) (Element, int, error) {
	return parse(stream, cursor, 0)
}

func parse(stream []byte, cursor, depth int) (Element, int, error) {
	if cursor < 0 || cursor >= len(stream) {
		return Element{}, 0, fmt.Errorf("cannot parse element: stream cursor is out of range: %d", cursor)
	}

	t := dataType(stream[cursor])
	switch t {
	case Error, String:
		el, cursor, err := parseSimple(t, stream, cursor+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing simple element: %v", err)
		}
		return el, cursor, nil
	case Integer:
		el, cursor, err := parseInteger(t, stream, cursor+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing integer: %v", err)
		}
		return el, cursor, nil
	case BulkString:
		el, cursor, err := parseBulkString(t, stream, cursor+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing bulk string: %v", err)
		}
		return el, cursor, nil
	case Array:
		if depth >= maxNestingDepth {
			return Element{}, 0, fmt.Errorf("maximum RESP nesting depth exceeded: %d", maxNestingDepth)
		}

		el, cursor, err := parseArray(t, stream, cursor+1, depth+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing array: %v", err)
		}
		return el, cursor, nil
	default:
		return Element{}, 0, fmt.Errorf("cannot parse invalid data type: %d", t)
	}
}

func parseSimple(t dataType, stream []byte, cursor int) (Element, int, error) {
	value, cursor, err := readUntilTerminator(stream, cursor, nil)
	if err != nil {
		return Element{}, 0, fmt.Errorf("error reading simple element: %v", err)
	}

	return Element{Type: t, Value: value}, cursor, nil
}

func parseInteger(t dataType, stream []byte, cursor int) (Element, int, error) {
	value, cursor, err := readUntilTerminator(stream, cursor, nil)
	if err != nil {
		return Element{}, 0, fmt.Errorf("error reading integer: %v", err)
	}

	if _, err := strconv.ParseInt(string(value), 10, 64); err != nil {
		return Element{}, 0, fmt.Errorf("invalid RESP integer %q: %v", value, err)
	}

	return Element{Type: t, Value: value}, cursor, nil
}

func parseBulkString(t dataType, stream []byte, cursor int) (Element, int, error) {
	length, cursor, err := readLengthPrefix(stream, cursor)
	if err != nil {
		return Element{}, 0, err
	}
	if length < -1 {
		return Element{}, 0, fmt.Errorf("bulk string length cannot be less than -1: %d", length)
	}
	if length == -1 {
		return Element{Type: t, Null: true}, cursor, nil
	}
	if length > maxBulkStringLength {
		return Element{}, 0, fmt.Errorf("bulk string length exceeds maximum of %d bytes: %d", maxBulkStringLength, length)
	}

	value, cursor, err := readExact(stream, cursor, length)
	if err != nil {
		return Element{}, 0, err
	}

	return Element{Type: t, Value: value}, cursor, nil
}

func parseArray(t dataType, stream []byte, cursor, depth int) (Element, int, error) {
	length, cursor, err := readLengthPrefix(stream, cursor)
	if err != nil {
		return Element{}, 0, err
	}
	if length < -1 {
		return Element{}, 0, fmt.Errorf("array length cannot be less than -1: %d", length)
	}
	if length > maxArrayElements {
		return Element{}, 0, fmt.Errorf("array length exceeds maximum of %d elements: %d", maxArrayElements, length)
	}

	el := Element{Type: t, Null: length == -1}
	if el.Null {
		return el, cursor, nil
	}

	elements := make([]Element, 0, length)
	for i := range length {
		child, nextCursor, err := parse(stream, cursor, depth)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing array element %d: %v", i, err)
		}

		elements = append(elements, child)
		cursor = nextCursor
	}

	el.Elements = elements

	return el, cursor, nil
}

func readUntilTerminator(stream []byte, cursor int, buf []byte) ([]byte, int, error) {
	if cursor >= len(stream)-1 {
		return nil, 0, fmt.Errorf("cannot parse the length prefix: stream cursor is out of range: %d", cursor)
	}

	if buf == nil {
		buf = make([]byte, 0, 8)
	}
	for cursor < len(stream) {
		terminated, err := isTerminated(stream, cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("statement is not terminated properly: %v", err)
		}

		if terminated {
			return buf, cursor + terminatorLength, nil
		}

		buf = append(buf, stream[cursor])
		cursor += 1
	}

	return nil, 0, fmt.Errorf("stream is not terminated")
}

func readExact(stream []byte, cursor int, length int) ([]byte, int, error) {
	if cursor < 0 || cursor > len(stream) {
		return nil, 0, fmt.Errorf("cannot parse exact stream portion: cursor is out of range: %d", cursor)
	}
	if length < 0 {
		return nil, 0, fmt.Errorf("cannot parse exact stream portion with negative length: %d", length)
	}

	remaining := len(stream) - cursor
	if remaining < terminatorLength || length > remaining-terminatorLength {
		return nil, 0, fmt.Errorf("cannot parse exact stream portion: need %d payload bytes and a terminator, have %d bytes", length, remaining)
	}

	end := cursor + length
	value := stream[cursor:end]
	cursor = end
	terminated, err := isTerminated(stream, cursor)
	if err != nil {
		return nil, 0, fmt.Errorf("statement is not terminated properly: %v", err)
	}
	if !terminated {
		return nil, 0, fmt.Errorf("statement is not terminated properly: expected \\r\\n after %d payload bytes", length)
	}

	return value, cursor + terminatorLength, nil
}

func readLengthPrefix(stream []byte, cursor int) (int, int, error) {
	value, cursor, err := readUntilTerminator(stream, cursor, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to read prefix length: %v", err)
	}

	length, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, 0, fmt.Errorf("unable to parse prefix length: %v", err)
	}

	return length, cursor, nil
}

func isTerminated(stream []byte, cursor int) (bool, error) {
	if stream[cursor] == '\n' {
		return false, errors.New("encountered \\n without preceding \\r")
	}
	if stream[cursor] != '\r' {
		return false, nil
	}

	if cursor == len(stream)-1 {
		return false, errors.New("data stream interrupted with \\r with no \\n")
	}
	if stream[cursor+1] != '\n' {
		return false, fmt.Errorf("encountered %q after \\r parsing the element", stream[cursor+1])
	}

	return true, nil
}
