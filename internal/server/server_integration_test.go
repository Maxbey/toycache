package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Maxbey/toycache/internal/api"
	"github.com/redis/go-redis/v9"
)

func TestConnectionReturnsRESPErrorForMalformedInput(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	server := NewServer(Config{}, api.Entrypoint)
	handlerDone := make(chan struct{})
	go func() {
		server.handle(serverConn)
		close(handlerDone)
	}()

	if err := clientConn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("setting client deadline: %v", err)
	}
	if _, err := io.WriteString(clientConn, "?\r\n"); err != nil {
		t.Fatalf("sending malformed RESP: %v", err)
	}

	response, err := bufio.NewReader(clientConn).ReadString('\n')
	if err != nil {
		t.Fatalf("reading RESP error response: %v", err)
	}
	if got, want := response, "-cannot parse invalid data type: 63\r\n"; got != want {
		t.Fatalf("unexpected response: got %q, want %q", got, want)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("connection handler did not finish after returning the error response")
	}
}

func TestConnectionExecutesPing(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	server := NewServer(Config{}, api.Entrypoint)
	handlerDone := make(chan struct{})
	go func() {
		server.handle(serverConn)
		close(handlerDone)
	}()

	if err := clientConn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("setting client deadline: %v", err)
	}
	if _, err := io.WriteString(clientConn, "*1\r\n$4\r\nPING\r\n"); err != nil {
		t.Fatalf("sending PING: %v", err)
	}

	response, err := bufio.NewReader(clientConn).ReadString('\n')
	if err != nil {
		t.Fatalf("reading PING response: %v", err)
	}
	if got, want := response, "+PONG\r\n"; got != want {
		t.Fatalf("unexpected response: got %q, want %q", got, want)
	}

	if err := clientConn.Close(); err != nil {
		t.Fatalf("closing client connection: %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("connection handler did not finish after the client disconnected")
	}
}

func TestRedisClientPing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting listener: %v", err)
	}

	server := NewServer(Config{MaxConnections: 1}, api.Entrypoint)
	serverContext, stopServer := context.WithCancel(t.Context())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.serve(serverContext, listener)
	}()

	client := redis.NewClient(&redis.Options{
		Addr:            listener.Addr().String(),
		Protocol:        2,
		DisableIdentity: true,
		DialTimeout:     time.Second,
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
	})
	t.Cleanup(func() {
		_ = client.Close()
		stopServer()
	})

	pingContext, cancelPing := context.WithTimeout(t.Context(), 2*time.Second)
	result, err := client.Ping(pingContext).Result()
	cancelPing()
	if err != nil {
		t.Fatalf("sending PING with go-redis: %v", err)
	}
	if result != "PONG" {
		t.Fatalf("unexpected PING response: got %q, want %q", result, "PONG")
	}

	if err := client.Close(); err != nil {
		t.Fatalf("closing go-redis client: %v", err)
	}
	stopServer()

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server stopped with an error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after cancellation")
	}
}
