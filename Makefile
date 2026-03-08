.PHONY: all build run dev deps tidy proto docker-up docker-down client test clean

## ─── Development ──────────────────────────────────────────────────────────────

# Download dependencies
deps:
	go mod download && go mod tidy

# Tidy modules
tidy:
	go mod tidy

# Build all binaries
build:
	go build -o bin/coordinator ./cmd/coordinator
	go build -o bin/client ./cmd/client

# Run the coordinator server (development mode)
run:
	HEARTBEAT_INTERVAL=2m go run ./cmd/coordinator

# Run with short heartbeat for testing (30 seconds)
dev:
	HEARTBEAT_INTERVAL=30s go run ./cmd/coordinator

# Run TUI client
client:
	go run ./cmd/client --server http://localhost:8080

# Run TUI client with custom name
client-new:
	go run ./cmd/client --server http://localhost:8080 --player "$(PLAYER)" --city "$(CITY)"

## ─── Docker ───────────────────────────────────────────────────────────────────

# Start all services (PostgreSQL, Redis, Coordinator)
docker-up:
	docker-compose up -d

# Stop all services
docker-down:
	docker-compose down

# View logs
docker-logs:
	docker-compose logs -f coordinator

# Rebuild and restart coordinator
docker-rebuild:
	docker-compose up -d --build coordinator

## ─── Proto ────────────────────────────────────────────────────────────────────

# Generate gRPC Go code from proto definitions
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       proto/cities.proto

## ─── Testing ──────────────────────────────────────────────────────────────────

test:
	go test ./...

# Trigger a manual heartbeat (for testing)
trigger-heartbeat:
	curl -s -X POST http://localhost:8080/api/debug/heartbeat | jq .

# Check game state
game-state:
	curl -s http://localhost:8080/api/game/state | jq .

# List all cities
cities:
	curl -s http://localhost:8080/api/cities | jq .

## ─── Cleanup ──────────────────────────────────────────────────────────────────

clean:
	rm -rf bin/
	docker-compose down -v

## ─── Help ─────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "CITIES — Social Simulator"
	@echo "========================="
	@echo ""
	@echo "Development:"
	@echo "  make deps          Download Go dependencies"
	@echo "  make dev           Run coordinator (30s heartbeat for testing)"
	@echo "  make run           Run coordinator (2min heartbeat)"
	@echo "  make client        Run TUI client"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up     Start all services"
	@echo "  make docker-down   Stop all services"
	@echo ""
	@echo "Testing:"
	@echo "  make trigger-heartbeat   Force a heartbeat cycle"
	@echo "  make game-state          Show current game state"
	@echo "  make cities              List all cities"
	@echo ""
