package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/Maxbey/toycache/internal/api/commands"
	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

type Dispatcher struct {
	commands map[string]command
}

type command interface {
	Validate(arguments []resp.Element) error
	Execute(ctx context.Context, arguments []resp.Element) (resp.Element, error)
}

func NewDispatcher(engine *engine.Engine) *Dispatcher {
	return &Dispatcher{
		commands: map[string]command{
			"PING": commands.Ping{},
			"GET":  commands.NewGet(engine),
			"SET":  commands.NewSet(engine),
		},
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, el resp.Element) (resp.Element, error) {
	if el.Type != resp.Array || el.Null {
		return resp.Element{}, fmt.Errorf("command must be a non-null array")
	}
	if len(el.Elements) == 0 {
		return resp.Element{}, fmt.Errorf("command array cannot be empty")
	}

	commandName := el.Elements[0]
	if commandName.Type != resp.BulkString || commandName.Null {
		return resp.Element{}, fmt.Errorf("command name must be a non-null bulk string")
	}

	name := strings.ToUpper(string(commandName.Value))
	cmd, ok := d.commands[name]
	if !ok {
		return resp.Element{}, fmt.Errorf("unknown command %q", commandName.Value)
	}

	arguments := el.Elements[1:]
	if err := cmd.Validate(arguments); err != nil {
		return resp.Element{}, err
	}

	return cmd.Execute(ctx, arguments)
}
