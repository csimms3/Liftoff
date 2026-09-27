.PHONY: help build run dev test clean db-up db-down deps health

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
	@echo "  test     - Run all backend tests (starts the dev database)"
	@echo "  clean    - Remove build artifacts"
	@echo "  deps     - Tidy and download Go dependencies"
	@echo ""
	@echo "Database (project-local PostgreSQL in .pgdata/, see scripts/dev-db.sh):"
	@echo "  db-up    - Start it"
	@echo "  db-down  - Stop it"
	@echo ""
	@echo "  health   - Check server health endpoint"

# Backend
build:
	@echo "Building..."
	cd backend && go build -o bin/liftoff .

run: build
	@echo "Starting server..."
	cd backend && ./bin/liftoff

dev:
	@echo "Starting server (dev)..."
	cd backend && go run .

test:
	@echo "Running backend tests..."
	./scripts/dev-db.sh start
	cd backend && LIFTOFF_TEST_DATABASE_URL="$$(../scripts/dev-db.sh url test)" go test ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf backend/bin/
	cd backend && go clean

deps:
	@echo "Tidying dependencies..."
	cd backend && go mod tidy && go mod download

# Project-local PostgreSQL (scripts/dev-db.sh)
db-up:
	./scripts/dev-db.sh start

db-down:
	./scripts/dev-db.sh stop

# Misc
health:
	curl -sf http://localhost:8080/health && echo "OK" || echo "Server not responding"
