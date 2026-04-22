include .env
export

.PHONY: run dev up down migrate-up migrate-down migrate-create build

run:
	go run ./cmd/server

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path migrations \
		-database "$(DATABASE_URL)" up

migrate-down:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path migrations \
		-database "$(DATABASE_URL)" down 1

migrate-create:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate \
		-path migrations create -ext sql -dir migrations -seq $(name)

build:
	go build -o bin/server ./cmd/server
