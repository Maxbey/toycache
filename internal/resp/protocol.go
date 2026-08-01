package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"strconv"
)

const (
	terminatorLength    = 2
	maxBulkStringLength = 1 << 20
	maxArrayElements    = 1024
	maxNestingDepth     = 32
	maxLineLength       = 64 << 10
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

type Parser struct {
	reader *bufio.Reader
}

func NewParser(reader *bufio.Reader) Parser {
	return Parser{reader: reader}
}

func (p Parser) Parse() (Element, error) {
	return p.parse(0)
}

func (p Parser) parse(depth int) (Element, error) {
	b, err := p.reader.ReadByte()
	if err != nil {
		return Element{}, fmt.Errorf("error reading buffer: %v", err)
	}

	t := dataType(b)
	switch t {
	case Error, String:
		el, err := p.parseSimple(t)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing simple element: %v", err)
		}
		return el, nil
	case Integer:
		el, err := p.parseInteger(t)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing integer: %v", err)
		}
		return el, nil
	case BulkString:
		el, err := p.parseBulkString(t)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing bulk string: %v", err)
		}
		return el, nil
	case Array:
		if depth >= maxNestingDepth {
			return Element{}, fmt.Errorf("maximum RESP nesting depth exceeded: %d", maxNestingDepth)
		}

		el, err := p.parseArray(t, depth+1)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing array: %v", err)
		}
		return el, nil
	default:
		return Element{}, fmt.Errorf("cannot parse invalid data type: %d", t)
	}
}

func (p Parser) parseSimple(t dataType) (Element, error) {
	value, err := p.readUntilTerminator()
	if err != nil {
		return Element{}, fmt.Errorf("error reading simple element: %v", err)
	}

	return Element{Type: t, Value: value}, nil
}

func (p Parser) parseInteger(t dataType) (Element, error) {
	value, err := p.readUntilTerminator()
	if err != nil {
		return Element{}, fmt.Errorf("error reading integer: %v", err)
	}

	if _, err := strconv.ParseInt(string(value), 10, 64); err != nil {
		return Element{}, fmt.Errorf("invalid RESP integer %q: %v", value, err)
	}

	return Element{Type: t, Value: value}, nil
}

func (p Parser) parseBulkString(t dataType) (Element, error) {
	length, err := p.readLengthPrefix()
	if err != nil {
		return Element{}, err
	}
	if length < -1 {
		return Element{}, fmt.Errorf("bulk string length cannot be less than -1: %d", length)
	}
	if length == -1 {
		return Element{Type: t, Null: true}, nil
	}
	if length > maxBulkStringLength {
		return Element{}, fmt.Errorf("bulk string length exceeds maximum of %d bytes: %d", maxBulkStringLength, length)
	}

	value, err := p.readExact(length)
	if err != nil {
		return Element{}, err
	}

	return Element{Type: t, Value: value}, nil
}

func (p Parser) parseArray(t dataType, depth int) (Element, error) {
	length, err := p.readLengthPrefix()
	if err != nil {
		return Element{}, err
	}
	if length < -1 {
		return Element{}, fmt.Errorf("array length cannot be less than -1: %d", length)
	}
	if length > maxArrayElements {
		return Element{}, fmt.Errorf("array length exceeds maximum of %d elements: %d", maxArrayElements, length)
	}

	el := Element{Type: t, Null: length == -1}
	if el.Null {
		return el, nil
	}

	elements := make([]Element, 0, length)
	for i := range length {
		child, err := p.parse(depth)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing array element %d: %v", i, err)
		}

		elements = append(elements, child)
	}

	el.Elements = elements

	return el, nil
}

func (p Parser) readUntilTerminator() ([]byte, error) {
	buf := make([]byte, 0, 8)

	for {
		b, err := p.reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("error reading buffer: %v", err)
		}

		terminated, err := p.isTerminated(b)
		if err != nil {
			return nil, fmt.Errorf("statement is not terminated properly: %v", err)
		}

		if terminated {
			return buf, nil
		}
		if len(buf) >= maxLineLength {
			return nil, fmt.Errorf("RESP line exceeds maximum of %d bytes", maxLineLength)
		}

		buf = append(buf, b)
	}
}

func (p Parser) readExact(length int) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("cannot parse exact stream portion with negative length: %d", length)
	}

	buf := make([]byte, length, length)
	n, err := io.ReadFull(p.reader, buf)
	if err != nil {
		return nil, fmt.Errorf("error reading buffer: %v", err)
	}

	if n != length {
		return nil, fmt.Errorf("incomplete buffer read: got %d, want %d", length, n)
	}

	b, err := p.reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("error reading buffer: %v", err)
	}

	terminated, err := p.isTerminated(b)
	if err != nil {
		return nil, fmt.Errorf("statement is not terminated properly: %v", err)
	}
	if !terminated {
		return nil, fmt.Errorf("statement is not terminated properly: expected \\r\\n after %d payload bytes", length)
	}

	return buf, nil
}

func (p Parser) readLengthPrefix() (int, error) {
	value, err := p.readUntilTerminator()
	if err != nil {
		return 0, fmt.Errorf("unable to read prefix length: %v", err)
	}

	length, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, fmt.Errorf("unable to parse prefix length: %v", err)
	}

	return length, nil
}

func (p Parser) isTerminated(b byte) (bool, error) {
	if b == '\n' {
		return false, errors.New("encountered \\n without preceding \\r")
	}
	if b != '\r' {
		return false, nil
	}

	b, err := p.reader.ReadByte()
	if err != nil {
		return false, fmt.Errorf("error reading buffer: %v", err)
	}

	if b != '\n' {
		return false, fmt.Errorf("encountered %q after \\r parsing the element", b)
	}

	return true, nil
}
