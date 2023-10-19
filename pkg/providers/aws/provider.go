package aws

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"

	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awsservicediscovery "github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/ultraviolet-black/cruiser/pkg/providers/aws/lambda"
	"github.com/ultraviolet-black/cruiser/pkg/server"
)

type awsProvider struct {
	grpcHandlers map[string]server.GrpcHandler
	httpHandlers map[string]http.Handler

	config aws.Config

	lambdaClient           *awslambda.Client
	s3Client               *awss3.Client
	serviceDiscoveryClient *awsservicediscovery.Client

	dynamodbEndpoint string
}

func (p *awsProvider) GetLambdaClient() *awslambda.Client {
	return p.lambdaClient
}

func (p *awsProvider) GetS3Client() *awss3.Client {
	return p.s3Client
}

func (p *awsProvider) GetServiceDiscoveryClient() *awsservicediscovery.Client {
	return p.serviceDiscoveryClient
}

func (p *awsProvider) BackendProviderKey() server.BackendProviderKey {
	return server.AWSBackendProvider
}

func (p *awsProvider) ToGrpcBackend(h *serverpb.Router_Handler) http.Handler {

	switch backend := h.Backend.(type) {

	case *serverpb.Router_Handler_AwsLambda:
		return lambda.NewGrpcBackend(p.lambdaClient, backend.AwsLambda)

	}

	observability.Log.Panicw(server.ErrNoBackendFound.Error())

	return nil

}

func (p *awsProvider) ToHttpBackend(h *serverpb.Router_Handler) http.Handler {

	switch backend := h.Backend.(type) {

	case *serverpb.Router_Handler_AwsLambda:
		return lambda.NewHttpBackend(p.lambdaClient, backend.AwsLambda)

	}

	observability.Log.Panicw(server.ErrNoBackendFound.Error())

	return nil

}
