# DATABASE_URL and other settings come from .env (copy from .env.example).
-include .env
export

MIGRATIONS := db/migrations
GOOSE      := go tool goose -dir $(MIGRATIONS)

.PHONY: up down migrate migrate-down migrate-new sqlc api run test lint keys storage-up

up: ## Start Postgres and Mailpit, wait until Postgres is healthy
	docker compose up -d --wait

storage-up: ## Provision optional private AIStor and verify signed upload/read/range
	bash scripts/storage-dev.sh

down: ## Stop the dev services (data volume is kept)
	docker compose down

migrate: ## Apply all migrations
	$(GOOSE) postgres "$(DATABASE_URL)" up

migrate-down: ## Roll back the last migration
	$(GOOSE) postgres "$(DATABASE_URL)" down

migrate-new: ## Create a new SQL migration: make migrate-new name=add_foo
	@test -n "$(name)" || (echo "usage: make migrate-new name=<name>" && exit 1)
	$(GOOSE) -s create $(name) sql

sqlc: ## Generate Go code from db/queries
	go tool sqlc generate

api: ## Generate Go models from api/openapi.yaml
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

run: ## Run the server on the host
	go run ./cmd/server

seed-library: ## Add the CC0 sample e-book to one school: make seed-library org=<org-id>
	@test -n "$(org)" || (echo "usage: make seed-library org=<org-id>" && exit 1)
	go run ./cmd/server seed-library $(org)

test: ## Run all tests (PostgreSQL integration tests need TEST_DATABASE_URL)
	go test -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

keys: ## Print random values for JWT_PRIVATE_KEY and OTP_HMAC_KEY
	@echo "JWT_PRIVATE_KEY=$$(openssl rand -base64 32)"
	@echo "OTP_HMAC_KEY=$$(openssl rand -base64 32)"
