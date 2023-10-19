package server

import "errors"

var (
	ErrEmptyListenerAddress = errors.New("empty listener address")
	ErrEmptyHTTP2TLSConfig  = errors.New("empty tls config for http2 protocol")
	ErrEmptyListener        = errors.New("empty listener")
	ErrNonEmptyListener     = errors.New("non empty listener")
	ErrNonEmptyHTTPServer   = errors.New("non empty http server")
	ErrEmptyHTTPHandler     = errors.New("empty http handler")
	ErrNoBackendProvider    = errors.New("no backend provider")
	ErrNoBackendFound       = errors.New("no backend found")
)
