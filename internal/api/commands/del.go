package commands

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

type Del struct {
	engine *engine.Engine
}

func NewDel(engine *engine.Engine) Del {
	return Del{engine: engine}
}

func (Del) Validate(arguments []resp.Element) error {
	if len(arguments) == 0 {
		return fmt.Errorf("wrong number of arguments for 'del' command")
	}

	for _, argument := range arguments {
		if argument.Type != resp.BulkString || argument.Null {
			return fmt.Errorf("del keys must be non-null bulk strings")
		}
	}

	return nil
}

func (d Del) Execute(ctx context.Context, arguments []resp.Element) (resp.Element, error) {
	keys := make([][]byte, len(arguments))
	for i, argument := range arguments {
		keys[i] = argument.Value
	}

	deleted, err := d.engine.Delete(ctx, keys...)
	if err != nil {
		return resp.Element{}, fmt.Errorf("deleting keys: %w", err)
	}

	return resp.Element{
		Type:  resp.Integer,
		Value: strconv.AppendInt(nil, deleted, 10),
	}, nil
}
