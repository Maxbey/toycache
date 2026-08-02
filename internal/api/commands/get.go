package commands

import (
	"context"
	"fmt"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

type Get struct {
	engine *engine.Engine
}

func NewGet(engine *engine.Engine) Get {
	return Get{engine: engine}
}

func (Get) Validate(arguments []resp.Element) error {
	if len(arguments) != 1 {
		return fmt.Errorf("wrong number of arguments for 'get' command")
	}
	if arguments[0].Type != resp.BulkString || arguments[0].Null {
		return fmt.Errorf("get key must be a non-null bulk string")
	}

	return nil
}

func (g Get) Execute(ctx context.Context, arguments []resp.Element) (resp.Element, error) {
	value, found, err := g.engine.Get(ctx, arguments[0].Value)
	if err != nil {
		return resp.Element{}, fmt.Errorf("getting key: %w", err)
	}
	if !found {
		return resp.Element{Type: resp.BulkString, Null: true}, nil
	}

	return resp.Element{Type: resp.BulkString, Value: value}, nil
}
