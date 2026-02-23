.PHONY: help build run test clean db-up db-down db-reset

# Default target
help:
	@echo "Liftoff Backend Development Commands"
	@echo ""
	@echo "Database:"
	@echo "  db-up      - Start PostgreSQL database with Docker"
	@echo "  db-down    - Stop PostgreSQL database"
	@echo "  db-reset   - Reset database (stop, start, migrate)"
	@echo ""
	@echo "Development:"
	@echo "  build      - Build the Go application"
	@echo "  run        - Run the server (requires database)"
	@echo "  test       - Run tests"
	@echo "  clean      - Clean build artifacts"
	@echo ""
	@echo "Example workflow:"
	@echo "  make db-up && make run"

# Database commands
db-up:
	@echo "Starting PostgreSQL database..."
	docker-compose up -d postgres
	@echo "Waiting for database to be ready..."
	@until docker-compose exec -T postgres pg_isready -U postgres; do sleep 1; done
	@echo "Database is ready!"

db-down:
	@echo "Stopping PostgreSQL database..."
	docker-compose down

db-reset: db-down db-up
	@echo "Database reset complete!"

# Development commands
build:
	@echo "Building application..."
	go build -o bin/liftoff main.go

run: build
	@echo "Starting Liftoff server..."
	./bin/liftoff

dev:
	@echo "Starting Liftoff server in development mode..."
	go run main.go

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Database migration
migrate:
	@echo "Running database migrations..."
	psql -h localhost -U postgres -d liftoff -f migrations/001_initial_schema.sql

# Health check
health:
	@echo "Checking application health..."
	curl -f http://localhost:8080/health || echo "Server not responding"
