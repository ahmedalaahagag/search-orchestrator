.PHONY: build run test fmt lint clean docker-up docker-down seed

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

docker-up:
	docker compose up -d

docker-down:
	docker compose down

seed:
	./scripts/setup-index.sh
