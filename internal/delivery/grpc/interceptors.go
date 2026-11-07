package grpc

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TokenParser memverifikasi access token. Diimplementasikan oleh
// security.TokenIssuer.
type TokenParser interface {
	Parse(token string) (*security.Claims, error)
}

// authKey adalah kunci context untuk identitas user yang login.
type authKey struct{}

// publicMethods adalah awalan method yang boleh diakses tanpa token.
var publicMethods = []string{"/grpc.health.v1.", "/grpc.reflection."}

// validRequestID sama dengan aturan di delivery HTTP.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// authInterceptor mewajibkan metadata "authorization: Bearer <token>" untuk
// semua method kecuali health check dan reflection.
func authInterceptor(tokens TokenParser) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		for _, prefix := range publicMethods {
			if strings.HasPrefix(info.FullMethod, prefix) {
				return handler(ctx, req)
			}
		}
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		scheme, token, ok := strings.Cut(values[0], " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		claims, err := tokens.Parse(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		ctx = context.WithValue(ctx, authKey{}, &model.Auth{UserID: claims.UserID, Role: claims.Role})
		ctx = observability.WithUserID(ctx, claims.UserID)
		return handler(ctx, req)
	}
}

// currentAuth mengambil identitas user dari context (diisi authInterceptor).
func currentAuth(ctx context.Context) *model.Auth {
	auth, _ := ctx.Value(authKey{}).(*model.Auth)
	return auth
}

// requestIDInterceptor memakai metadata "x-request-id" bila valid, atau
// membuat id baru, lalu menyimpannya di context untuk korelasi log.
func requestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get("x-request-id"); len(values) > 0 && validRequestID.MatchString(values[0]) {
				id = values[0]
			}
		}
		if id == "" {
			id = uuid.NewString()
		}
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", id))
		return handler(observability.WithRequestID(ctx, id), req)
	}
}

// recoverPanic mengubah panic menjadi status Internal tanpa membocorkan
// detailnya ke client; detail dicatat di log.
func recoverPanic(log *slog.Logger) func(ctx context.Context, p any) error {
	return func(ctx context.Context, p any) error {
		log.ErrorContext(ctx, "grpc panic recovered", "panic", p)
		return status.Error(codes.Internal, "internal server error")
	}
}

// slogAdapter menghubungkan interceptor logging go-grpc-middleware ke slog.
func slogAdapter(log *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, level logging.Level, msg string, fields ...any) {
		log.Log(ctx, slog.Level(level), msg, fields...)
	})
}
