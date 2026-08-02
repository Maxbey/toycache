package engine

import (
	"bytes"
	"context"
)

type executable interface {
	execute(*Engine)
}

type operation[T any] struct {
	handler func(*Engine) T
	result  chan T
}

func newOperation[T any](handler func(e *Engine) T) operation[T] {
	return operation[T]{
		handler: handler,
		result:  make(chan T, 1),
	}
}

func (op operation[T]) execute(engine *Engine) {
	op.result <- op.handler(engine)
}

type Engine struct {
	operations chan executable
	keyspace   map[string][]byte
}

func NewEngine() *Engine {
	return &Engine{
		operations: make(chan executable),
		keyspace:   make(map[string][]byte),
	}
}

func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case op := <-e.operations:
			op.execute(e)
		}
	}
}

func (e *Engine) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	type getResult struct {
		value []byte
		found bool
	}

	k := string(key)
	r, err := submit(ctx, e, func(e *Engine) getResult {
		v, found := e.keyspace[k]

		return getResult{bytes.Clone(v), found}
	})
	if err != nil {
		return nil, false, err
	}

	return r.value, r.found, nil
}

func submit[T any](
	ctx context.Context,
	engine *Engine,
	handler func(*Engine) T,
) (T, error) {
	op := newOperation(handler)

	select {
	case engine.operations <- op:
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}

	select {
	case result := <-op.result:
		return result, nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
