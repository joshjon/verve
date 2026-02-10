.PHONY: all build build-server build-worker build-agent build-agent-no-cache run-server run-server-pg run-worker test-task clean tidy ui-install ui-dev ui-build db-up db-down db-logs

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

# Run components
run-server: build-server
	./bin/server

run-worker: build-worker
	./bin/worker

# Create a test task
test-task:
	curl -X POST http://localhost:8080/api/v1/tasks \
		-H "Content-Type: application/json" \
		-d '{"description":"Init project with hello world main function using a plain bash script"}'

# List all tasks
list-tasks:
	curl -s http://localhost:8080/api/v1/tasks | jq .

# Get task by ID (usage: make get-task ID=tsk_xxx)
get-task:
	curl -s http://localhost:8080/api/v1/tasks/$(ID) | jq .

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

# Database commands
db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

# Run server with PostgreSQL
run-server-pg: build-server db-up
	DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" ./bin/server
