package state

import (
	"context"
	"sync"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"
)

type StateManagerOption func(*state)

func WithXdsState() StateManagerOption {
	return func(s *state) {
		s.xds = &xdsState{
			listenersMap:              make(map[string]*listenerv3.Listener),
			virtualHostsMap:           make(map[string]*routev3.VirtualHost),
			routeConfigurationsMap:    make(map[string]*routev3.RouteConfiguration),
			clustersMap:               make(map[string]*clusterv3.Cluster),
			clusterLoadAssignmentsMap: make(map[string]*endpointv3.ClusterLoadAssignment),

			endpointsMap: make(map[string][]*endpointv3.LocalityLbEndpoints),

			listenersToDelete:              make(map[string]*listenerv3.Listener),
			virtualHostsToDelete:           make(map[string]*routev3.VirtualHost),
			routeConfigurationsToDelete:    make(map[string]*routev3.RouteConfiguration),
			clustersToDelete:               make(map[string]*clusterv3.Cluster),
			clusterLoadAssignmentsToDelete: make(map[string]*endpointv3.ClusterLoadAssignment),

			listenerCache:              cache.NewLinearCache(resource.ListenerType),
			virtualHostCache:           cache.NewLinearCache(resource.VirtualHostType),
			routeConfigurationCache:    cache.NewLinearCache(resource.RouteType),
			clusterCache:               cache.NewLinearCache(resource.ClusterType),
			clusterLoadAssignmentCache: cache.NewLinearCache(resource.EndpointType),

			rwLock: new(sync.RWMutex),
		}
	}
}

func WithRoutesState() StateManagerOption {
	return func(s *state) {
		s.routes = &routesState{
			routes: NewGraph(func(r *serverpb.Router_Route) string {
				return r.Name
			}),
			routesMap: make(map[string]*serverpb.Router_Route),
			rwLock:    new(sync.RWMutex),
		}
	}
}

func WithPeriodicSyncInterval(interval time.Duration) StateManagerOption {
	return func(s *state) {
		s.periodicSyncInterval = interval
	}
}

func WithTfstateSource(tfstateSource TfstateSource) StateManagerOption {
	return func(s *state) {
		s.tfstateSource = tfstateSource
	}
}

type StateManager interface {
	Start(context.Context)

	RoutesState() RoutesState
	XdsState() XdsState

	ErrorCh() <-chan error
	UpdateRoutesCh() <-chan RoutesState
	UpdateXdsCh() <-chan XdsState
}

func NewStateManager(opts ...StateManagerOption) StateManager {
	s := &state{
		periodicSyncInterval: 5 * time.Second,
		errCh:                make(chan error),
		updateXdsCh:          make(chan XdsState),
		updateRoutesCh:       make(chan RoutesState),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
