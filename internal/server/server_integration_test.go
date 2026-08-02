package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Maxbey/toycache/internal/api"
	"github.com/Maxbey/toycache/internal/engine"
	"github.com/redis/go-redis/v9"
)

func TestConnectionReturnsRESPErrorForMalformedInput(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	server := NewServer(Config{}, newAPIHandler(t.Context()))
	handlerDone := make(chan struct{})
	go func() {
		server.handle(t.Context(), serverConn)
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

	server := NewServer(Config{}, newAPIHandler(t.Context()))
	handlerDone := make(chan struct{})
	go func() {
		server.handle(t.Context(), serverConn)
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

func TestRedisClientCommands(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting listener: %v", err)
	}

	serverContext, stopServer := context.WithCancel(t.Context())
	server := NewServer(Config{MaxConnections: 1}, newAPIHandler(serverContext))
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

	getContext, cancelGet := context.WithTimeout(t.Context(), 2*time.Second)
	_, err = client.Get(getContext, "missing").Result()
	cancelGet()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("expected missing GET to return redis.Nil, got %v", err)
	}

	setContext, cancelSet := context.WithTimeout(t.Context(), 2*time.Second)
	err = client.Set(setContext, "key", "value", 0).Err()
	cancelSet()
	if err != nil {
		t.Fatalf("sending SET with go-redis: %v", err)
	}

	getContext, cancelGet = context.WithTimeout(t.Context(), 2*time.Second)
	value, err := client.Get(getContext, "key").Result()
	cancelGet()
	if err != nil {
		t.Fatalf("sending GET with go-redis: %v", err)
	}
	if value != "value" {
		t.Fatalf("unexpected GET response: got %q, want %q", value, "value")
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

func newAPIHandler(ctx context.Context) func(context.Context, *bufio.ReadWriter) error {
	eng := engine.NewEngine()
	go eng.Run(ctx)

	return api.NewHandler(eng).Handle
}
