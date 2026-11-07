package testsupport

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestApp adalah aplikasi web lengkap yang terhubung ke database container.
// Request dikirim langsung ke Fiber (app.Test) tanpa membuka port jaringan.
type TestApp struct {
	App *fiber.App
	DB  *gorm.DB
	Cfg *config.Config
}

// Option mengubah konfigurasi sebelum aplikasi dibuat, misalnya untuk
// mengaktifkan fitur opsional di test tertentu.
type Option func(cfg *config.Config)

// NewWebApp membuat aplikasi web baru di atas database bersama dan
// mengosongkan semua tabel lebih dulu, sehingga setiap test mulai dari
// keadaan bersih dan tidak bergantung pada test lain.
func NewWebApp(t testing.TB, opts ...Option) *TestApp {
	t.Helper()
	cfg, err := config.Load(RepoRoot())
	require.NoError(t, err)
	cfg.Database = SharedDatabase()
	cfg.Log.Level = "error"
	for _, opt := range opts {
		opt(cfg)
	}

	log := slog.New(slog.DiscardHandler)
	db, err := config.NewDatabase(cfg, log)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	Reset(t, db)

	app := config.NewFiber(cfg, log)
	config.BootstrapWeb(&config.WebDeps{Config: cfg, Log: log, DB: db, App: app})
	return &TestApp{App: app, DB: db, Cfg: cfg}
}

// Reset mengosongkan semua tabel aplikasi (kecuali tabel catatan migrasi).
// Pemeriksaan foreign key dimatikan sementara di satu koneksi yang sama
// supaya urutan pengosongan tabel tidak menjadi masalah.
func Reset(t testing.TB, db *gorm.DB) {
	t.Helper()
	err := db.Connection(func(conn *gorm.DB) error {
		var tables []string
		if err := conn.Raw(`SELECT table_name FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_name <> 'schema_migrations'`).Scan(&tables).Error; err != nil {
			return err
		}
		if err := conn.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := conn.Exec("TRUNCATE TABLE `" + table + "`").Error; err != nil {
				return err
			}
		}
		return conn.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	})
	require.NoError(t, err)
}

// Do mengirim request ke aplikasi. body di-encode sebagai JSON bila tidak
// nil; token (bila tidak kosong) dikirim di header Authorization.
// Mengembalikan respons dan body mentahnya. Body respons sudah dibaca dan
// ditutup di sini, jadi pemanggil tidak perlu menutupnya lagi.
func (a *TestApp) Do(t testing.TB, method, path string, body any, token string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := a.App.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, raw
}

// DecodeData membaca field "data" dari amplop WebResponse ke tipe T.
func DecodeData[T any](t testing.TB, raw []byte) T {
	t.Helper()
	var envelope struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &envelope), string(raw))
	return envelope.Data
}

// Problem adalah bentuk body error Problem Details untuk assertion di test.
type Problem struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Errors []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"errors"`
}

// DecodeProblem membaca body Problem Details.
func DecodeProblem(t testing.TB, raw []byte) Problem {
	t.Helper()
	var p Problem
	require.NoError(t, json.Unmarshal(raw, &p), string(raw))
	return p
}
