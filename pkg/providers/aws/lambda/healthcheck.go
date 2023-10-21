package lambda

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	awspb "github.com/ultraviolet-black/cruiser/pkg/proto/providers/aws"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/proto"
)

func DoHealthcheck(ctx context.Context, lambdaCli *lambda.Client, backend *awspb.LambdaBackend) {

	reqPb := &grpc_health_v1.HealthCheckRequest{
		Service: "*",
	}

	protoBuf, _ := proto.Marshal(reqPb)

	req := &events.APIGatewayProxyRequest{
		HTTPMethod:                      "POST",
		Path:                            fmt.Sprintf("%s/%s", grpc_health_v1.Health_ServiceDesc.ServiceName, "Check"),
		MultiValueHeaders:               make(map[string][]string),
		MultiValueQueryStringParameters: make(map[string][]string),
		Body:                            base64.StdEncoding.EncodeToString(protoBuf[:]),
		IsBase64Encoded:                 true,
	}

	payload, err := json.Marshal(req)
	if err != nil {

	}

	result, err := lambdaCli.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(backend.FunctionName),
		Qualifier:      aws.String(backend.Qualifier),
		InvocationType: types.InvocationTypeRequestResponse,
		Payload:        payload,
	})

	if result.FunctionError != nil {

	}
	if err != nil {

	}

	lambdaResponse := &events.APIGatewayProxyResponse{}
	if err := json.Unmarshal(result.Payload, lambdaResponse); err != nil {
	}

	var resBuf []byte

	if lambdaResponse.IsBase64Encoded {

		responseBody, err := base64.StdEncoding.DecodeString(lambdaResponse.Body)
		if err != nil {

		}

		resBuf = responseBody

	} else {
		resBuf = []byte(lambdaResponse.Body)
	}

	resPb := &grpc_health_v1.HealthCheckResponse{}

	if err := proto.Unmarshal(resBuf, resPb); err != nil {

	}

	switch resPb.Status {
	case grpc_health_v1.HealthCheckResponse_SERVING:

	case grpc_health_v1.HealthCheckResponse_NOT_SERVING:

	case grpc_health_v1.HealthCheckResponse_UNKNOWN:
	}

}
