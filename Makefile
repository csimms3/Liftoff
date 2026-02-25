.PHONY: help build run dev test clean db-up db-down db-reset deps health

help:
	@echo "Liftoff Development Commands"
	@echo ""
	@echo "Quick start:"
	@echo "  ./scripts/boot.sh  - Start backend + frontend together"
	@echo ""
	@echo "Backend:"
	@echo "  build    - Build the Go binary"
	@echo "  run      - Build and run the server"
	@echo "  dev      - Run the server without building (go run .)"
	@echo "  test     - Run all tests"
	@echo "  clean    - Remove build artifacts"
	@echo "  deps     - Tidy and download Go dependencies"
	@echo ""
	@echo "Database (PostgreSQL via Docker):"
	@echo "  db-up    - Start PostgreSQL with Docker"
	@echo "  db-down  - Stop PostgreSQL"
	@echo "  db-reset - Reset PostgreSQL (stop + start)"
	@echo ""
	@echo "  health   - Check server health endpoint"

# Backend
build:
	@echo "Building..."
	cd backend && go build -o bin/liftoff .

run: build
	@echo "Starting server..."
	./backend/bin/liftoff

dev:
	@echo "Starting server (dev)..."
	cd backend && go run .

test:
	@echo "Running backend tests..."
	cd backend && go test ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf backend/bin/
	cd backend && go clean

deps:
	@echo "Tidying dependencies..."
	cd backend && go mod tidy && go mod download

# PostgreSQL via Docker
db-up:
	@echo "Starting PostgreSQL..."
	docker-compose up -d postgres
	@until docker-compose exec -T postgres pg_isready -U postgres; do sleep 1; done
	@echo "Database ready."

db-down:
	docker-compose down

db-reset: db-down db-up

# Misc
health:
	curl -sf http://localhost:8080/health && echo "OK" || echo "Server not responding"
