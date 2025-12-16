.PHONY: build run test clean generate migrate

# Build the server
build:
	go build -o bin/server ./cmd/server

# Run the server
run:
	go run ./cmd/server

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f *.db

# Generate sqlc code
generate:
	sqlc generate

# Install sqlc (if needed)
install-sqlc:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Initialize database with migrations
migrate:
	sqlite3 books.db < db/migrations/001_init.sql

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# All checks
check: fmt test lint
