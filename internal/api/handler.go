package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/example/test-go/internal/db"
)

// ResponseBody defines the shape of the JSON body returned by the handler.
type ResponseBody struct {
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

// HandleRequest is the Lambda entry point that receives HTTP API requests from API Gateway.
func HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method != http.MethodGet {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"message":"method not allowed"}`,
		}, nil
	}

	mysqlStatus := "ok"

	if conn, err := db.Connection(); err != nil {
		mysqlStatus = "error: " + err.Error()
	} else if err := conn.PingContext(ctx); err != nil {
		mysqlStatus = "error: " + err.Error()
	}

	body := ResponseBody{
		Message:   "Lambda skeleton is running",
		Timestamp: time.Now().UTC(),
		Metadata: map[string]string{
			"stage":         req.RequestContext.Stage,
			"requestId":     req.RequestContext.RequestID,
			"mysqlDatabase": os.Getenv("MYSQL_DATABASE"),
			"mysqlStatus":   mysqlStatus,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{}, errors.New("failed to marshal response body")
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(jsonBody),
	}, nil
}
