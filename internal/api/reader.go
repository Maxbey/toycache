package api

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/Maxbey/toycache/internal/resp"
)

const initialRequestBufferSize = 16 << 10

type reader struct {
	reader *bufio.Reader
	buffer []byte
	cursor int
}

func newReader(source *bufio.Reader) *reader {
	return &reader{
		reader: source,
		buffer: make([]byte, 0, initialRequestBufferSize),
	}
}

// Read returns an element that may borrow r.buffer. The caller must finish
// using it before calling Read again.
func (r *reader) Read() (resp.Element, error) {
	for {
		element, available, err := r.TryRead()
		if err != nil {
			return resp.Element{}, err
		}
		if available {
			return element, nil
		}

		r.compact()
		r.grow()

		length := len(r.buffer)
		r.buffer = r.buffer[:cap(r.buffer)]
		n, readErr := r.reader.Read(r.buffer[length:])
		r.buffer = r.buffer[:length+n]
		if n > 0 {
			continue
		}
		if readErr != nil {
			return resp.Element{}, fmt.Errorf("reading request: %w", readErr)
		}

		return resp.Element{}, io.ErrNoProgress
	}
}

func (r *reader) TryRead() (resp.Element, bool, error) {
	element, next, err := resp.Parse(r.buffer, r.cursor)
	if err == nil {
		r.cursor = next
		return element, true, nil
	}
	if errors.Is(err, resp.ErrIncomplete) {
		return resp.Element{}, false, nil
	}

	return resp.Element{}, false, err
}

func (r *reader) compact() {
	if r.cursor == 0 {
		return
	}

	copy(r.buffer, r.buffer[r.cursor:])
	r.buffer = r.buffer[:len(r.buffer)-r.cursor]
	r.cursor = 0
}

func (r *reader) grow() {
	if len(r.buffer) < cap(r.buffer) {
		return
	}

	capacity := cap(r.buffer) * 2
	if capacity == 0 {
		capacity = initialRequestBufferSize
	}

	buffer := make([]byte, len(r.buffer), capacity)
	copy(buffer, r.buffer)
	r.buffer = buffer
}
