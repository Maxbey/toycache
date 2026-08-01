package cache

import (
	"bufio"

	"github.com/Maxbey/toycache/internal/resp"
)

func Entrypoint(reader *bufio.Reader) error {
	p := resp.NewParser(reader)

	p.Parse()

	return nil
}
