package state

import (
	"context"
	"sync"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"

	"github.com/envoyproxy/go-control-plane/pkg/server/v3"

	_ "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/stream/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cors/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_web/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/health_check/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	_ "github.com/envoyproxy/go-control-plane/pkg/wellknown"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

type XdsState interface {
	Register(context.Context, *grpc.Server)
	SetClusterEndpoints([]*endpointv3.ClusterLoadAssignment)
}

type xdsState struct {
	listenersMap              map[string]*listenerv3.Listener
	virtualHostsMap           map[string]*routev3.VirtualHost
	routeConfigurationsMap    map[string]*routev3.RouteConfiguration
	clustersMap               map[string]*clusterv3.Cluster
	clusterLoadAssignmentsMap map[string]*endpointv3.ClusterLoadAssignment

	endpointsMap map[string][]*endpointv3.LocalityLbEndpoints

	listenersToDelete              map[string]*listenerv3.Listener
	virtualHostsToDelete           map[string]*routev3.VirtualHost
	routeConfigurationsToDelete    map[string]*routev3.RouteConfiguration
	clustersToDelete               map[string]*clusterv3.Cluster
	clusterLoadAssignmentsToDelete map[string]*endpointv3.ClusterLoadAssignment

	listenerCache              *cache.LinearCache
	virtualHostCache           *cache.LinearCache
	routeConfigurationCache    *cache.LinearCache
	clusterCache               *cache.LinearCache
	clusterLoadAssignmentCache *cache.LinearCache

	rwLock *sync.RWMutex
}

func readXdsFromTfstate(tfstate *Tfstate, xs *xdsState) error {

	for _, resource := range tfstate.Resources {

		for _, instance := range resource.Instances {

			switch resource.Type {

			case "cruiser_envoy_listener":

				listener := &listenerv3.Listener{}

				if err := protojson.Unmarshal(instance, listener); err != nil {
					return err
				}

				name := cache.GetResourceName(listener)

				xs.listenersMap[name] = listener

			case "cruiser_envoy_virtual_host":

				virtualHost := &routev3.VirtualHost{}

				if err := protojson.Unmarshal(instance, virtualHost); err != nil {
					return err
				}

				name := cache.GetResourceName(virtualHost)

				xs.virtualHostsMap[name] = virtualHost

			case "cruiser_envoy_cluster":

				cluster := &clusterv3.Cluster{}

				if err := protojson.Unmarshal(instance, cluster); err != nil {
					return err
				}

				name := cache.GetResourceName(cluster)

				xs.clustersMap[name] = cluster

			case "cruiser_envoy_route_configuration":

				routeConfiguration := &routev3.RouteConfiguration{}

				if err := protojson.Unmarshal(instance, routeConfiguration); err != nil {
					return err
				}

				name := cache.GetResourceName(routeConfiguration)

				xs.routeConfigurationsMap[name] = routeConfiguration

			case "cruiser_envoy_cluster_load_assignment":

				clusterLoadAssignment := &endpointv3.ClusterLoadAssignment{}

				if err := protojson.Unmarshal(instance, clusterLoadAssignment); err != nil {
					return err
				}

				name := cache.GetResourceName(clusterLoadAssignment)

				if endpoints, ok := xs.endpointsMap[name]; ok {
					clusterLoadAssignment.Endpoints = endpoints
				}

				xs.clusterLoadAssignmentsMap[name] = clusterLoadAssignment

			}

		}

	}

	return nil

}

func transactXdsResource[T types.Resource](cache *cache.LinearCache, toUpdate map[string]T, toDelete map[string]T) error {

	updates := make(map[string]types.Resource)

	for name, resource := range toUpdate {

		delete(toDelete, name)

		updates[name] = resource

	}

	deletes := []string{}

	for name := range toDelete {

		deletes = append(deletes, name)

	}

	return cache.UpdateResources(updates, deletes)

}

func (xs *xdsState) build() error {

	xs.rwLock.Lock()
	defer xs.rwLock.Unlock()

	if err := transactXdsResource(xs.clusterCache, xs.clustersMap, xs.clustersToDelete); err != nil {
		return err
	}

	if err := transactXdsResource(xs.clusterLoadAssignmentCache, xs.clusterLoadAssignmentsMap, xs.clusterLoadAssignmentsToDelete); err != nil {
		return err
	}

	if err := transactXdsResource(xs.listenerCache, xs.listenersMap, xs.listenersToDelete); err != nil {
		return err
	}

	if err := transactXdsResource(xs.virtualHostCache, xs.virtualHostsMap, xs.virtualHostsToDelete); err != nil {
		return err
	}

	if err := transactXdsResource(xs.routeConfigurationCache, xs.routeConfigurationsMap, xs.routeConfigurationsToDelete); err != nil {
		return err
	}

	xs.listenersToDelete = xs.listenersMap
	xs.virtualHostsToDelete = xs.virtualHostsMap
	xs.routeConfigurationsToDelete = xs.routeConfigurationsMap
	xs.clustersToDelete = xs.clustersMap
	xs.clusterLoadAssignmentsToDelete = xs.clusterLoadAssignmentsMap

	xs.listenersMap = make(map[string]*listenerv3.Listener)
	xs.virtualHostsMap = make(map[string]*routev3.VirtualHost)
	xs.routeConfigurationsMap = make(map[string]*routev3.RouteConfiguration)
	xs.clustersMap = make(map[string]*clusterv3.Cluster)
	xs.clusterLoadAssignmentsMap = make(map[string]*endpointv3.ClusterLoadAssignment)

	return nil

}

func (xs *xdsState) SetClusterEndpoints(assignments []*endpointv3.ClusterLoadAssignment) {

	xs.rwLock.Lock()
	defer xs.rwLock.Unlock()

	for _, assignment := range assignments {

		if len(assignment.Endpoints) == 0 {
			delete(xs.endpointsMap, assignment.ClusterName)
			return
		}

		xs.endpointsMap[assignment.ClusterName] = assignment.Endpoints

	}

}

func (xs *xdsState) Register(ctx context.Context, grpcServer *grpc.Server) {

	cache := &cache.MuxCache{
		Classify: func(r *cache.Request) string {
			return r.TypeUrl
		},
		ClassifyDelta: func(dr *cache.DeltaRequest) string {
			return dr.TypeUrl
		},
		Caches: map[string]cache.Cache{
			resource.ListenerType:    xs.listenerCache,
			resource.RouteType:       xs.routeConfigurationCache,
			resource.ClusterType:     xs.clusterCache,
			resource.VirtualHostType: xs.virtualHostCache,
			resource.EndpointType:    xs.clusterLoadAssignmentCache,
		},
	}

	xdsServer := server.NewServer(ctx, cache, nil)

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)

	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, xdsServer)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterVirtualHostDiscoveryServiceServer(grpcServer, xdsServer)

}
