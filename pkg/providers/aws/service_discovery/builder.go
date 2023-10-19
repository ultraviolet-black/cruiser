package servicediscovery

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/ultraviolet-black/cruiser/pkg/state"
)

type XdsOption func(*xds)

func WithServiceDiscoveryClient(sdClient *servicediscovery.Client) XdsOption {
	return func(x *xds) {
		x.sdClient = sdClient
	}
}

func WithNamespacesNames(namespacesNames ...string) XdsOption {
	return func(x *xds) {
		x.namespacesNames = namespacesNames
	}
}

func WithServicePortTagKey(servicePortTagKey string) XdsOption {
	return func(x *xds) {
		x.servicePortTagKey = servicePortTagKey
	}
}

func WithPeriodicSyncInterval(interval time.Duration) XdsOption {
	return func(x *xds) {
		x.periodicSyncInterval = interval
	}
}

func WithXdsState(xdsState state.XdsState) XdsOption {
	return func(x *xds) {
		x.xdsState = xdsState
	}
}

type Xds interface {
	Start(context.Context)
	ErrorCh() <-chan error
}

func NewXds(opts ...XdsOption) Xds {

	xs := &xds{
		clusterLoadAssignments: make([]*endpointv3.ClusterLoadAssignment, 0),
		periodicSyncInterval:   5 * time.Second,
		errCh:                  make(chan error),
	}

	for _, opt := range opts {
		opt(xs)
	}

	return xs

}
