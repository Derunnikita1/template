.PHONY: generate migrate migrate-down run test lint build

ifneq (,$(wildcard .env))
include .env
export
endif

generate:
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

migrate:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" down

run:
	go run ./cmd/trip-service

test:
	go test -race ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/trip-service ./cmd/trip-service
