package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/security"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
)

// errInvalidCredentials sengaja satu pesan untuk email tidak terdaftar
// maupun password salah, supaya penyerang tidak bisa memastikan email mana
// yang terdaftar.
const errInvalidCredentials = "invalid email or password"

// AuthDeps adalah dependency AuthUseCase. Memakai struct karena jumlah
// dependency-nya banyak; field bernama lebih jelas daripada parameter
// berurutan.
type AuthDeps struct {
	Log        *slog.Logger
	Validate   *validation.Validator
	Tx         Transactor
	Users      UserRepository
	Tokens     RefreshTokenRepository
	Issuer     TokenIssuer
	Audit      *AuditRecorder
	RefreshTTL time.Duration
	// Now mengembalikan waktu sekarang; disuntikkan supaya kedaluwarsa bisa
	// diuji. Nil berarti time.Now.
	Now func() time.Time
	// CompareDummy menyamakan waktu respons login untuk email yang tidak
	// terdaftar. Nil berarti security.CompareDummy.
	CompareDummy func(password string)
}

// AuthUseCase mengatur sesi login: login, rotasi refresh token, dan logout.
//
// Modelnya dua token:
//   - access token (JWT) berumur pendek, dikirim di setiap request dan
//     diverifikasi tanpa ke database;
//   - refresh token (string acak) berumur panjang, hanya untuk meminta
//     pasangan token baru, dan disimpan di database dalam bentuk hash.
//
// Setiap refresh token hanya boleh dipakai sekali (rotasi). Bila token yang
// sudah dipakai muncul lagi, berarti token itu dicuri atau bocor, sehingga
// seluruh keluarga token hasil login tersebut dicabut.
type AuthUseCase struct {
	deps AuthDeps
}

// NewAuthUseCase membuat AuthUseCase.
func NewAuthUseCase(deps AuthDeps) *AuthUseCase {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.CompareDummy == nil {
		deps.CompareDummy = security.CompareDummy
	}
	return &AuthUseCase{deps: deps}
}

// Login memeriksa email dan password, lalu menerbitkan access token dan
// refresh token baru (keluarga token baru).
func (c *AuthUseCase) Login(ctx context.Context, req *model.LoginRequest) (*model.TokenResponse, error) {
	req.Email = NormalizeEmail(req.Email)
	if err := c.deps.Validate.Struct(req); err != nil {
		return nil, err
	}

	var res *model.TokenResponse
	err := c.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := c.deps.Users.FindByEmail(ctx, req.Email)
		if err != nil {
			if isNotFound(err) {
				c.deps.CompareDummy(req.Password)
				return apperror.Unauthorized(errInvalidCredentials)
			}
			return err
		}
		if !security.ComparePassword(user.Password, req.Password) {
			return apperror.Unauthorized(errInvalidCredentials)
		}
		res, _, err = c.issuePair(ctx, user, uuid.NewString(), req.UserAgent, req.IP)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Refresh menukar refresh token dengan pasangan token baru.
//
// Baris token dikunci (SELECT ... FOR UPDATE) selama transaksi, sehingga
// dua request yang memakai token yang sama diproses bergiliran: yang pertama
// berhasil, yang kedua melihat token sudah dicabut dan memicu deteksi reuse.
func (c *AuthUseCase) Refresh(ctx context.Context, req *model.RefreshRequest) (*model.TokenResponse, error) {
	if err := c.deps.Validate.Struct(req); err != nil {
		return nil, err
	}

	var res *model.TokenResponse
	reused := false
	err := c.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		old, err := c.deps.Tokens.FindByHashForUpdate(ctx, security.HashRefreshToken(req.RefreshToken))
		if err != nil {
			if isNotFound(err) {
				return apperror.Unauthorized("invalid refresh token")
			}
			return err
		}

		if old.RevokedAt != nil {
			// Pencabutan keluarga harus tetap tersimpan walau request ini
			// ditolak. Karena itu fungsi ini mengembalikan nil (transaksi
			// di-commit), dan penolakan dikembalikan setelah transaksi selesai.
			reused = true
			c.deps.Log.WarnContext(ctx, "refresh token reuse detected, revoking token family",
				"user_id", old.UserID, "family_id", old.FamilyID)
			if err := c.deps.Tokens.RevokeFamily(ctx, old.FamilyID); err != nil {
				return err
			}
			return c.deps.Audit.Record(ctx, "auth.refresh_reuse_detected", "user", old.UserID, nil, nil)
		}
		if old.ExpiresAt <= c.deps.Now().UnixMilli() {
			return apperror.Unauthorized("refresh token expired")
		}

		user, err := c.deps.Users.FindByID(ctx, old.UserID)
		if err != nil {
			if isNotFound(err) {
				return apperror.Unauthorized("invalid refresh token")
			}
			return err
		}

		var newID string
		res, newID, err = c.issuePair(ctx, user, old.FamilyID, req.UserAgent, req.IP)
		if err != nil {
			return err
		}
		return c.deps.Tokens.Revoke(ctx, old.ID, &newID)
	})
	if err != nil {
		return nil, err
	}
	if reused {
		return nil, apperror.Unauthorized("invalid refresh token")
	}
	return res, nil
}

// Logout mencabut seluruh keluarga refresh token milik sesi ini. Token yang
// tidak dikenal diabaikan, sehingga logout selalu berhasil (idempoten).
//
// Access token yang sudah terbit tetap berlaku sampai kedaluwarsa (paling
// lama sesuai jwt.access_ttl). Ini trade-off yang disengaja: verifikasi JWT
// tanpa database jauh lebih murah, dan umur access token dibuat pendek.
func (c *AuthUseCase) Logout(ctx context.Context, req *model.LogoutRequest) error {
	if err := c.deps.Validate.Struct(req); err != nil {
		return err
	}
	return c.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		token, err := c.deps.Tokens.FindByHashForUpdate(ctx, security.HashRefreshToken(req.RefreshToken))
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		return c.deps.Tokens.RevokeFamily(ctx, token.FamilyID)
	})
}

// issuePair menerbitkan access token dan menyimpan refresh token baru dalam
// keluarga familyID. Mengembalikan respons dan id refresh token baru.
func (c *AuthUseCase) issuePair(ctx context.Context, user *entity.User, familyID, userAgent, ip string) (*model.TokenResponse, string, error) {
	access, expiresAt, err := c.deps.Issuer.Issue(user.ID, user.Role)
	if err != nil {
		return nil, "", err
	}
	plain, hash, err := security.NewRefreshToken()
	if err != nil {
		return nil, "", apperror.Internal(err)
	}

	now := c.deps.Now()
	token := &entity.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		FamilyID:  familyID,
		TokenHash: hash,
		ExpiresAt: now.Add(c.deps.RefreshTTL).UnixMilli(),
		UserAgent: truncate(userAgent, 255),
		IP:        truncate(ip, 45),
	}
	if err := c.deps.Tokens.Create(ctx, token); err != nil {
		return nil, "", err
	}

	return &model.TokenResponse{
		AccessToken:  access,
		RefreshToken: plain,
		TokenType:    "Bearer",
		ExpiresIn:    int(expiresAt.Sub(now).Seconds()),
	}, token.ID, nil
}

// truncate memotong s agar muat di kolom database berukuran max.
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// isNotFound melaporkan apakah err adalah apperror NOT_FOUND.
func isNotFound(err error) bool {
	appErr, ok := apperror.As(err)
	return ok && appErr.Code == apperror.CodeNotFound
}
