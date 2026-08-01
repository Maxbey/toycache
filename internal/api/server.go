package api

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
	handler func(*bufio.Reader) error
}

func NewServer(cfg Config, handler func(*bufio.Reader) error) Server {
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
			return err
		}
		if err != nil {
			// Add retry?
			<-slots
			continue
		}

		go func(con net.Conn) {
			defer func() { <-slots }()

			s.handle(con)
		}(con)
	}
}

func (s Server) handle(con net.Conn) {
	defer con.Close()

	if err := s.handler(bufio.NewReader(con)); err != nil {
		slog.Warn("error handling the client connection", "error", err)
	}
}
