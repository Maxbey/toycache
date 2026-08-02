package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/Maxbey/toycache/internal/api"
	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/server"
)

var (
	host           = flag.String("host", "", "api server host")
	port           = flag.Int("port", 0, "api server port")
	maxConnections = flag.Int("max-connections", 1024, "api server port")
)

func main() {
	flag.Parse()

	if err := validateFlags(); err != nil {
		flag.Usage()
		log.Fatalf("invalid flags: %v", err)
	}

	cfg := server.Config{
		Host:           *host,
		Port:           *port,
		MaxConnections: *maxConnections,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := engine.NewEngine()
	go eng.Run(ctx)

	handler := api.NewHandler(eng)
	srv := server.NewServer(cfg, handler.Handle)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("err running api server: %v", err)
	}
}

func validateFlags() error {
	if *host == "" {
		return fmt.Errorf("-host is required")
	}

	if *port < 1 || *port > 65535 {
		return fmt.Errorf("-port must be between 1 and 65535")
	}

	return nil
}
