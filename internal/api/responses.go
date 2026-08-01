package api

import "github.com/Maxbey/toycache/internal/resp"

func ErrorResponse(err error) resp.Element {
	return resp.Element{
		Type:  resp.Error,
		Value: []byte(err.Error()),
	}
}
