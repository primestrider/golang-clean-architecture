//go:build integration

package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/gateway/cache"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisCache_SetGetDelete(t *testing.T) {
	addr := testsupport.StartRedis(t)
	c := cache.NewRedisCache(redis.NewClient(&redis.Options{Addr: addr}))
	ctx := context.Background()

	var got model.ContactResponse
	hit, err := c.Get(ctx, "k", &got)
	require.NoError(t, err)
	assert.False(t, hit)

	require.NoError(t, c.Set(ctx, "k", model.ContactResponse{ID: "c1", FirstName: "Budi"}, time.Minute))
	hit, err = c.Get(ctx, "k", &got)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, "Budi", got.FirstName)

	require.NoError(t, c.Delete(ctx, "k"))
	hit, err = c.Get(ctx, "k", &got)
	require.NoError(t, err)
	assert.False(t, hit)
}

// TestContactCache_UpdateIsVisibleImmediately: dengan Redis aktif, perubahan
// kontak langsung terlihat (cache dihapus setelah update).
func TestContactCache_UpdateIsVisibleImmediately(t *testing.T) {
	addr := testsupport.StartRedis(t)
	app := testsupport.NewWebApp(t, func(cfg *config.Config) {
		cfg.Redis.Enabled = true
		cfg.Redis.Addr = addr
	})
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	token := app.TokenFor(t, user)
	contact := app.CreateContact(t, user.ID, "Budi")
	path := "/api/v1/contacts/" + contact.ID

	_, raw := app.Do(t, http.MethodGet, path, nil, token) // mengisi cache
	assert.Equal(t, "Budi", testsupport.DecodeData[model.ContactResponse](t, raw).FirstName)

	resp, _ := app.Do(t, http.MethodPut, path, model.UpdateContactRequest{FirstName: "Budi Baru", Version: 1}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	_, raw = app.Do(t, http.MethodGet, path, nil, token)
	assert.Equal(t, "Budi Baru", testsupport.DecodeData[model.ContactResponse](t, raw).FirstName)
}
