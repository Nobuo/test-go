# Build the Lambda handler as a custom runtime binary for arm64.
FROM --platform=$BUILDPLATFORM golang:1.22 AS build
ARG TARGETARCH=arm64
ENV CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH
WORKDIR /src

COPY go.mod ./
COPY third_party ./third_party
RUN go list ./...

COPY . .
RUN go build -o /out/bootstrap ./main.go

# Package into the AWS Lambda provided.al2023 base image.
FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /out/bootstrap /var/runtime/bootstrap
ENTRYPOINT ["/var/runtime/bootstrap"]
