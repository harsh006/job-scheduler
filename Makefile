.PHONY: build run test lint docker-up docker-down migrate fmt

# Build binary locally
build:
	go build -o bin/scheduler ./cmd/main.go

# Run locally (requires .env or env vars to be set)
run:
	go run ./cmd/main.go

# Run all tests
test:
	go test ./... -v -count=1

# Run tests with race detector
test-race:
	go test -race ./... -count=1

# Format code
fmt:
	gofmt -w .

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# Start Docker services (MySQL + app)
docker-up:
	docker compose up --build -d

# Stop Docker services
docker-down:
	docker compose down

# Remove Docker services and volumes
docker-clean:
	docker compose down -v

# Apply DB migration (requires mysql client or runs via Docker)
migrate:
	docker compose exec mysql mysql -uroot -proot scheduler < migrations/001_init.sql

# Tail app logs
logs:
	docker compose logs -f app
