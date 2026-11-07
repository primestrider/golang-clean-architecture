//go:build integration

package test

import (
	"context"
	"net"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/config"
	contactv1 "github.com/primestrider/golang-clean-architecture/internal/delivery/grpc/pb/contact/v1"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// grpcClient menjalankan server gRPC aplikasi di atas koneksi in-memory
// (bufconn) dan mengembalikan koneksi client-nya.
func grpcClient(t *testing.T, app *testsupport.TestApp) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := config.NewGRPCServer(app.Cfg, discard(), app.Components)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func withToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func TestGRPC_Unauthenticated(t *testing.T) {
	app := testsupport.NewWebApp(t)
	client := contactv1.NewContactServiceClient(grpcClient(t, app))

	_, err := client.ListContacts(context.Background(), &contactv1.ListContactsRequest{Page: 1, Size: 10})

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGRPC_CreateAndGetContact(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	ctx := withToken(app.TokenFor(t, user))
	client := contactv1.NewContactServiceClient(grpcClient(t, app))

	created, err := client.CreateContact(ctx, &contactv1.CreateContactRequest{FirstName: "Budi", Email: "budi@example.com"})
	require.NoError(t, err)
	got, err := client.GetContact(ctx, &contactv1.GetContactRequest{Id: created.GetContact().GetId()})
	require.NoError(t, err)

	assert.Equal(t, "Budi", got.GetContact().GetFirstName())
	assert.EqualValues(t, 1, got.GetContact().GetVersion())
}

func TestGRPC_ValidationReturnsInvalidArgumentWithFieldViolations(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	client := contactv1.NewContactServiceClient(grpcClient(t, app))

	_, err := client.CreateContact(withToken(app.TokenFor(t, user)), &contactv1.CreateContactRequest{Email: "bukan-email"})

	st := status.Convert(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
	var fields []string
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				fields = append(fields, v.GetField())
			}
		}
	}
	assert.ElementsMatch(t, []string{"first_name", "email"}, fields)
}

func TestGRPC_NotFoundAndConflictCodes(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	ctx := withToken(app.TokenFor(t, user))
	client := contactv1.NewContactServiceClient(grpcClient(t, app))
	contact := app.CreateContact(t, user.ID, "Budi")

	_, err := client.GetContact(ctx, &contactv1.GetContactRequest{Id: "6f1c1f0e-4b7a-4c64-9d7e-6a0b7a1d2c3e"})
	assert.Equal(t, codes.NotFound, status.Code(err))

	_, err = client.UpdateContact(ctx, &contactv1.UpdateContactRequest{Id: contact.ID, FirstName: "X", Version: 9})
	assert.Equal(t, codes.Aborted, status.Code(err), "stale version maps to Aborted")
}

func TestGRPC_HealthNoAuth(t *testing.T) {
	app := testsupport.NewWebApp(t)
	health := healthpb.NewHealthClient(grpcClient(t, app))

	res, err := health.Check(context.Background(), &healthpb.HealthCheckRequest{})

	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, res.GetStatus())
}
