# AWS Lambda Go Skeleton

This repository contains a Go skeleton project designed to run as an AWS Lambda function behind an API Gateway HTTP API. The layout favours container-based deployments on the `public.ecr.aws/lambda/provided:al2023` runtime for the `arm64` architecture while still enabling productive local development with hot reload support.

## Features

- **Amazon Linux 2023 base** – builds a custom runtime image from the `provided.al2023` base.
- **API Gateway HTTP API contract** – request/response structures align with the Lambda HTTP API payloads.
- **Arm64 friendly** – the Lambda `Dockerfile` compiles a statically linked arm64 binary.
- **Local hot reload** – `docker compose` + [`air`](https://github.com/cosmtrek/air) rebuild the local HTTP server on every file change.
- **MySQL ready** – ships with a MySQL 8.0 container for local development, plus a minimal MySQL driver that performs real
  connectivity checks from the handler (requires the `mysql_native_password` auth plugin).
- **Cross-platform** – tested on Windows (WSL2) + Docker CE and macOS with JetBrains IDEs.

## Project structure

```
.
├── cmd/local             # Local development HTTP server entrypoint
├── internal/api          # Lambda request handler implementation
├── main.go               # Lambda runtime entrypoint
├── third_party/          # Minimal vendor implementation of aws-lambda-go packages
├── Dockerfile            # Production/staging container image (arm64, AL2023)
├── Dockerfile.dev        # Local development image with hot reload tooling
├── docker-compose.yml    # Local stack (API + MySQL)
├── .air.toml             # air configuration for hot reload
├── .env.example          # Sample environment configuration
└── Makefile              # Helper commands
```

## Getting started

### Prerequisites

- Docker CE with buildx enabled (both WSL2 and macOS are supported).
- Docker Compose v2.
- Git + Go 1.22 if you plan to run commands on the host machine.

### Configure environment

1. Copy the sample environment file:

   ```bash
   cp .env.example .env
   ```

2. Adjust `API_PLATFORM`/`MYSQL_PLATFORM` if your Docker host cannot emulate the defaults. For example, set `API_PLATFORM=linux/arm64/v8` on Apple Silicon to avoid amd64 emulation, or `MYSQL_PLATFORM=linux/amd64` if your Windows host cannot run arm64 database images.

### Run the local stack with hot reload

```bash
docker compose up --build
```

- Source files are mounted into the container and recompiled via `air` on change.
- The local API server listens on [http://localhost:9000](http://localhost:9000) and mimics the API Gateway HTTP event contract.
- MySQL is reachable at `mysql:3306` from the API container. No reliance on `docker.internal` hostnames is required.

### Run without Docker (optional)

If you prefer to iterate directly on the host:

```bash
make run-local
```

This executes the same handler as the Lambda runtime but via the lightweight HTTP server under `cmd/local`.

### Build the Lambda container image

```bash
make docker-build
```

This produces an arm64 image that uses the Amazon Linux 2023 Lambda base. The resulting container exposes the standard Lambda runtime entrypoint.

### Deploying to AWS

1. Authenticate your Docker client with Amazon ECR (public or private).
2. Build and push the image:

   ```bash
   docker buildx build --platform linux/arm64 -t <account>.dkr.ecr.<region>.amazonaws.com/<repository>:<tag> --push .
   ```

3. Create or update an AWS Lambda function using the pushed image and integrate it with an API Gateway HTTP API.
4. Restrict the API Gateway to your private network (e.g. VPC link, resource policy) per your security requirements.

## Handler walkthrough

The Lambda handler is implemented in `internal/api/handler.go`. It demonstrates:

- Validating the HTTP method coming from API Gateway.
- Emitting a JSON response with metadata (stage, request ID, selected MySQL database, MySQL health check result).
- Using environment variables to supply database configuration values and opening a connection via the bundled MySQL driver.

## MySQL connectivity notes

- The repository vendors a trimmed down `github.com/go-sql-driver/mysql` implementation under `third_party/` so the project
  can compile without external downloads. The implementation focuses on connection establishment and health checks – extend it
  or replace it with the official module once network access is available.
- The development database container is started with `--default-authentication-plugin=mysql_native_password` to ensure
  compatibility with the bundled driver. Adjust the command if you migrate to the upstream driver or a managed MySQL service.
- When running the API directly on the host (outside Docker), override `MYSQL_HOST=127.0.0.1` so the process connects through
  the published port instead of the internal Docker service name.

The local runtime in `cmd/local` repackages incoming HTTP requests into the same structure so you can debug locally without SAM or LocalStack.

## Testing

Run Go unit tests (currently none are defined, but the command ensures the project compiles):

```bash
make test
```

## Next steps

- Replace the placeholder MySQL credentials with secrets management suitable for your organisation.
- Extend the handler logic and add modules under `internal/` to organise business code.
- Wire the Lambda into your CI/CD system (e.g. AWS CodeBuild, GitHub Actions) to build and publish the container image.
- Add automated tests as the project grows.
