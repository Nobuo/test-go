package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/example/test-go/internal/api"
)

func main() {
	lambda.Start(api.HandleRequest)
}
