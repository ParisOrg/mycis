set dotenv-load := true

app := "mycis"
framework_slugs := "cis-v8-1 nist-csf-2-0 iso-27001-2022 iso-27002-2022 soc2-2017 nis2-2022 dora-2025 gdpr-2018 eu-ai-act-2024"

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
  for slug in {{framework_slugs}}; do go run ./cmd/app seed-framework -slug "$slug"; done

seed-force:
  for slug in {{framework_slugs}}; do go run ./cmd/app seed-framework -slug "$slug" -force; done

run:
  go run ./cmd/app web

dev:
  air
