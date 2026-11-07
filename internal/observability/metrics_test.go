package observability

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetrics_UsesRouteTemplate(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())
	app := fiber.New()
	app.Use(m.HTTPMiddleware())
	app.Get("/api/v1/contacts/:contactId", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	for range 2 {
		resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/contacts/6f1c1f0e-4b7a-4c64-9d7e-6a0b7a1d2c3e", nil))
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	assert.InDelta(t, 2, testutil.ToFloat64(m.HTTPRequests.WithLabelValues("GET", "/api/v1/contacts/:contactId", "200")), 0)
	assert.Equal(t, 1, testutil.CollectAndCount(m.HTTPRequests), "raw ids must not create new series")
}

func TestHTTPMetrics_UnmatchedRouteHasFixedLabel(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())
	app := fiber.New()
	app.Use(m.HTTPMiddleware())

	resp, err := app.Test(httptest.NewRequest("GET", "/random/path/123", nil))
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.InDelta(t, 1, testutil.ToFloat64(m.HTTPRequests.WithLabelValues("GET", "unmatched", "404")), 0)
}
