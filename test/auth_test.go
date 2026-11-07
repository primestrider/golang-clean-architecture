//go:build integration

package test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func login(t *testing.T, app *testsupport.TestApp, email, password string) model.TokenResponse {
	t.Helper()
	resp, raw := app.Do(t, http.MethodPost, "/api/v1/auth/login", model.LoginRequest{Email: email, Password: password}, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	return testsupport.DecodeData[model.TokenResponse](t, raw)
}

func refresh(t *testing.T, app *testsupport.TestApp, token string) (int, model.TokenResponse) {
	t.Helper()
	resp, raw := app.Do(t, http.MethodPost, "/api/v1/auth/refresh", model.RefreshRequest{RefreshToken: token}, "")
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, model.TokenResponse{}
	}
	return resp.StatusCode, testsupport.DecodeData[model.TokenResponse](t, raw)
}

func TestLogin_ReturnsWorkingAccessToken(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")

	tokens := login(t, app, "EKO@example.com", "rahasia123")

	assert.Equal(t, "Bearer", tokens.TokenType)
	assert.Equal(t, 900, tokens.ExpiresIn)
	resp, _ := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, tokens.AccessToken)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLogin_WrongPasswordIsUnauthorized(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/auth/login", model.LoginRequest{Email: "eko@example.com", Password: "salahsalah"}, "")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "invalid email or password", testsupport.DecodeProblem(t, raw).Detail)
}

func TestRefresh_RotatesAndOldTokenStopsWorking(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")
	first := login(t, app, "eko@example.com", "rahasia123")

	status, second := refresh(t, app, first.RefreshToken)
	require.Equal(t, http.StatusOK, status)
	assert.NotEqual(t, first.RefreshToken, second.RefreshToken)

	// Memakai token lama lagi = reuse: ditolak, dan seluruh keluarga dicabut
	// sehingga token terbaru pun ikut tidak berlaku.
	status, _ = refresh(t, app, first.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, status)
	status, _ = refresh(t, app, second.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, status)
}

// TestRefresh_ConcurrentSameToken memastikan dua refresh bersamaan dengan
// token yang sama tidak pernah menghasilkan dua pasangan token yang valid.
func TestRefresh_ConcurrentSameToken(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")
	tokens := login(t, app, "eko@example.com", "rahasia123")

	type result struct {
		status int
		pair   model.TokenResponse
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() {
			status, pair := refresh(t, app, tokens.RefreshToken)
			results[i] = result{status, pair}
		})
	}
	wg.Wait()

	var winners []model.TokenResponse
	for _, r := range results {
		if r.status == http.StatusOK {
			winners = append(winners, r.pair)
		} else {
			assert.Equal(t, http.StatusUnauthorized, r.status)
		}
	}
	require.Len(t, winners, 1, "exactly one concurrent refresh may succeed")

	status, _ := refresh(t, app, winners[0].RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, status, "the family must be revoked after reuse")
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")
	tokens := login(t, app, "eko@example.com", "rahasia123")

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/auth/logout", model.LogoutRequest{RefreshToken: tokens.RefreshToken}, "")
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	status, _ := refresh(t, app, tokens.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestPasswordChange_RevokesRefreshTokens(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")
	tokens := login(t, app, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodPatch, "/api/v1/users/_current",
		model.UpdateUserRequest{CurrentPassword: "rahasia123", NewPassword: "passwordbaru"}, tokens.AccessToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))

	status, _ := refresh(t, app, tokens.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, status)
}
