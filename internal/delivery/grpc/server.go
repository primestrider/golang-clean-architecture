// Package grpc adalah delivery layer untuk protokol gRPC.
//
// Perannya sama dengan delivery HTTP: menerjemahkan request gRPC menjadi
// model request, memanggil use case yang SAMA dengan yang dipakai REST API,
// lalu menerjemahkan hasil dan error ke format gRPC. Ini menunjukkan bahwa
// logika bisnis tidak terikat pada satu protokol.
package grpc

import (
	"log/slog"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	contactv1 "github.com/primestrider/golang-clean-architecture/internal/delivery/grpc/pb/contact/v1"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// ServerOptions berisi dependency server gRPC.
type ServerOptions struct {
	Log      *slog.Logger
	Tokens   TokenParser
	Contacts *usecase.ContactUseCase
	Metrics  *observability.Metrics
	// Reflection mengaktifkan gRPC reflection (dipakai tool seperti grpcurl).
	// Sebaiknya hanya aktif di development.
	Reflection bool
}

// NewServer membuat server gRPC dengan rantai interceptor:
// recovery → request id → logging → metrics → auth, plus tracing lewat
// stats handler OpenTelemetry. Health service standar (grpc.health.v1)
// didaftarkan dan bisa diakses tanpa token.
func NewServer(opts ServerOptions) *grpc.Server {
	serverMetrics := grpcprom.NewServerMetrics()
	opts.Metrics.Register(serverMetrics)

	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(recoverPanic(opts.Log))),
			requestIDInterceptor(),
			logging.UnaryServerInterceptor(slogAdapter(opts.Log), logging.WithLogOnEvents(logging.FinishCall)),
			serverMetrics.UnaryServerInterceptor(),
			authInterceptor(opts.Tokens),
		),
	)

	contactv1.RegisterContactServiceServer(server, &ContactService{contacts: opts.Contacts})

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)

	if opts.Reflection {
		reflection.Register(server)
	}
	serverMetrics.InitializeMetrics(server)
	return server
}
