package api

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
)

const (
	protocol          = "tcp"
	frameLengthHeader = 4
	frameMaxBuffer    = 1 << 20
	sharedFrameBuffer = 4 << 10
)

type Config struct {
	Host           string
	Port           int
	MaxConnections int
}

type Server struct {
	cfg Config
}

func NewServer(cfg Config) Server {
	return Server{
		cfg: cfg,
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

			if err := s.handle(con); err != nil && !errors.Is(err, io.EOF) {
				slog.Error("connection failed", "error", err)
			}
		}(con)
	}
}

func (s Server) handle(con net.Conn) error {
	defer con.Close()

	var (
		frame  []byte
		err    error
		length uint32
	)
	headerBuf := make([]byte, frameLengthHeader)
	sharedMessageBuf := make([]byte, sharedFrameBuffer)

	for {
		length, err = readFrameLength(con, headerBuf)
		if err != nil {
			return err
		}

		if length > sharedFrameBuffer {
			frame, err = readFrame(con, make([]byte, length))
		} else {
			frame, err = readFrame(con, sharedMessageBuf[:length])
		}
		if err != nil {
			return err
		}

		slog.Info("got", "frame", frame)
	}

}

func readFrameLength(con net.Conn, buf []byte) (uint32, error) {
	_, err := io.ReadFull(con, buf)
	if err != nil {
		return 0, err
	}

	length := binary.BigEndian.Uint32(buf[:])
	if length > frameMaxBuffer {
		return 0, fmt.Errorf("frame size is too big: %d", length)
	}

	return length, nil
}

func readFrame(con net.Conn, buf []byte) ([]byte, error) {
	_, err := io.ReadFull(con, buf)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
