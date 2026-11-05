BUILD_DIR := bin

.PHONY: all build run-api run-worker test lint up down

all: build

build:
	go build -trimpath -o $(BUILD_DIR)/api ./cmd/api
	go build -trimpath -o $(BUILD_DIR)/worker ./cmd/worker

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

test:
	go test -race ./...

lint:
	golangci-lint run

# Postgres, Kafka (with topics), the API and the worker.
up:
	docker compose up -d --build

down:
	docker compose down
