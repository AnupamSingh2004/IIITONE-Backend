.PHONY: run test build up down migrate-up migrate-down

run:
	go run ./cmd/server

test:
	go test ./... -v

build:
	go build -o bin/server ./cmd/server

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	migrate -path ./migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path ./migrations -database "$$DATABASE_URL" down 1
