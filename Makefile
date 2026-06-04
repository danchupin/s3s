.PHONY: fmt vet lint test test-integration build run check-readonly all

all: fmt vet lint test

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

build:
	go build -o bin/s3s ./cmd/s3s

run:
	go run ./cmd/s3s

check-readonly:
	./scripts/check-readonly.sh
