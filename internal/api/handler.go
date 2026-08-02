package api

import (
	"bufio"
	"context"
	"fmt"

	"github.com/Maxbey/toycache/internal/engine"
	"github.com/Maxbey/toycache/internal/resp"
)

type Handler struct {
	dispatcher *Dispatcher
}

const maxBatchSize = 16

func NewHandler(engine *engine.Engine) *Handler {
	return &Handler{dispatcher: NewDispatcher(engine)}
}

func (h *Handler) Handle(ctx context.Context, readWriter *bufio.ReadWriter) error {
	reader := newReader(readWriter.Reader)
	writer := resp.NewWriter(readWriter.Writer)

	for {
		request, err := reader.Read()
		if err != nil {
			if err := writeResponse(writer, ErrorResponse(err)); err != nil {
				return err
			}
			if err := flushResponses(readWriter); err != nil {
				return err
			}

			return nil
		}

		closeConnection := false
		for batchSize := range maxBatchSize {
			response, err := h.dispatcher.Dispatch(ctx, request)
			if err != nil {
				response = ErrorResponse(err)
			}
			if err := writeResponse(writer, response); err != nil {
				return err
			}
			if batchSize+1 == maxBatchSize {
				break
			}

			nextRequest, available, readErr := reader.TryRead()
			if readErr != nil {
				if err := writeResponse(writer, ErrorResponse(readErr)); err != nil {
					return err
				}
				closeConnection = true
				break
			}
			if !available {
				break
			}
			request = nextRequest
		}

		if err := flushResponses(readWriter); err != nil {
			return err
		}
		if closeConnection {
			return nil
		}
	}
}

func writeResponse(writer resp.Writer, response resp.Element) error {
	if err := writer.Write(response); err != nil {
		return fmt.Errorf("writing response: %w", err)
	}

	return nil
}

func flushResponses(readWriter *bufio.ReadWriter) error {
	if err := readWriter.Flush(); err != nil {
		return fmt.Errorf("flushing responses: %w", err)
	}

	return nil
}
