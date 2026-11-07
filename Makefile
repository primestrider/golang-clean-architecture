# Makefile berisi perintah yang sering dipakai selama pengembangan.
# Di Windows, jalankan lewat Git Bash atau WSL. Perintah aslinya juga
# tercantum di README untuk pengguna PowerShell.

.PHONY: help tools proto build run-web run-worker test test-integration lint vuln mocks create-admin migrate-up migrate-down migrate-create compose-up compose-full compose-down

help: ## Tampilkan daftar target
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

tools: ## Pasang tool pengembangan (golangci-lint, mockery, govulncheck)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
	go install github.com/vektra/mockery/v3@v3.9.0
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/bufbuild/buf/cmd/buf@v1.75.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

proto: ## Generate kode Go dari api/proto (butuh buf, protoc-gen-go, protoc-gen-go-grpc)
	buf lint
	buf generate

build: ## Build semua binary ke folder bin/
	go build -o bin/ ./cmd/...

run-web: ## Jalankan API secara lokal (butuh MySQL, lihat config.json)
	go run ./cmd/web

run-worker: ## Jalankan worker secara lokal
	go run ./cmd/worker

test: ## Jalankan unit test
	go test ./internal/...

test-integration: ## Jalankan integration test (butuh Docker)
	go test -tags integration -count=1 ./test/...

lint: ## Jalankan linter
	golangci-lint run ./...

vuln: ## Periksa kerentanan dependency yang benar-benar terpanggil
	govulncheck ./...

mocks: ## Generate ulang mock dari interface di internal/usecase/ports.go
	mockery

create-admin: ## Jadikan user sebagai admin, contoh: make create-admin EMAIL=admin@example.com
	go run ./cmd/cli create-admin --email $(EMAIL)

MIGRATE_DB ?= mysql://gca:gca@tcp(mysql:3306)/golang_clean_architecture?multiStatements=true

migrate-up: ## Terapkan semua migrasi (lewat docker compose)
	docker compose run --rm migrate

migrate-down: ## Batalkan satu migrasi terakhir
	docker compose run --rm migrate -path /migrations -database "$(MIGRATE_DB)" down 1

migrate-create: ## Buat file migrasi baru, contoh: make migrate-create NAME=create_table_orders
	docker run --rm -v "$(CURDIR)/db/migrations:/migrations" migrate/migrate:v4.18.1 create -ext sql -dir /migrations -seq $(NAME)

compose-up: ## Jalankan mode minimal (MySQL + web + worker)
	docker compose up -d --build

compose-full: ## Jalankan mode lengkap (Kafka, Redis, S3, Mailpit, Jaeger, Prometheus)
	APP_ENV_FILE=deploy/full.env docker compose --profile full up -d --build

compose-down: ## Hentikan semua container (data volume tetap ada)
	docker compose --profile full down
