# WorldSignal - developer Makefile
# Run `make` or `make help` to see available targets.

# Default DATABASE_URL used by db-bootstrap if not provided in the environment.
DATABASE_URL ?= postgresql://worldsignal:worldsignal@localhost:5432/worldsignal?sslmode=disable

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

.PHONY: dev
dev: ## Start backend (:4800) + frontend (:5400) for local development
	./dev.sh

##@ Database

.PHONY: db-up
db-up: ## Start the Postgres container
	docker compose up -d postgres

.PHONY: db-bootstrap
db-bootstrap: ## Apply the base schema (uses DATABASE_URL or a local default)
	psql "$(DATABASE_URL)" -f backend/schema/schema.sql

##@ Backend (Go)

.PHONY: build
build: ## Build all Go packages
	cd backend && go build ./...

.PHONY: test
test: ## Run Go tests (serialized; needs Postgres)
	cd backend && go test ./... -p 1

.PHONY: cover
cover: ## Run Go tests with coverage and print the total
	cd backend && go test ./... -p 1 -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -n 1

.PHONY: fmt
fmt: ## Format Go code in place
	cd backend && gofmt -w .

.PHONY: fmt-check
fmt-check: ## Check Go formatting (fails if any file needs formatting)
	@cd backend && out="$$(gofmt -l .)"; \
		if [ -n "$$out" ]; then echo "These files need gofmt:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## Run go vet
	cd backend && go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint to be installed)
	cd backend && golangci-lint run

.PHONY: tidy
tidy: ## Tidy the Go module
	cd backend && go mod tidy

##@ Frontend (Node/Vite)

.PHONY: web-install
web-install: ## Install frontend dependencies (npm ci)
	cd frontend && npm ci

.PHONY: web-lint
web-lint: ## Lint the frontend
	cd frontend && npm run lint

.PHONY: web-test
web-test: ## Run frontend unit tests
	cd frontend && npm run test

.PHONY: web-build
web-build: ## Build the frontend for production
	cd frontend && npm run build

.PHONY: web-typecheck
web-typecheck: ## Typecheck the frontend
	cd frontend && npm run typecheck

##@ Aggregate / CI

.PHONY: check
check: fmt-check vet test web-lint web-typecheck web-test ## Run the full local CI suite

.PHONY: docker-build
docker-build: ## Build all docker images via compose
	docker compose build

.PHONY: clean
clean: ## Remove build artifacts
	rm -f backend/coverage.out
	rm -rf frontend/dist frontend/coverage frontend/playwright-report frontend/test-results
