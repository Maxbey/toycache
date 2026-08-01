package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"strconv"
)

const (
	maxBulkStringLength = 1 << 20
	maxArrayElements    = 1024
	maxNestingDepth     = 32
	maxLineLength       = 64 << 10
)

type Reader struct {
	reader *bufio.Reader
}

func NewReader(reader *bufio.Reader) Reader {
	return Reader{reader: reader}
}

func (d Reader) Read() (Element, error) {
	return d.read(0)
}

func (d Reader) read(depth int) (Element, error) {
	b, err := d.reader.ReadByte()
	if err != nil {
		return Element{}, fmt.Errorf("error reading buffer: %v", err)
	}

	t := dataType(b)
	switch t {
	case Error, String:
		el, err := d.readSimple(t)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing simple element: %v", err)
		}
		return el, nil
	case Integer:
		el, err := d.readInteger(t)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing integer: %v", err)
		}
		return el, nil
	case BulkString:
		el, err := d.readBulkString(t)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing bulk string: %v", err)
		}
		return el, nil
	case Array:
		if depth >= maxNestingDepth {
			return Element{}, fmt.Errorf("maximum RESP nesting depth exceeded: %d", maxNestingDepth)
		}

		el, err := d.readArray(t, depth+1)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing array: %v", err)
		}
		return el, nil
	default:
		return Element{}, fmt.Errorf("cannot parse invalid data type: %d", t)
	}
}

func (d Reader) readSimple(t dataType) (Element, error) {
	value, err := d.readUntilTerminator()
	if err != nil {
		return Element{}, fmt.Errorf("error reading simple element: %v", err)
	}

	return Element{Type: t, Value: value}, nil
}

func (d Reader) readInteger(t dataType) (Element, error) {
	value, err := d.readUntilTerminator()
	if err != nil {
		return Element{}, fmt.Errorf("error reading integer: %v", err)
	}

	if _, err := strconv.ParseInt(string(value), 10, 64); err != nil {
		return Element{}, fmt.Errorf("invalid RESP integer %q: %v", value, err)
	}

	return Element{Type: t, Value: value}, nil
}

func (d Reader) readBulkString(t dataType) (Element, error) {
	length, err := d.readLengthPrefix()
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

	value, err := d.readExact(length)
	if err != nil {
		return Element{}, err
	}

	return Element{Type: t, Value: value}, nil
}

func (d Reader) readArray(t dataType, depth int) (Element, error) {
	length, err := d.readLengthPrefix()
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
		child, err := d.read(depth)
		if err != nil {
			return Element{}, fmt.Errorf("error parsing array element %d: %v", i, err)
		}

		elements = append(elements, child)
	}

	el.Elements = elements

	return el, nil
}

func (d Reader) readUntilTerminator() ([]byte, error) {
	buf := make([]byte, 0, 8)

	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("error reading buffer: %v", err)
		}

		terminated, err := d.isTerminated(b)
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

func (d Reader) readExact(length int) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("cannot parse exact stream portion with negative length: %d", length)
	}

	buf := make([]byte, length, length)
	n, err := io.ReadFull(d.reader, buf)
	if err != nil {
		return nil, fmt.Errorf("error reading buffer: %v", err)
	}

	if n != length {
		return nil, fmt.Errorf("incomplete buffer read: got %d, want %d", length, n)
	}

	b, err := d.reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("error reading buffer: %v", err)
	}

	terminated, err := d.isTerminated(b)
	if err != nil {
		return nil, fmt.Errorf("statement is not terminated properly: %v", err)
	}
	if !terminated {
		return nil, fmt.Errorf("statement is not terminated properly: expected \\r\\n after %d payload bytes", length)
	}

	return buf, nil
}

func (d Reader) readLengthPrefix() (int, error) {
	value, err := d.readUntilTerminator()
	if err != nil {
		return 0, fmt.Errorf("unable to read prefix length: %v", err)
	}

	length, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, fmt.Errorf("unable to parse prefix length: %v", err)
	}

	return length, nil
}

func (d Reader) isTerminated(b byte) (bool, error) {
	if b == '\n' {
		return false, errors.New("encountered \\n without preceding \\r")
	}
	if b != '\r' {
		return false, nil
	}

	b, err := d.reader.ReadByte()
	if err != nil {
		return false, fmt.Errorf("error reading buffer: %v", err)
	}

	if b != '\n' {
		return false, fmt.Errorf("encountered %q after \\r parsing the element", b)
	}

	return true, nil
}
