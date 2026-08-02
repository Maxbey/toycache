package api

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Maxbey/toycache/internal/engine"
)

type countingWriter struct {
	bytes.Buffer
	writes int
}

func (w *countingWriter) Write(buffer []byte) (int, error) {
	w.writes++
	return w.Buffer.Write(buffer)
}

func TestHandlerBatchesBufferedCommands(t *testing.T) {
	output := &countingWriter{}
	readWriter := bufio.NewReadWriter(
		bufio.NewReader(strings.NewReader(strings.Repeat("*1\r\n$4\r\nPING\r\n", maxBatchSize+1))),
		bufio.NewWriter(output),
	)

	if err := NewHandler(engine.NewEngine()).Handle(t.Context(), readWriter); err != nil {
		t.Fatalf("handling pipelined commands: %v", err)
	}
	if count := strings.Count(output.String(), "+PONG\r\n"); count != maxBatchSize+1 {
		t.Fatalf("unexpected response count: got %d, want %d", count, maxBatchSize+1)
	}
	if output.writes != 3 {
		t.Fatalf("unexpected underlying write count: got %d, want 3", output.writes)
	}
}

func TestHandlerFlushesBeforeReadingIncompleteNextCommand(t *testing.T) {
	serverConnection, clientConnection := net.Pipe()
	t.Cleanup(func() {
		serverConnection.Close()
		clientConnection.Close()
	})

	handler := NewHandler(engine.NewEngine())
	done := make(chan error, 1)
	go func() {
		done <- handler.Handle(t.Context(), bufio.NewReadWriter(
			bufio.NewReader(serverConnection),
			bufio.NewWriter(serverConnection),
		))
	}()

	if _, err := io.WriteString(clientConnection, "*1\r\n$4\r\nPING\r\n*1\r\n$4\r\nPI"); err != nil {
		t.Fatalf("writing complete and partial commands: %v", err)
	}
	if err := clientConnection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("setting read deadline: %v", err)
	}

	response := make([]byte, len("+PONG\r\n"))
	if _, err := io.ReadFull(clientConnection, response); err != nil {
		t.Fatalf("reading first response before completing second command: %v", err)
	}
	if string(response) != "+PONG\r\n" {
		t.Fatalf("unexpected first response: %q", response)
	}

	if _, err := io.WriteString(clientConnection, "NG\r\n"); err != nil {
		t.Fatalf("finishing second command: %v", err)
	}
	if _, err := io.ReadFull(clientConnection, response); err != nil {
		t.Fatalf("reading second response: %v", err)
	}
	if string(response) != "+PONG\r\n" {
		t.Fatalf("unexpected second response: %q", response)
	}

	clientConnection.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not stop after connection closed")
	}
}
