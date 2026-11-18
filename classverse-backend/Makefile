# DATABASE_URL and other settings come from .env (copy from .env.example).
-include .env

# Only host run/seed commands inherit locally provisioned storage; explicit config wins as a unit.
ifneq ($(filter run seed-library,$(MAKECMDGOALS)),)
ifeq ($(strip $(STORAGE_ENDPOINT)$(STORAGE_BUCKET)$(STORAGE_ACCESS_KEY)$(STORAGE_SECRET_KEY)$(STORAGE_SECURE)),)
ifneq ($(wildcard ../.config/storage-app.env),)
include ../.config/storage-app.env
STORAGE_ENDPOINT := 127.0.0.1:$(or $(STORAGE_PORT),9100)
STORAGE_BUCKET := materials
STORAGE_ACCESS_KEY := $(TEST_STORAGE_ACCESS_KEY)
STORAGE_SECRET_KEY := $(TEST_STORAGE_SECRET_KEY)
STORAGE_SECURE := false
endif
endif
endif
export

MIGRATIONS := db/migrations
GOOSE      := go tool goose -dir $(MIGRATIONS)

.PHONY: up down migrate migrate-down migrate-new sqlc api run seed-library test lint keys storage-up

up: ## Start Postgres and Mailpit, wait until Postgres is healthy
	docker compose up -d --wait

storage-up: ## Provision optional private AIStor and verify signed upload/read/range
	bash scripts/storage-dev.sh

down: ## Stop the dev services (data volume is kept)
	docker compose down

migrate: ## Apply migrations through canonical Content Studio preflight
	bash scripts/migrate.sh

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
