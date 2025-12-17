# builder
FROM golang:1.22-alpine AS builder

WORKDIR /app

# tools
RUN apk add --no-cache git make

# deps
COPY go.mod go.sum ./
RUN go mod download

# src
COPY . .

# build
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker/main.go

# prod
FROM alpine:3.19

WORKDIR /app

# cp bins
COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/config.yaml ./config.yaml

# port
EXPOSE 8080

# run
CMD ["/app/api"]
