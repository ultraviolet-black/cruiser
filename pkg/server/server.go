package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type ListenerProtocol int

const (
	HTTP2 ListenerProtocol = iota
	H2C
)

type server struct {
	listenerProtocol ListenerProtocol
	listenerAddress  string

	tlsConfig func() *tls.Config

	listener net.Listener

	httpServer  *http.Server
	httpHandler http.Handler

	shutdownTimeout time.Duration
	closeChannel    chan error
}

func (s *server) Open() error {

	if err := s.listen(); err != nil {
		return err
	}

	return s.serve()

}

func (s *server) Close() error {

	return s.close()

}

func (s *server) assertListen() error {

	if len(s.listenerAddress) == 0 {
		return ErrEmptyListenerAddress
	}

	if s.listenerProtocol == HTTP2 && s.tlsConfig == nil {
		return ErrEmptyHTTP2TLSConfig
	}

	if s.listener != nil {
		return ErrNonEmptyListener
	}

	return nil

}

func (s *server) listen() error {

	if err := s.assertListen(); err != nil {
		return err
	}

	if tlsConfig := s.tlsConfig(); tlsConfig != nil {

		lis, err := tls.Listen("tcp", s.listenerAddress, tlsConfig)

		if err != nil {
			return err
		}

		s.listener = lis

		return nil

	}

	lis, err := net.Listen("tcp", s.listenerAddress)

	if err != nil {
		return err
	}

	s.listener = lis

	return nil

}

func (s *server) assertServe() error {

	if s.listener == nil {
		return ErrEmptyListener
	}

	if s.httpServer != nil {
		return ErrNonEmptyHTTPServer
	}

	if s.httpHandler == nil {
		return ErrEmptyHTTPHandler
	}

	return nil

}

func (s *server) serve() error {

	if err := s.assertServe(); err != nil {
		return err
	}

	s.httpServer = &http.Server{}

	switch s.listenerProtocol {

	case HTTP2:

		if err := s.serveHTTP2(); err != nil {
			return err
		}

	case H2C:

		if err := s.serveH2C(); err != nil {
			return err
		}

	}

	go func() {
		s.closeChannel <- s.httpServer.Serve(s.listener)
	}()

	return nil
}

func (s *server) serveHTTP2() error {

	s.httpServer.Handler = s.httpHandler

	return http2.ConfigureServer(s.httpServer, &http2.Server{})

}

func (s *server) serveH2C() error {

	s.httpServer.Handler = h2c.NewHandler(s.httpHandler, &http2.Server{})

	return nil

}

func (s *server) close() error {

	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {

		if err == context.DeadlineExceeded {

			if err := s.httpServer.Close(); err != nil {
				return err
			}

			return s.waitCloseChannel()
		}

		return err
	}

	return s.waitCloseChannel()

}

func (s *server) waitCloseChannel() error {

	defer close(s.closeChannel)

	switch err := <-s.closeChannel; err {

	case http.ErrServerClosed:
		return nil

	default:
		return err
	}

}
