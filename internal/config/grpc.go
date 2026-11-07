package config

import (
	"log/slog"

	deliverygrpc "github.com/primestrider/golang-clean-architecture/internal/delivery/grpc"
	"google.golang.org/grpc"
)

// NewGRPCServer membuat server gRPC di atas use case yang sama dengan API
// HTTP. Reflection hanya aktif di luar production.
func NewGRPCServer(cfg *Config, log *slog.Logger, comps *WebComponents) *grpc.Server {
	return deliverygrpc.NewServer(deliverygrpc.ServerOptions{
		Log:        log,
		Tokens:     comps.Tokens,
		Contacts:   comps.Contacts,
		Metrics:    comps.Metrics,
		Reflection: !cfg.IsProduction(),
	})
}
