package resp

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	maxBulkStringLength = 1 << 20
	maxArrayElements    = 1024
	maxNestingDepth     = 32
	maxLineLength       = 64 << 10
)

var ErrIncomplete = errors.New("incomplete RESP element")

// Parse returns elements whose Value slices borrow stream. Callers must finish
// using the element before modifying or reusing stream.
func Parse(stream []byte, cursor int) (Element, int, error) {
	return parse(stream, cursor, 0)
}

func parse(stream []byte, cursor, depth int) (Element, int, error) {
	if cursor < 0 || cursor > len(stream) {
		return Element{}, 0, fmt.Errorf("cannot parse element: stream cursor is out of range: %d", cursor)
	}
	if cursor == len(stream) {
		return Element{}, 0, ErrIncomplete
	}

	t := dataType(stream[cursor])
	switch t {
	case Error, String:
		el, next, err := parseSimple(t, stream, cursor+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing simple element: %w", err)
		}
		return el, next, nil
	case Integer:
		el, next, err := parseInteger(t, stream, cursor+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing integer: %w", err)
		}
		return el, next, nil
	case BulkString:
		el, next, err := parseBulkString(t, stream, cursor+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing bulk string: %w", err)
		}
		return el, next, nil
	case Array:
		if depth >= maxNestingDepth {
			return Element{}, 0, fmt.Errorf("maximum RESP nesting depth exceeded: %d", maxNestingDepth)
		}

		el, next, err := parseArray(t, stream, cursor+1, depth+1)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing array: %w", err)
		}
		return el, next, nil
	default:
		return Element{}, 0, fmt.Errorf("cannot parse invalid data type: %d", t)
	}
}

func parseSimple(t dataType, stream []byte, cursor int) (Element, int, error) {
	value, next, err := readUntilTerminator(stream, cursor)
	if err != nil {
		return Element{}, 0, fmt.Errorf("error reading simple element: %w", err)
	}

	return Element{Type: t, Value: value}, next, nil
}

func parseInteger(t dataType, stream []byte, cursor int) (Element, int, error) {
	value, next, err := readUntilTerminator(stream, cursor)
	if err != nil {
		return Element{}, 0, fmt.Errorf("error reading integer: %w", err)
	}

	if _, err := strconv.ParseInt(string(value), 10, 64); err != nil {
		return Element{}, 0, fmt.Errorf("invalid RESP integer %q: %w", value, err)
	}

	return Element{Type: t, Value: value}, next, nil
}

func parseBulkString(t dataType, stream []byte, cursor int) (Element, int, error) {
	length, next, err := readLengthPrefix(stream, cursor)
	if err != nil {
		return Element{}, 0, err
	}
	if length < -1 {
		return Element{}, 0, fmt.Errorf("bulk string length cannot be less than -1: %d", length)
	}
	if length == -1 {
		return Element{Type: t, Null: true}, next, nil
	}
	if length > maxBulkStringLength {
		return Element{}, 0, fmt.Errorf("bulk string length exceeds maximum of %d bytes: %d", maxBulkStringLength, length)
	}

	value, next, err := readExact(stream, next, length)
	if err != nil {
		return Element{}, 0, err
	}

	return Element{Type: t, Value: value}, next, nil
}

func parseArray(t dataType, stream []byte, cursor, depth int) (Element, int, error) {
	length, next, err := readLengthPrefix(stream, cursor)
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
		return el, next, nil
	}

	elements := make([]Element, 0, length)
	for i := range length {
		child, childNext, err := parse(stream, next, depth)
		if err != nil {
			return Element{}, 0, fmt.Errorf("error parsing array element %d: %w", i, err)
		}

		elements = append(elements, child)
		next = childNext
	}

	el.Elements = elements

	return el, next, nil
}

func readUntilTerminator(stream []byte, cursor int) ([]byte, int, error) {
	if cursor < 0 || cursor > len(stream) {
		return nil, 0, fmt.Errorf("cannot read RESP line: stream cursor is out of range: %d", cursor)
	}

	start := cursor
	for cursor < len(stream) {
		if stream[cursor] == '\n' {
			return nil, 0, errors.New("encountered \\n without preceding \\r")
		}
		if stream[cursor] == '\r' {
			if cursor+1 >= len(stream) {
				return nil, 0, ErrIncomplete
			}
			if stream[cursor+1] != '\n' {
				return nil, 0, fmt.Errorf("encountered %q after \\r parsing the element", stream[cursor+1])
			}

			return stream[start:cursor], cursor + terminatorLength, nil
		}
		if cursor-start >= maxLineLength {
			return nil, 0, fmt.Errorf("RESP line exceeds maximum of %d bytes", maxLineLength)
		}

		cursor++
	}

	return nil, 0, ErrIncomplete
}

func readExact(stream []byte, cursor, length int) ([]byte, int, error) {
	if cursor < 0 || cursor > len(stream) {
		return nil, 0, fmt.Errorf("cannot parse exact stream portion: cursor is out of range: %d", cursor)
	}
	if length < 0 {
		return nil, 0, fmt.Errorf("cannot parse exact stream portion with negative length: %d", length)
	}

	remaining := len(stream) - cursor
	if remaining < length+terminatorLength {
		return nil, 0, ErrIncomplete
	}

	end := cursor + length
	if stream[end] != '\r' || stream[end+1] != '\n' {
		return nil, 0, fmt.Errorf("statement is not terminated properly: expected \\r\\n after %d payload bytes", length)
	}

	return stream[cursor:end], end + terminatorLength, nil
}

func readLengthPrefix(stream []byte, cursor int) (int, int, error) {
	value, next, err := readUntilTerminator(stream, cursor)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to read prefix length: %w", err)
	}

	length, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, 0, fmt.Errorf("unable to parse prefix length: %w", err)
	}

	return length, next, nil
}
