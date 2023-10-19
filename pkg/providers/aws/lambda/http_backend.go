package lambda

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/google/uuid"
	"github.com/ultraviolet-black/cruiser/pkg/observability"
	awspb "github.com/ultraviolet-black/cruiser/pkg/proto/providers/aws"
)

type httpBackend struct {
	lambdaCli *lambda.Client

	backend *awspb.LambdaBackend
}

func NewHttpBackend(lambdaCli *lambda.Client, backend *awspb.LambdaBackend) http.Handler {
	return &httpBackend{
		lambdaCli: lambdaCli,
		backend:   backend,
	}
}

func wrapHttpError(w http.ResponseWriter, err error) {

	errorId := uuid.NewString()

	observability.Log.Errorw("error handling request", "error", err, "errorId", errorId)

	w.Header().Add("x-error-id", errorId)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(fmt.Sprintf("internal server error: %s", errorId)))

}

func (h *httpBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		wrapHttpError(w, err)
		return
	}

	req := &events.APIGatewayProxyRequest{
		HTTPMethod:                      r.Method,
		Path:                            r.URL.Path,
		MultiValueHeaders:               r.Header,
		MultiValueQueryStringParameters: r.URL.Query(),
		Body:                            string(body[:]),
		IsBase64Encoded:                 false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		wrapHttpError(w, err)
		return
	}

	result, err := h.lambdaCli.Invoke(r.Context(), &lambda.InvokeInput{
		FunctionName:   aws.String(h.backend.FunctionName),
		Qualifier:      aws.String(h.backend.Qualifier),
		InvocationType: types.InvocationTypeRequestResponse,
		Payload:        payload,
	})

	if result.FunctionError != nil {
		wrapHttpError(w, fmt.Errorf("function error: %s", *result.FunctionError))
		return
	}
	if err != nil {
		wrapHttpError(w, err)
		return
	}

	response := &events.APIGatewayProxyResponse{
		MultiValueHeaders: make(map[string][]string),
		Headers:           make(map[string]string),
	}
	if err := json.Unmarshal(result.Payload, response); err != nil {
		wrapHttpError(w, err)
		return
	}

	if response.MultiValueHeaders != nil {
		for key, vals := range response.MultiValueHeaders {
			w.Header()[key] = vals
		}
	}

	if response.Headers != nil {
		for key, val := range response.Headers {
			w.Header().Add(key, val)
		}
	}

	w.WriteHeader(response.StatusCode)

	responseBody := []byte(response.Body)

	if response.IsBase64Encoded {
		bodyBytes, err := base64.StdEncoding.DecodeString(response.Body)
		if err != nil {
			wrapHttpError(w, err)
			return
		}
		responseBody = bodyBytes
	}

	if _, err := w.Write(responseBody); err != nil {
		errorId := uuid.NewString()
		observability.Log.Errorw("error writing response", "error", err, "errorId", errorId)
	}

}
