package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
)

const (
	protocol = "tcp"
)

type Config struct {
	Host           string
	Port           int
	MaxConnections int
}

type Server struct {
	cfg     Config
	handler func(context.Context, *bufio.ReadWriter) error
}

func NewServer(cfg Config, handler func(context.Context, *bufio.ReadWriter) error) Server {
	return Server{
		cfg:     cfg,
		handler: handler,
	}
}

func (s Server) Run(ctx context.Context) error {
	listener, err := net.Listen(protocol, fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port))
	if err != nil {
		return fmt.Errorf("error creating TCP listener: %v", err)
	}
	defer listener.Close()

	slog.Info("Started server", "host", s.cfg.Host, "port", s.cfg.Port)
	return s.serve(ctx, listener)
}

func (s Server) serve(ctx context.Context, listener net.Listener) error {
	stopClosingListener := context.AfterFunc(ctx, func() {
		_ = listener.Close()
	})
	defer stopClosingListener()

	slots := make(chan struct{}, s.cfg.MaxConnections)
	for {
		select {
		case <-ctx.Done():
			return nil
		case slots <- struct{}{}:
		}

		con, err := listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			<-slots
			if ctx.Err() != nil {
				return nil
			}

			return err
		}
		if err != nil {
			// Add retry?
			<-slots
			continue
		}

		go func(con net.Conn) {
			defer func() { <-slots }()

			s.handle(ctx, con)
		}(con)
	}
}

func (s Server) handle(ctx context.Context, con net.Conn) {
	defer con.Close()

	readWriter := bufio.NewReadWriter(
		bufio.NewReader(con),
		bufio.NewWriter(con),
	)
	if err := s.handler(ctx, readWriter); err != nil {
		slog.Warn("error handling the client connection", "error", err)
	}
}
