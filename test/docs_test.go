//go:build integration

package test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/primestrider/golang-clean-architecture/api"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDocs_ServesSpecAndUI(t *testing.T) {
	app := testsupport.NewWebApp(t, func(cfg *config.Config) { cfg.Docs.Enabled = true })

	resp, spec := rawRequest(t, app, http.MethodGet, "/openapi.yaml", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, spec, "openapi: 3.1")

	resp, page := rawRequest(t, app, http.MethodGet, "/docs", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, page, "swagger-ui")
	assert.Equal(t, 2, strings.Count(page, `integrity="sha384-`), "CDN assets must carry Subresource Integrity hashes")
	assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "cdn.jsdelivr.net")
}

func TestDocs_NotRegisteredWhenDisabled(t *testing.T) {
	app := testsupport.NewWebApp(t, func(cfg *config.Config) { cfg.Docs.Enabled = false })

	resp, _ := rawRequest(t, app, http.MethodGet, "/docs", "", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestOpenAPI_CoversAllRoutes memastikan setiap route /api/v1 yang
// terdaftar di aplikasi terdokumentasi di api/openapi.yaml, supaya
// dokumentasi tidak tertinggal dari kode.
func TestOpenAPI_CoversAllRoutes(t *testing.T) {
	app := testsupport.NewWebApp(t, func(cfg *config.Config) { cfg.Docs.Enabled = true })
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(api.OpenAPI, &spec))

	documented := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			documented[strings.ToUpper(method)+" /api/v1"+path] = true
		}
	}

	param := regexp.MustCompile(`:([A-Za-z]+)`)
	for _, r := range app.App.GetRoutes(true) {
		if !strings.HasPrefix(r.Path, "/api/v1/") || r.Method == http.MethodHead {
			continue
		}
		key := r.Method + " " + param.ReplaceAllString(r.Path, "{$1}")
		assert.True(t, documented[key], "route %s is not documented in api/openapi.yaml", key)
	}
}
