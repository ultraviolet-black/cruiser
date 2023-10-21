package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"
)

type BackendProviderKey int

const (
	AWSBackendProvider BackendProviderKey = iota
)

type BackendProvider interface {
	BackendProviderKey() BackendProviderKey
	HealthCheckHandlers(context.Context, ...*serverpb.Router_Handler)
	ToGrpcBackend(*serverpb.Router_Handler) http.Handler
	ToHttpBackend(*serverpb.Router_Handler) http.Handler
}

type router struct {
	rtr      *mux.Router
	provs    map[BackendProviderKey]BackendProvider
	handlers []*serverpb.Router_Handler
}

func (r *router) parseProtoRouterConfig(routerConfig *serverpb.Router) error {

	for _, route := range routerConfig.Routes {
		if err := r.parseProtoRoute(route); err != nil {
			return err
		}
	}

	return nil

}

func (r *router) parseProtoRoute(route *serverpb.Router_Route) error {

	rtr := r.rtr

	if route.ParentName != "" {
		rtr = r.rtr.Get(route.ParentName).Subrouter()
	}

	rt := rtr.NewRoute().Name(route.Name)

	isGrpcCall := false

	for _, matcher := range route.Matchers {

		switch rule := matcher.Rule.(type) {

		case *serverpb.Router_Route_Matcher_Host:
			rt.Host(rule.Host)

		case *serverpb.Router_Route_Matcher_Path:
			rt.Path(rule.Path)

		case *serverpb.Router_Route_Matcher_PathPrefix:
			rt.PathPrefix(rule.PathPrefix)

		case *serverpb.Router_Route_Matcher_Methods:

			methods := make([]string, len(rule.Methods.Methods))

			for i, method := range rule.Methods.Methods {
				methods[i] = serverpb.Router_Route_MethodsRule_Method_name[int32(method)]
			}

			rt.Methods(methods...)

		case *serverpb.Router_Route_Matcher_Schemes:

			schemes := make([]string, len(rule.Schemes.Schemes))

			for i, scheme := range rule.Schemes.Schemes {
				schemes[i] = serverpb.Router_Route_SchemesRule_Scheme_name[int32(scheme)]
			}

			rt.Schemes(schemes...)

		case *serverpb.Router_Route_Matcher_Headers:

			headers := make([]string, len(rule.Headers.Headers))

			for key, value := range rule.Headers.Headers {
				headers = append(headers, key, value)
			}

			rt.Headers(headers...)

		case *serverpb.Router_Route_Matcher_HeadersRegexp:

			headers := make([]string, len(rule.HeadersRegexp.HeadersRegexp))

			for key, value := range rule.HeadersRegexp.HeadersRegexp {
				headers = append(headers, key, value)
			}

			rt.HeadersRegexp(headers...)

		case *serverpb.Router_Route_Matcher_Queries:

			queries := make([]string, len(rule.Queries.Queries))

			for key, value := range rule.Queries.Queries {
				queries = append(queries, key, value)
			}

			rt.Queries(queries...)

		case *serverpb.Router_Route_Matcher_IsGrpcCall:

			if rule.IsGrpcCall {
				rt.MatcherFunc(func(req *http.Request, rm *mux.RouteMatch) bool {
					return req.ProtoMajor == 2 && strings.Contains(req.Header.Get("Content-Type"), "application/grpc")
				})

				isGrpcCall = true
			}

		}

	}

	if route.Handler == nil {
		return nil
	}

	var provKey BackendProviderKey

	r.handlers = append(r.handlers, route.Handler)

	switch route.Handler.Backend.(type) {

	case *serverpb.Router_Handler_AwsLambda:
		provKey = AWSBackendProvider

	}

	prov, ok := r.provs[provKey]
	if !ok {
		return ErrNoBackendProvider
	}

	if isGrpcCall {

		rt.Handler(prov.ToGrpcBackend(route.Handler))

		return nil

	}

	rt.Handler(prov.ToHttpBackend(route.Handler))

	return nil

}

func (r *router) DoHealthcheck(ctx context.Context) {

	for _, prov := range r.provs {
		prov.HealthCheckHandlers(ctx, r.handlers...)
	}

}

func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	r.rtr.ServeHTTP(w, req)

}
