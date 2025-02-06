package server

import "github.com/lightyen/cloudflare-ddns/zok/log"

type ServerInternalError struct {
	err any
	log.StackTrace
}

func (e *ServerInternalError) Unwrap() error {
	if v, ok := e.err.(error); ok {
		return v
	}
	return nil
}

func (e *ServerInternalError) Error() string {
	switch v := e.err.(type) {
	default:
		return "<unknown error>"
	case string:
		return v
	case error:
		return v.Error()
	}
}

func (e *ServerInternalError) Stack() string {
	return e.StackTrace.String()
}

func (e *ServerInternalError) String() string {
	return e.Error() + "\n" + e.StackTrace.String()
}

func InternalServerError(e any) *ServerInternalError {
	return &ServerInternalError{err: e, StackTrace: log.Stack(4, 10)}
}
