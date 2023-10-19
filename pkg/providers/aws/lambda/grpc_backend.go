package lambda

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/google/uuid"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	awspb "github.com/ultraviolet-black/cruiser/pkg/proto/providers/aws"
	"github.com/ultraviolet-black/cruiser/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type grpcBackend struct {
	server.GrpcHandler

	lambdaCli *lambda.Client

	backend *awspb.LambdaBackend
}

func NewGrpcBackend(lambdaCli *lambda.Client, backend *awspb.LambdaBackend) http.Handler {

	backendFactory := func(stream grpc.ServerStream) (server.GrpcMethodBackend, error) {

		return &grpcMethodBackend{
			lambdaCli:    lambdaCli,
			functionName: backend.FunctionName,
			qualifier:    backend.Qualifier,
			stream:       stream,
			transport:    grpc.ServerTransportStreamFromContext(stream.Context()),
		}, nil

	}

	return &grpcBackend{
		lambdaCli: lambdaCli,
		backend:   backend,
		GrpcHandler: server.NewGrpcHandler(
			server.WithGrpcMethodBackendFactory(backendFactory),
		),
	}
}

type grpcMethodBackend struct {
	lambdaCli *lambda.Client

	functionName string
	qualifier    string

	stream    grpc.ServerStream
	transport grpc.ServerTransportStream

	incomingMetadata metadata.MD
	outgoingMetadata metadata.MD

	lambdaResponse *events.APIGatewayProxyResponse
	responseBody   []byte
}

func wrapGrpcError(err error) error {

	errorId := uuid.NewString()

	observability.Log.Errorw("error handling request", "error", err, "errorId", errorId)

	return grpc.Errorf(codes.Internal, "internal server error: %s", errorId)

}

func (g *grpcMethodBackend) Begin(ctx context.Context, incomingMetadata metadata.MD) error {

	g.incomingMetadata = incomingMetadata

	return nil

}

func (g *grpcMethodBackend) Request(ctx context.Context, payload []byte) error {

	body := base64.StdEncoding.EncodeToString(payload[:])

	req := &events.APIGatewayProxyRequest{
		Path:              g.transport.Method(),
		MultiValueHeaders: g.incomingMetadata,
		Body:              body,
		IsBase64Encoded:   true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return wrapGrpcError(err)
	}

	result, err := g.lambdaCli.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(g.functionName),
		Qualifier:      aws.String(g.qualifier),
		InvocationType: types.InvocationTypeRequestResponse,
		Payload:        payload,
	})

	if result.FunctionError != nil {
		return wrapGrpcError(fmt.Errorf("function error: %s", *result.FunctionError))
	}
	if err != nil {
		return wrapGrpcError(err)
	}

	g.lambdaResponse = &events.APIGatewayProxyResponse{}
	if err := json.Unmarshal(result.Payload, g.lambdaResponse); err != nil {
		return wrapGrpcError(err)
	}

	if g.lambdaResponse.IsBase64Encoded {

		responseBody, err := base64.StdEncoding.DecodeString(g.lambdaResponse.Body)
		if err != nil {
			return wrapGrpcError(err)
		}

		g.responseBody = responseBody

	} else {
		g.responseBody = []byte(g.lambdaResponse.Body)
	}

	g.outgoingMetadata = metadata.Join(metadata.New(g.lambdaResponse.Headers), g.lambdaResponse.MultiValueHeaders)

	return nil

}

func (g *grpcMethodBackend) Response(context.Context) (metadata.MD, []byte, error) {

	var st *status.Status

	switch g.lambdaResponse.StatusCode {

	case http.StatusOK:
		return g.outgoingMetadata, g.responseBody, nil

	case http.StatusGone:
		st = status.New(codes.OK, "moved permanently")

	case http.StatusNotFound:
		st = status.New(codes.NotFound, "not found")

	case http.StatusForbidden:
		st = status.New(codes.PermissionDenied, "forbidden")

	case http.StatusUnauthorized:
		st = status.New(codes.Unauthenticated, "unauthorized")

	case http.StatusConflict:
		st = status.New(codes.AlreadyExists, "already exists")

	case http.StatusNotImplemented:
		st = status.New(codes.Unimplemented, "not implemented")

	case http.StatusMethodNotAllowed:
		st = status.New(codes.Unimplemented, "method not allowed")

	case http.StatusTooManyRequests:
		st = status.New(codes.ResourceExhausted, "too many requests")

	case http.StatusInternalServerError:
		st = status.New(codes.Internal, "internal server error")

	case http.StatusServiceUnavailable:
		st = status.New(codes.Unavailable, "service unavailable")

	case http.StatusNoContent:
		st = status.New(codes.OutOfRange, "out of range")

	case http.StatusPreconditionFailed:
		st = status.New(codes.FailedPrecondition, "failed precondition")

	default:
		st = status.New(codes.Internal, "internal server error")

	}

	errId := uuid.NewString()

	g.outgoingMetadata.Set("x-error-id", errId)

	return g.outgoingMetadata, nil, st.Err()

}

func (g *grpcMethodBackend) End(context.Context) {

}
