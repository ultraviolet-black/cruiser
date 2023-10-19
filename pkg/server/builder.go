package server

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"
	"google.golang.org/grpc"
)

type ServerOption func(*server)

func WithListenerProtocol(listenerProtocol ListenerProtocol) ServerOption {
	return func(s *server) {
		s.listenerProtocol = listenerProtocol
	}
}

func WithListenerAddress(listenerAddress string) ServerOption {
	return func(s *server) {
		s.listenerAddress = listenerAddress
	}
}

func WithTLSConfig(tlsConfig func() *tls.Config) ServerOption {
	return func(s *server) {
		s.tlsConfig = tlsConfig
	}
}

func WithHTTPHandler(httpHandler http.Handler) ServerOption {
	return func(s *server) {
		s.httpHandler = httpHandler
	}
}

func WithShutdownTimeout(shutdownTimeout time.Duration) ServerOption {
	return func(s *server) {
		s.shutdownTimeout = shutdownTimeout
	}
}

type Server interface {
	Open() error
	Close() error
}

func NewServer(options ...ServerOption) Server {

	s := &server{
		closeChannel: make(chan error),
	}

	for _, option := range options {
		option(s)
	}

	return s

}

type GrpcHandlerOption func(*grpcHandler)

func WithGrpcMethodBackendFactory(methodBackendFactory GrpcMethodBackendFactory) GrpcHandlerOption {
	return func(g *grpcHandler) {
		g.methodBackendFactory = methodBackendFactory
	}
}

type GrpcHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	Handle(interface{}, grpc.ServerStream) error
}

func NewGrpcHandler(options ...GrpcHandlerOption) GrpcHandler {

	g := &grpcHandler{}

	g.grpcServer = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(g.Handle),
	)

	for _, option := range options {
		option(g)
	}

	return g

}

type RouterOption func(*router)

func WithRouterConfig(routerConfig *serverpb.Router) RouterOption {
	return func(r *router) {
		if err := r.parseProtoRouterConfig(routerConfig); err != nil {
			panic(err)
		}
	}
}

func WithBackendProvider(provider BackendProvider) RouterOption {
	return func(r *router) {
		r.provs[provider.BackendProviderKey()] = provider
	}
}

type Router interface {
	http.Handler
}

func NewRouter(options ...RouterOption) Router {

	r := &router{
		rtr:   mux.NewRouter(),
		provs: make(map[BackendProviderKey]BackendProvider),
	}

	for _, option := range options {
		option(r)
	}

	return r

}

type SwapHandler interface {
	http.Handler
	Swap(http.Handler)
	Close()
}

func NewSwapHandler() SwapHandler {

	h := &swapHandler{
		handlerCh: make(chan http.Handler),
	}

	go h.watchSwap()

	return h

}
