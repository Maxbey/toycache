package engine

import (
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

// Get returns an immutable view of the stored value. Callers must not modify
// the returned slice.
func (e *Engine) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	type getResult struct {
		value []byte
		found bool
	}

	k := string(key)
	r, err := submit(ctx, e, func(e *Engine) getResult {
		v, found := e.keyspace[k]

		return getResult{v, found}
	})
	if err != nil {
		return nil, false, err
	}

	return r.value, r.found, nil
}

// Set stores value without copying it. The caller transfers ownership of value
// to the engine and must not access or modify it after calling Set, even when
// Set returns an error.
func (e *Engine) Set(ctx context.Context, key []byte, value []byte) error {
	k := string(key)
	_, err := submit(ctx, e, func(e *Engine) struct{} {
		e.keyspace[k] = value

		return struct{}{}
	})
	if err != nil {
		return err
	}

	return nil
}

func (e *Engine) Delete(ctx context.Context, keys ...[]byte) (int64, error) {
	ks := make([]string, len(keys))
	for i, key := range keys {
		ks[i] = string(key)
	}

	deleted, err := submit(ctx, e, func(e *Engine) int64 {
		var count int64
		for _, key := range ks {
			if _, found := e.keyspace[key]; !found {
				continue
			}

			delete(e.keyspace, key)
			count++
		}

		return count
	})
	if err != nil {
		return 0, err
	}

	return deleted, nil
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
