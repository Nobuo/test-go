package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// HandlerFunc is a function that processes API Gateway HTTP API requests.
type HandlerFunc func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

// Start launches a minimal Lambda runtime that communicates with the Lambda Runtime API.
// It is intentionally scoped to HTTP API events to keep the skeleton lightweight.
func Start(handler HandlerFunc) {
	runtimeAPI := os.Getenv("AWS_LAMBDA_RUNTIME_API")
	if runtimeAPI == "" {
		log.Fatal("AWS_LAMBDA_RUNTIME_API is not set; are you running inside Lambda?")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	for {
		eventBytes, requestID, err := nextInvocation(client, runtimeAPI)
		if err != nil {
			log.Fatalf("failed to receive invocation: %v", err)
		}

		var event events.APIGatewayV2HTTPRequest
		if err := json.Unmarshal(eventBytes, &event); err != nil {
			reportError(client, runtimeAPI, requestID, fmt.Errorf("failed to decode event: %w", err))
			continue
		}

		ctx := context.Background()
		ctx = context.WithValue(ctx, requestContextKey{}, requestID)
		response, err := handler(ctx, event)
		if err != nil {
			reportError(client, runtimeAPI, requestID, err)
			continue
		}

		if err := postResponse(client, runtimeAPI, requestID, response); err != nil {
			log.Fatalf("failed to post response: %v", err)
		}
	}
}

type requestContextKey struct{}

func nextInvocation(client *http.Client, runtimeAPI string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/2018-06-01/runtime/invocation/next", runtimeAPI), nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status from runtime API: %s - %s", resp.Status, string(body))
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	requestID := resp.Header.Get("Lambda-Runtime-Aws-Request-Id")
	if requestID == "" {
		return nil, "", errors.New("missing Lambda-Runtime-Aws-Request-Id header")
	}

	return payload, requestID, nil
}

func reportError(client *http.Client, runtimeAPI, requestID string, cause error) {
	errPayload := map[string]string{
		"errorMessage": cause.Error(),
		"errorType":    "HandlerError",
	}
	payload, _ := json.Marshal(errPayload)

	url := fmt.Sprintf("http://%s/2018-06-01/runtime/invocation/%s/error", runtimeAPI, requestID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("failed to create error request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("failed to report error: %v", err)
		return
	}
	resp.Body.Close()
}

func postResponse(client *http.Client, runtimeAPI, requestID string, response events.APIGatewayV2HTTPResponse) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/2018-06-01/runtime/invocation/%s/response", runtimeAPI, requestID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
