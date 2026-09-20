default:
    @just --list

# Generate local trusted TLS certificates using mkcert
certs:
    mkdir -p .certs
    mkcert -install
    mkcert -cert-file .certs/cert.pem -key-file .certs/key.pem localhost 127.0.0.1 ::1

# Install all toolchains via mise, generate TLS certs, and install frontend dependencies
setup:
    mise install
    just certs
    cd web && pnpm install

# Regenerate Protobuf, Connect stubs, OpenAPI specs, TypeScript types, and descriptor image
generate:
    buf generate
    buf build -o gen/image.binpb
    sqlc generate
    mkdir -p web/public && cp gen/openapi/pet/v1/pet.openapi.yaml web/public/openapi.yaml

# Build Go binary and Vite React web frontend
build:
    go build -v ./...
    cd web && pnpm build

# Run unit tests (auth & protovalidate)
test:
    go test -v ./internal/...

# Run golangci-lint with gosec security analysis
lint:
    golangci-lint run

# Run govulncheck vulnerability scanner
vulncheck:
    govulncheck ./...

# Run all quality & security checks (lint, vulncheck, unit tests, frontend mock tests)
check: lint vulncheck test test-web

# Run all tests (including integration tests)
test-integration:
    go test -v ./test/...

# Start PostgreSQL container via docker-compose
up:
    docker-compose up -d postgres

# Stop PostgreSQL container
down:
    docker-compose down

# Run database migrations up
migrate-up:
    go run ./cmd/migrate up

# Roll back the most recent database migration
migrate-down:
    go run ./cmd/migrate down

# Check database migration status
migrate-status:
    go run ./cmd/migrate status

# Run the Go microservice
run:
    go run ./cmd/server

# Run FauxRPC mock server with HTTPS, protobuf descriptor image, OpenAPI specification, and normal stubs
fauxrpc:
    fauxrpc run --schema=gen/image.binpb,gen/openapi/pet/v1/pet.openapi.yaml --stubs=stubs/normal --addr=127.0.0.1:8080 --https --cert=.certs/cert.pem --cert-key=.certs/key.pem --log-level=debug

# Run FauxRPC mock server configured with failure stubs to test error handling
fauxrpc-fail:
    fauxrpc run --schema=gen/image.binpb,gen/openapi/pet/v1/pet.openapi.yaml --stubs=stubs/failures --addr=127.0.0.1:8080 --https --cert=.certs/cert.pem --cert-key=.certs/key.pem

# Run Vite React frontend dev server against backend (Go server or FauxRPC on :8080)
web-dev:
    cd web && pnpm dev

# Run frontend tests against FauxRPC mock server
test-web:
    cd web && pnpm test
