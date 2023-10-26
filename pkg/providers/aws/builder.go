package aws

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awsservicediscovery "github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	awssts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	serverpb "github.com/ultraviolet-black/cruiser/pkg/proto/server"
	"github.com/ultraviolet-black/cruiser/pkg/server"
)

type ProviderOption func(*awsProvider)

func WithDynamoDBEndpoint(endpoint string) ProviderOption {
	return func(p *awsProvider) {
		p.dynamodbEndpoint = endpoint
	}
}

func WithHealthCheckInterval(interval time.Duration) ProviderOption {
	return func(p *awsProvider) {
		p.healthCheckInterval = interval
	}
}

func WithHealthCheckParallelism(parallelism int) ProviderOption {
	return func(p *awsProvider) {
		p.healthCheckParallelism = parallelism
	}
}

type Provider interface {
	GetLambdaClient() *awslambda.Client
	GetS3Client() *awss3.Client
	GetS3ClientWithRole(roleArn string) func() *awss3.Client
	GetServiceDiscoveryClient() *awsservicediscovery.Client

	HealthCheckHandlers(context.Context, ...*serverpb.Router_Handler)

	BackendProviderKey() server.BackendProviderKey
	ToGrpcBackend(*serverpb.Router_Handler) http.Handler
	ToHttpBackend(*serverpb.Router_Handler) http.Handler
}

func NewProvider(opts ...ProviderOption) Provider {

	p := &awsProvider{
		grpcHandlers:           make(map[string]server.GrpcHandler),
		httpHandlers:           make(map[string]http.Handler),
		healthCheckInterval:    0,
		healthCheckParallelism: 4,
		healthCheckWg:          &sync.WaitGroup{},
	}

	for _, opt := range opts {
		opt(p)
	}

	customResolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
		if len(p.dynamodbEndpoint) > 0 && service == dynamodb.ServiceID {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           p.dynamodbEndpoint,
				SigningRegion: "us-east-1",
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfgOpts := []func(*config.LoadOptions) error{
		config.WithEndpointResolver(customResolver),
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), cfgOpts...)
	if err != nil {
		observability.Log.Panic(err.Error())
	}

	p.healthCheckCh = make(chan *serverpb.Router_Handler, p.healthCheckParallelism)

	p.config = cfg
	p.stsClient = awssts.NewFromConfig(cfg)
	p.lambdaClient = awslambda.NewFromConfig(cfg)
	p.s3Client = awss3.NewFromConfig(cfg)
	p.serviceDiscoveryClient = awsservicediscovery.NewFromConfig(cfg)

	return p

}
