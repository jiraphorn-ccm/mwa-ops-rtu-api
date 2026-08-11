SHELL := /bin/sh

APP        := rtu-api
BIN_DIR    := bin
BINARY     := $(BIN_DIR)/server
VERSION    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X main.version=$(VERSION)

MIGRATIONS := migrations

# Database connection for golang-migrate CLI. Values match .env.example;
# override on the command line or via a local .env file.
DB_HOST     ?= 127.0.0.1
DB_PORT     ?= 5432
DB_USER     ?= rtu
DB_PASSWORD ?= rtu_password
DB_NAME     ?= rtu
DB_SSLMODE  ?= disable

DATABASE_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

# golang-migrate keeps schema_migrations in public. search_path has to be
# pinned because the DB role is also named "rtu": with the default
# `"$$user", public` the table would jump into the rtu schema once it exists.
MIGRATE_URL := $(DATABASE_URL)&search_path=public

.DEFAULT_GOAL := help

## help: list the available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

## tools: install the code generation and migration tooling
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

## generate: regenerate the type-safe query layer from queries/ and migrations/
generate:
	sqlc generate

## sqlc-vet: check the queries against the schema without writing code
sqlc-vet:
	sqlc vet

## build: compile the server binary
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server

## run: start the server from source
run:
	go run ./cmd/server

## test: run the test suite with the race detector
test:
	go test -race -count=1 ./...

## test-integration: run integration tests (requires DB_* in .env)
test-integration:
	go test -race -count=1 -tags=integration ./...

## cover: run the tests and open the coverage summary
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy

## fmt: format the source tree
fmt:
	go fmt ./...

## vet: run the standard static checks
vet:
	go vet ./...

## lint: run fmt, vet and the query checker together
lint: fmt vet sqlc-vet

## migrate-up: apply every pending migration
migrate-up:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" up

## migrate-down: roll back the most recent migration
migrate-down:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" down 1

## migrate-status: show the applied migration version
migrate-status:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" version

## migrate-force: clear a dirty state, e.g. make migrate-force V=1
migrate-force:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" force $(V)

## migrate-create: scaffold a migration pair, e.g. make migrate-create NAME=add_alarms
migrate-create:
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(NAME)

.PHONY: help tools generate sqlc-vet build run test test-integration cover tidy fmt vet lint \
        migrate-up migrate-down migrate-status migrate-force migrate-create
