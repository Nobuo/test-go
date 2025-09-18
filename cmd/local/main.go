package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"github.com/example/test-go/internal/api"
)

func main() {
	addr := ":9000"
	if v := os.Getenv("LOCAL_API_PORT"); v != "" {
		if !strings.HasPrefix(v, ":") {
			v = ":" + v
		}
		addr = v
	}

	log.Printf("Starting local development server on %s", addr)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		event := events.APIGatewayV2HTTPRequest{
			RawPath:               r.URL.Path,
			RawQueryString:        r.URL.RawQuery,
			Headers:               map[string]string{},
			QueryStringParameters: map[string]string{},
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				Stage: r.Header.Get("X-Stage"),
				HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
					Method:    r.Method,
					Path:      r.URL.Path,
					Protocol:  r.Proto,
					SourceIP:  r.RemoteAddr,
					UserAgent: r.UserAgent(),
				},
			},
		}
		for k, values := range r.Header {
			if len(values) > 0 {
				event.Headers[strings.ToLower(k)] = values[0]
			}
		}

		ctx := context.Background()
		resp, err := api.HandleRequest(ctx, event)
		if err != nil {
			log.Printf("handler error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.StatusCode)
		if _, err := w.Write([]byte(resp.Body)); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	})

	log.Fatal(http.ListenAndServe(addr, nil))
}
