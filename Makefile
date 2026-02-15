.PHONY: all build build-server build-worker build-agent build-agent-no-cache push-agent run-server run-server-pg run-worker test-task clean tidy generate up down logs ui-install ui-dev ui-build ui-build-go

AGENT_IMAGE = ghcr.io/joshjon/verve-agent

# Build all components
all: build-agent build

# Build Go binaries
build: build-server build-worker

build-server:
	go build -o bin/server ./cmd/server

build-worker:
	go build -o bin/worker ./cmd/worker

# Build agent Docker image
build-agent:
	docker build -t verve-agent:latest ./agent

build-agent-no-cache:
	docker build --no-cache -t verve-agent:latest ./agent

# Push agent image to GitHub Container Registry
# Usage:
#   make push-agent              # pushes :latest
#   make push-agent TAG=v0.1.0   # pushes :v0.1.0 and :latest
push-agent: build-agent
ifdef TAG
	docker tag verve-agent:latest $(AGENT_IMAGE):$(TAG)
	docker tag verve-agent:latest $(AGENT_IMAGE):latest
	docker push $(AGENT_IMAGE):$(TAG)
	docker push $(AGENT_IMAGE):latest
else
	docker tag verve-agent:latest $(AGENT_IMAGE):latest
	docker push $(AGENT_IMAGE):latest
endif

# Generate sqlc code
generate:
	go generate ./internal/postgres/... ./internal/sqlite/...

# Run locally (without Docker)
run-server: build-server
	./bin/server

run-server-pg: build-server
	docker compose up -d postgres
	@echo "Waiting for postgres..."
	@sleep 2
	DATABASE_URL=postgres POSTGRES_USER=verve POSTGRES_PASSWORD=verve POSTGRES_HOST_PORT=localhost:5432 POSTGRES_DATABASE=verve ./bin/server

run-worker: build-worker
	./bin/worker

# Docker Compose (full stack)
up:
	docker compose up -d

up-build:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

# Create a test task
test-task:
	curl -X POST http://localhost:7400/api/v1/tasks \
		-H "Content-Type: application/json" \
		-d '{"description":"Init project with hello world main function using a plain bash script"}'

# List all tasks
list-tasks:
	curl -s http://localhost:7400/api/v1/tasks | jq .

# Get task by ID (usage: make get-task ID=tsk_xxx)
get-task:
	curl -s http://localhost:7400/api/v1/tasks/$(ID) | jq .

# Tidy dependencies
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf ui/dist/
	docker rmi verve-agent:latest 2>/dev/null || true

# UI commands
ui-install:
	cd ui && pnpm install

ui-dev:
	cd ui && pnpm dev

ui-build:
	cd ui && pnpm build

ui-build-go:
	cd ui && BUILD_PATH="../internal/frontend/dist" VITE_API_ADDRESS="" pnpm build
