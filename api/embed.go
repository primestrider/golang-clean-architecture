// Package api berisi kontrak API: spesifikasi OpenAPI (HTTP) dan file proto
// (gRPC). Spesifikasi OpenAPI di-embed ke binary supaya bisa disajikan oleh
// Swagger UI tanpa file tambahan saat deploy.
package api

import _ "embed"

// OpenAPI adalah isi api/openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPI []byte
