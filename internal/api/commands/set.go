package commands

import (
	"context"
	"fmt"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

type Set struct {
	engine *engine.Engine
}

func NewSet(engine *engine.Engine) Set {
	return Set{engine: engine}
}

func (Set) Validate(arguments []resp.Element) error {
	if len(arguments) != 2 {
		return fmt.Errorf("wrong number of arguments for 'set' command")
	}
	if arguments[0].Type != resp.BulkString || arguments[0].Null {
		return fmt.Errorf("set key must be a non-null bulk string")
	}
	if arguments[1].Type != resp.BulkString || arguments[1].Null {
		return fmt.Errorf("set value must be a non-null bulk string")
	}

	return nil
}

func (s Set) Execute(ctx context.Context, arguments []resp.Element) (resp.Element, error) {
	if err := s.engine.Set(ctx, arguments[0].Value, arguments[1].Value); err != nil {
		return resp.Element{}, fmt.Errorf("setting key: %w", err)
	}

	return resp.Element{Type: resp.String, Value: []byte("OK")}, nil
}
