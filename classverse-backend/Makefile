# DATABASE_URL and other settings come from .env (copy from .env.example).
-include .env
export

MIGRATIONS := db/migrations
GOOSE      := go tool goose -dir $(MIGRATIONS)

.PHONY: up down migrate migrate-down migrate-new sqlc run test lint

up: ## Start Postgres and Mailpit, wait until Postgres is healthy
	docker compose up -d --wait

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

run: ## Run the server on the host
	go run ./cmd/server

test: ## Run all tests
	go test -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...
