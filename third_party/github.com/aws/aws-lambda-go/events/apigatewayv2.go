package events

import "encoding/json"

// APIGatewayV2HTTPRequest represents the payload from API Gateway HTTP APIs.
type APIGatewayV2HTTPRequest struct {
	Version               string                         `json:"version"`
	RouteKey              string                         `json:"routeKey"`
	RawPath               string                         `json:"rawPath"`
	RawQueryString        string                         `json:"rawQueryString"`
	Cookies               []string                       `json:"cookies"`
	Headers               map[string]string              `json:"headers"`
	QueryStringParameters map[string]string              `json:"queryStringParameters"`
	RequestContext        APIGatewayV2HTTPRequestContext `json:"requestContext"`
	Body                  string                         `json:"body"`
	IsBase64Encoded       bool                           `json:"isBase64Encoded"`
}

// APIGatewayV2HTTPRequestContext describes the request context.
type APIGatewayV2HTTPRequestContext struct {
	RouteKey  string                                        `json:"routeKey"`
	AccountID string                                        `json:"accountId"`
	Stage     string                                        `json:"stage"`
	RequestID string                                        `json:"requestId"`
	HTTP      APIGatewayV2HTTPRequestContextHTTPDescription `json:"http"`
}

// APIGatewayV2HTTPRequestContextHTTPDescription describes HTTP details.
type APIGatewayV2HTTPRequestContextHTTPDescription struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Protocol  string `json:"protocol"`
	SourceIP  string `json:"sourceIp"`
	UserAgent string `json:"userAgent"`
}

// APIGatewayV2HTTPResponse is the response payload expected by API Gateway HTTP APIs.
type APIGatewayV2HTTPResponse struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	Cookies           []string            `json:"cookies"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders,omitempty"`
}

// MarshalJSON ensures empty maps are encoded as empty JSON objects.
func (r APIGatewayV2HTTPResponse) MarshalJSON() ([]byte, error) {
	type Alias APIGatewayV2HTTPResponse
	alias := Alias(r)
	if alias.Headers == nil {
		alias.Headers = map[string]string{}
	}
	if alias.MultiValueHeaders == nil {
		alias.MultiValueHeaders = map[string][]string{}
	}
	return json.Marshal(alias)
}
