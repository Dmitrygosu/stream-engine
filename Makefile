APP_NAME := stream-engine
BUILD_DIR := bin
API_MAIN := cmd/api/main.go
WORKER_MAIN := cmd/worker/main.go

.PHONY: all build run-api run-worker test clean docker-up docker-down lint migrate-up

all: build

build:
	@echo "Building API..."
	go build -o $(BUILD_DIR)/api $(API_MAIN)
	@echo "Building Worker..."
	go build -o $(BUILD_DIR)/worker $(WORKER_MAIN)

run-api:
	@echo "Running API..."
	go run $(API_MAIN)

run-worker:
	@echo "Running Worker..."
	go run $(WORKER_MAIN)

test:
	@echo "Running tests..."
	go test -v -race ./...

docker-up:
	@echo "Starting services..."
	docker-compose up -d

docker-down:
	@echo "Stopping services..."
	docker-compose down

lint:
	@echo "Running linter..."
	golangci-lint run

migrate-up:
	@echo "Migrations applied via initdb or golang-migrate in production"
