# Makefile berisi perintah yang sering dipakai selama pengembangan.
# Di Windows, jalankan lewat Git Bash atau WSL. Perintah aslinya juga
# tercantum di README untuk pengguna PowerShell.

.PHONY: help tools build test test-integration lint vuln mocks create-admin

help: ## Tampilkan daftar target
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

tools: ## Pasang tool pengembangan (golangci-lint, mockery, govulncheck)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
	go install github.com/vektra/mockery/v3@v3.9.0
	go install golang.org/x/vuln/cmd/govulncheck@latest

build: ## Build semua binary ke folder bin/
	go build -o bin/ ./cmd/...

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
