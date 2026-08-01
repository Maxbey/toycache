package commands

import (
	"fmt"

	"github.com/Maxbey/toycache/internal/resp"
)

type Ping struct{}

func (Ping) Validate(arguments []resp.Element) error {
	if len(arguments) > 1 {
		return fmt.Errorf("wrong number of arguments for 'ping' command")
	}
	if len(arguments) == 1 && (arguments[0].Type != resp.BulkString || arguments[0].Null) {
		return fmt.Errorf("ping argument must be a non-null bulk string")
	}

	return nil
}

func (Ping) Execute(arguments []resp.Element) (resp.Element, error) {
	if len(arguments) == 0 {
		return resp.Element{Type: resp.String, Value: []byte("PONG")}, nil
	}

	return resp.Element{Type: resp.BulkString, Value: arguments[0].Value}, nil
}
