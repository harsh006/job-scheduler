.PHONY: install-prereqs run stop clean build test test-race fmt lint migrate logs

API_BASE  = http://localhost:8080/api/v1
API_TOKEN = dev-secret

# ─────────────────────────────────────────────
# Prerequisites
# ─────────────────────────────────────────────

## install-prereqs: Install all tools needed to run this project (macOS + Homebrew)
install-prereqs:
	@echo "→ Checking Homebrew..."
	@which brew > /dev/null 2>&1 || \
		(echo "✗ Homebrew not found. Install it first: https://brew.sh" && exit 1)
	@echo "→ Checking Colima (Docker runtime)..."
	@which colima > /dev/null 2>&1 || brew install colima
	@echo "→ Checking Docker CLI..."
	@which docker > /dev/null 2>&1 || brew install docker
	@echo "→ Checking Docker Compose..."
	@docker compose version > /dev/null 2>&1 || brew install docker-compose
	@echo "→ Checking Go..."
	@which go > /dev/null 2>&1 || brew install go
	@echo "→ Checking MySQL client (for make migrate)..."
	@which mysql > /dev/null 2>&1 || brew install mysql-client
	@echo ""
	@echo "✓ All prerequisites installed."

# ─────────────────────────────────────────────
# Run (full stack via Docker)
# ─────────────────────────────────────────────

## run: Start Colima + MySQL + app, apply migrations, print ready message
run: _colima-start _docker-up _wait-db _migrate _wait-app
	@echo ""
	@echo "╔══════════════════════════════════════════════╗"
	@echo "║  Job Scheduler is ready                      ║"
	@echo "║                                              ║"
	@echo "║  Base URL : http://localhost:8080/api/v1     ║"
	@echo "║  API Key  : dev-secret                       ║"
	@echo "║                                              ║"
	@echo "║  make logs   → tail app logs                 ║"
	@echo "║  make stop   → stop containers               ║"
	@echo "╚══════════════════════════════════════════════╝"

_colima-start:
	@colima status > /dev/null 2>&1 && echo "→ Colima already running" || \
		(echo "→ Starting Colima..." && colima start)

_docker-up:
	@echo "→ Building and starting containers..."
	@docker compose up --build -d 2>&1 | grep -E "Built|Started|Running|Healthy|error" || true

_wait-db:
	@echo "→ Waiting for MySQL to be ready..."
	@for i in $$(seq 1 30); do \
		docker compose exec mysql mysqladmin ping -h localhost -uroot -proot --silent > /dev/null 2>&1 \
			&& echo "✓ MySQL ready" && exit 0; \
		sleep 2; \
	done; \
	echo "✗ MySQL did not become ready in time" && exit 1

_migrate:
	@echo "→ Applying migrations..."
	@docker compose exec mysql mysql -uroot -proot scheduler \
		< migrations/001_init.sql 2>/dev/null || true
	@docker compose exec mysql mysql -uroot -proot scheduler \
		< migrations/002_add_missed_status.sql 2>/dev/null || true
	@echo "✓ Migrations applied"

_wait-app:
	@echo "→ Waiting for app to be ready..."
	@for i in $$(seq 1 20); do \
		docker compose exec app wget -qO- http://localhost:8080/api/v1/jobs \
			--header="Authorization: Bearer dev-secret" > /dev/null 2>&1 \
			&& echo "✓ App ready" && exit 0; \
		sleep 2; \
	done; \
	echo "✗ App did not become ready in time. Check: make logs" && exit 1

# ─────────────────────────────────────────────
# Lifecycle
# ─────────────────────────────────────────────

## stop: Stop all containers
stop:
	docker compose down

## clean: Stop containers and delete all data volumes
clean:
	docker compose down -v
	@echo "✓ Containers and volumes removed"

## logs: Tail app container logs
logs:
	docker compose logs -f app

## migrate: Apply DB migrations (containers must be running)
migrate: _wait-db _migrate

# ─────────────────────────────────────────────
# Local development (without Docker)
# ─────────────────────────────────────────────

## build: Compile binary to bin/scheduler
build:
	go build -o bin/scheduler ./cmd/main.go

## dev: Run locally (requires .env with DB_DSN and API_KEY set)
dev:
	@test -f .env || (cp .env.example .env && echo "Created .env from .env.example — fill in DB_DSN and API_KEY")
	go run ./cmd/main.go

## test: Run all unit tests
test:
	go test ./... -v -count=1

## test-race: Run tests with race detector
test-race:
	go test -race ./... -count=1

## fmt: Format all Go source files
fmt:
	gofmt -w .

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run ./...

# ─────────────────────────────────────────────
# Help
# ─────────────────────────────────────────────

help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## /  /'
