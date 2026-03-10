.PHONY: build run test fmt lint clean

BINARY=bin/search-orchestrator

build:
	go build -o $(BINARY) .

run:
	go run . http

test:
	go test ./... -v -race

fmt:
	gofmt -s -w .
	goimports -w .

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
