.PHONY: run build migrate test lint tidy clean help

# Default target
help:
	@echo "AOMS - AI Operational Memory System"
	@echo ""
	@echo "Available commands:"
	@echo "  make run     - Run the API server"
	@echo "  make build   - Build the binary"
	@echo "  make migrate - Run database migrations"
	@echo "  make test   - Run tests"
	@echo "  make lint   - Run linter"
	@echo "  make tidy   - tidy Go modules"
	@echo "  make clean  - Clean binaries"

# Run the API server
run:
	go run ./cmd/api

# Build the binary
build:
	go build -o bin/aoms ./cmd/api

# Run migrations
migrate:
	@echo "Running migrations..."
	@for f in migrations/*.sql; do \
		echo "Applying $$f"; \
		psql $$(echo $$DATABASE_URL) -f $$f; \
	done

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run

# Tidy Go modules
tidy:
	go mod tidy
	go mod verify

# Clean binaries
clean:
	rm -rf bin/