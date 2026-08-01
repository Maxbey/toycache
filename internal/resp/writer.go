package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
)

type Writer struct {
	writer *bufio.Writer
}

func NewWriter(writer *bufio.Writer) Writer {
	return Writer{writer: writer}
}

func (e Writer) Write(element Element) error {
	return e.write(element, 0)
}

func (e Writer) write(element Element, depth int) error {
	switch element.Type {
	case String:
		return e.writeString(element)
	case Error:
		return e.writeError(element)
	case Integer:
		return e.writeInteger(element)
	case BulkString:
		return e.writeBulkString(element)
	case Array:
		if depth >= maxNestingDepth {
			return fmt.Errorf("maximum RESP nesting depth exceeded: %d", maxNestingDepth)
		}

		return e.writeArray(element, depth+1)
	default:
		return fmt.Errorf("cannot write unsupported data type: %q", element.Type)
	}
}

func (e Writer) writeString(element Element) error {
	if bytes.ContainsAny(element.Value, terminator) {
		return fmt.Errorf("RESP string cannot contain CR or LF")
	}

	return e.writeLine(String, element.Value, "string")
}

func (e Writer) writeError(element Element) error {
	if bytes.ContainsAny(element.Value, terminator) {
		return fmt.Errorf("RESP error cannot contain CR or LF")
	}

	return e.writeLine(Error, element.Value, "error")
}

func (e Writer) writeInteger(element Element) error {
	if _, err := strconv.ParseInt(string(element.Value), 10, 64); err != nil {
		return fmt.Errorf("invalid RESP integer %q: %w", element.Value, err)
	}

	return e.writeLine(Integer, element.Value, "integer")
}

func (e Writer) writeBulkString(element Element) error {
	if err := e.writer.WriteByte(byte(BulkString)); err != nil {
		return fmt.Errorf("error writing RESP bulk string type: %w", err)
	}

	if element.Null {
		if _, err := e.writer.WriteString("-1" + terminator); err != nil {
			return fmt.Errorf("error writing null RESP bulk string: %w", err)
		}

		return nil
	}

	if _, err := e.writer.WriteString(strconv.Itoa(len(element.Value))); err != nil {
		return fmt.Errorf("error writing RESP bulk string length: %w", err)
	}
	if _, err := e.writer.WriteString(terminator); err != nil {
		return fmt.Errorf("error writing RESP bulk string length terminator: %w", err)
	}
	if _, err := e.writer.Write(element.Value); err != nil {
		return fmt.Errorf("error writing RESP bulk string value: %w", err)
	}
	if _, err := e.writer.WriteString(terminator); err != nil {
		return fmt.Errorf("error writing RESP bulk string terminator: %w", err)
	}

	return nil
}

func (e Writer) writeArray(element Element, depth int) error {
	if err := e.writer.WriteByte(byte(Array)); err != nil {
		return fmt.Errorf("error writing RESP array type: %w", err)
	}

	if element.Null {
		if _, err := e.writer.WriteString("-1" + terminator); err != nil {
			return fmt.Errorf("error writing null RESP array: %w", err)
		}

		return nil
	}

	if _, err := e.writer.WriteString(strconv.Itoa(len(element.Elements))); err != nil {
		return fmt.Errorf("error writing RESP array length: %w", err)
	}
	if _, err := e.writer.WriteString(terminator); err != nil {
		return fmt.Errorf("error writing RESP array length terminator: %w", err)
	}

	for i, child := range element.Elements {
		if err := e.write(child, depth); err != nil {
			return fmt.Errorf("error writing RESP array element %d: %w", i, err)
		}
	}

	return nil
}

func (e Writer) writeLine(t dataType, value []byte, name string) error {
	if err := e.writer.WriteByte(byte(t)); err != nil {
		return fmt.Errorf("error writing RESP %s type: %w", name, err)
	}
	if _, err := e.writer.Write(value); err != nil {
		return fmt.Errorf("error writing RESP %s value: %w", name, err)
	}
	if _, err := e.writer.WriteString(terminator); err != nil {
		return fmt.Errorf("error writing RESP %s terminator: %w", name, err)
	}

	return nil
}
