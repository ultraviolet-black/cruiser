package servicediscovery

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/ultraviolet-black/cruiser/pkg/state"
)

type xds struct {
	sdClient               *servicediscovery.Client
	namespacesNames        []string
	clusterLoadAssignments []*endpointv3.ClusterLoadAssignment
	servicePortTagKey      string

	xdsState state.XdsState

	periodicSyncInterval time.Duration
	errCh                chan error
}

func (x *xds) listNamespaces(ctx context.Context) ([]types.NamespaceSummary, error) {

	namespaces := []types.NamespaceSummary{}

	for _, namespaceName := range x.namespacesNames {

		ouput, err := x.sdClient.ListNamespaces(ctx, &servicediscovery.ListNamespacesInput{
			Filters: []types.NamespaceFilter{
				{
					Name:      types.NamespaceFilterNameType,
					Values:    []string{namespaceName},
					Condition: types.FilterConditionEq,
				},
			},
		})

		if err != nil {
			return nil, err
		}

		namespaces = append(namespaces, ouput.Namespaces...)

	}

	return namespaces, nil

}

type serviceInfo struct {
	types.ServiceSummary
	NamespaceId string
	Tags        []types.Tag
}

func (s serviceInfo) getTagByKey(key string) types.Tag {

	for _, tag := range s.Tags {
		if aws.ToString(tag.Key) == key {
			return tag
		}
	}

	return types.Tag{}

}

func (x *xds) listServices(ctx context.Context, namespaces []types.NamespaceSummary) ([]serviceInfo, error) {

	services := []serviceInfo{}

	for _, namespace := range namespaces {

		paginator := servicediscovery.NewListServicesPaginator(x.sdClient, &servicediscovery.ListServicesInput{
			Filters: []types.ServiceFilter{
				{
					Name:      types.ServiceFilterNameNamespaceId,
					Values:    []string{aws.ToString(namespace.Id)},
					Condition: types.FilterConditionEq,
				},
			},
		})

		for paginator.HasMorePages() {

			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}

			for _, service := range output.Services {

				listTags, err := x.sdClient.ListTagsForResource(ctx, &servicediscovery.ListTagsForResourceInput{
					ResourceARN: service.Arn,
				})
				if err != nil {
					return nil, err
				}

				services = append(services, serviceInfo{
					ServiceSummary: service,
					NamespaceId:    aws.ToString(namespace.Id),
					Tags:           listTags.Tags,
				})

			}

		}
	}

	return services, nil

}

func (x *xds) sync(ctx context.Context) error {

	namespaces, err := x.listNamespaces(ctx)
	if err != nil {
		return err
	}

	services, err := x.listServices(ctx, namespaces)
	if err != nil {
		return err
	}

	for _, assignment := range x.clusterLoadAssignments {
		assignment.Endpoints = []*endpointv3.LocalityLbEndpoints{}
	}

	for _, service := range services {

		servicePortTag := service.getTagByKey(x.servicePortTagKey)

		if servicePortTag.Value == nil {
			continue
		}

		servicePort, err := strconv.ParseInt(aws.ToString(servicePortTag.Value), 10, 32)
		if err != nil {
			return err
		}

		clusterLoadAssignment := &endpointv3.ClusterLoadAssignment{
			ClusterName: fmt.Sprintf("%s.%s", aws.ToString(service.Name), service.NamespaceId),
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpointv3.LbEndpoint{},
				},
			},
		}

		paginator := servicediscovery.NewListInstancesPaginator(x.sdClient, &servicediscovery.ListInstancesInput{
			ServiceId: service.Id,
		})

		for paginator.HasMorePages() {

			output, err := paginator.NextPage(ctx)
			if err != nil {
				return err
			}

			for _, instance := range output.Instances {

				clusterLoadAssignment.Endpoints[0].LbEndpoints = append(clusterLoadAssignment.Endpoints[0].LbEndpoints, &endpointv3.LbEndpoint{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Hostname: aws.ToString(instance.Id),
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Protocol: corev3.SocketAddress_TCP,
										Address:  instance.Attributes["AWS_INSTANCE_IPV4"],
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: uint32(servicePort),
										},
									},
								},
							},
						},
					},
				})

			}
		}
	}

	return nil

}

func (x *xds) flush() {

	x.xdsState.SetClusterEndpoints(x.clusterLoadAssignments)

	clusterLoadAssignments := []*endpointv3.ClusterLoadAssignment{}

	for _, assignment := range x.clusterLoadAssignments {

		if len(assignment.Endpoints) > 0 {
			clusterLoadAssignments = append(clusterLoadAssignments, assignment)
		}

	}

	x.clusterLoadAssignments = clusterLoadAssignments

}

func (x *xds) periodicSync(ctx context.Context) {

	for {

		if err := x.sync(ctx); err != nil {
			x.errCh <- err
			return
		}

		x.flush()

		select {

		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				x.errCh <- err
			}
			return

		case <-time.After(x.periodicSyncInterval):

		}

	}

}

func (x *xds) Start(ctx context.Context) {
	go x.periodicSync(ctx)
}

func (x *xds) ErrorCh() <-chan error {
	return x.errCh
}
