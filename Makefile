.PHONY: build test test-unit test-integration lint run dev dev-down install-tools clean tidy

build:
	@echo "Building server..."
	go build -o bin/server ./cmd/server

test:
	go test -v -race ./...

test-unit:
	go test -v -race -short ./...

test-integration:
	go test -v -race -run Integration ./tests/integration/...

lint:
	golangci-lint run

run:
	go run ./cmd/server/main.go

dev:
	docker compose up -d
	@echo "Starting dev services..."
	@echo "  Postgres: localhost:5432"
	@echo "  Redis: localhost:6379"
	@echo "  Kafka: localhost:9092"

dev-down:
	docker compose down
	@echo "Stopping dev services..."

clean:
	rm -rf bin/
	go clean

tidy:
	go mod tidy
	go mod verify

install-tools:
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
