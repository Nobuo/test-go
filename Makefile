.PHONY: build-local run-local docker-build test

build-local:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/bootstrap main.go

run-local:
	go run ./cmd/local

docker-build:
	docker buildx build --platform linux/arm64 -t lambda-skeleton:latest .

test:
	go test ./...
