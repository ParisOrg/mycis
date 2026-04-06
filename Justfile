set dotenv-load := true

app := "mycis"

default:
  @just --list

generate:
  sqlc generate

assets:
  npm run build

build: generate assets
  mkdir -p bin
  go build -o bin/{{app}} ./cmd/app

test:
  go test ./...

migrate:
  go run ./cmd/app migrate

seed:
  go run ./cmd/app seed-framework -slug cis-v8-1

seed-force:
  go run ./cmd/app seed-framework -slug cis-v8-1 -force

run:
  go run ./cmd/app web

dev:
  air
