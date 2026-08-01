package api

import (
	"bufio"
	"fmt"

	"github.com/Maxbey/toycache/internal/resp"
)

var dispatcher = NewDispatcher()

func Entrypoint(readWriter *bufio.ReadWriter) error {
	reader := resp.NewReader(readWriter.Reader)
	writer := resp.NewWriter(readWriter.Writer)

	for {
		request, err := reader.Read()
		if err != nil {
			if err := writeResponse(readWriter, writer, ErrorResponse(err)); err != nil {
				return err
			}

			return nil
		}

		response, err := dispatcher.Dispatch(request)
		if err != nil {
			response = ErrorResponse(err)
		}
		if err := writeResponse(readWriter, writer, response); err != nil {
			return err
		}
	}
}

func writeResponse(readWriter *bufio.ReadWriter, writer resp.Writer, response resp.Element) error {
	if err := writer.Write(response); err != nil {
		return fmt.Errorf("writing response: %w", err)
	}
	if err := readWriter.Flush(); err != nil {
		return fmt.Errorf("flushing response: %w", err)
	}

	return nil
}
