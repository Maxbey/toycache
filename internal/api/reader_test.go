package api

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

type chunkReader struct {
	stream []byte
	size   int
}

func (r *chunkReader) Read(buffer []byte) (int, error) {
	if len(r.stream) == 0 {
		return 0, io.EOF
	}

	size := min(len(r.stream), r.size, len(buffer))
	copy(buffer, r.stream[:size])
	r.stream = r.stream[size:]

	return size, nil
}

func TestReaderHandlesFragmentedPipelinedElements(t *testing.T) {
	reader := newReader(bufio.NewReader(&chunkReader{
		stream: []byte("+OK\r\n:42\r\n"),
		size:   1,
	}))

	first, err := reader.Read()
	if err != nil {
		t.Fatalf("reading first element: %v", err)
	}
	if string(first.Value) != "OK" {
		t.Fatalf("unexpected first element: %q", first.Value)
	}
	second, err := reader.Read()
	if err != nil {
		t.Fatalf("reading second element: %v", err)
	}

	if string(second.Value) != "42" {
		t.Fatalf("unexpected second element: %q", second.Value)
	}
}

func TestReaderTryReadUsesOnlyBufferedInput(t *testing.T) {
	reader := newReader(bufio.NewReader(bytes.NewBufferString("+OK\r\n:42\r\n")))

	first, err := reader.Read()
	if err != nil {
		t.Fatalf("reading first element: %v", err)
	}
	second, available, err := reader.TryRead()
	if err != nil {
		t.Fatalf("trying second element: %v", err)
	}
	if !available {
		t.Fatal("second buffered element was not available")
	}
	if string(first.Value) != "OK" || string(second.Value) != "42" {
		t.Fatalf("unexpected elements: first=%q second=%q", first.Value, second.Value)
	}

	_, available, err = reader.TryRead()
	if err != nil {
		t.Fatalf("trying incomplete element: %v", err)
	}
	if available {
		t.Fatal("unexpected buffered element")
	}
}

func TestReaderGrowsForLargeElement(t *testing.T) {
	value := bytes.Repeat([]byte{'x'}, initialRequestBufferSize+1)
	stream := append([]byte("$16385\r\n"), value...)
	stream = append(stream, '\r', '\n')
	reader := newReader(bufio.NewReader(&chunkReader{
		stream: stream,
		size:   1024,
	}))

	element, err := reader.Read()
	if err != nil {
		t.Fatalf("reading large element: %v", err)
	}
	if !bytes.Equal(element.Value, value) {
		t.Fatalf("unexpected value length: got %d, want %d", len(element.Value), len(value))
	}
}
