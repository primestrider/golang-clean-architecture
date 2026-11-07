// Package testsupport menyediakan perangkat integration test: container
// dependency (MySQL dan lainnya) lewat testcontainers, aplikasi web siap
// pakai, helper request HTTP, dan fixture data.
//
// Integration test membutuhkan Docker. Test di folder test/ memakai build
// tag "integration", sehingga `go test ./...` biasa tidak menjalankannya.
// Jalankan dengan `make test-integration` atau
// `go test -tags integration ./test/...`.
package testsupport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	// Mendaftarkan driver "mysql" untuk golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

// mysqlImage adalah versi MySQL untuk test. Samakan dengan docker-compose.
const mysqlImage = "mysql:8.4"

// RepoRoot mengembalikan path root repository, dihitung dari lokasi file
// ini. Dipakai untuk menemukan config.json dan folder migrasi dari mana pun
// test dijalankan.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// StartMySQL menjalankan container MySQL, menerapkan semua migrasi, lalu
// mengembalikan konfigurasi database untuk terhubung ke container itu.
// Container dimatikan oleh fungsi cleanup yang dikembalikan.
//
// Fungsi ini menerima context alih-alih *testing.T supaya bisa dipanggil
// sekali dari TestMain dan dipakai bersama oleh semua test di satu package.
// Menyalakan satu container per test terlalu lambat.
func StartMySQL(ctx context.Context) (config.DatabaseConfig, func(), error) {
	container, err := mysql.Run(ctx, mysqlImage,
		mysql.WithDatabase("gca_test"),
		mysql.WithUsername("gca"),
		mysql.WithPassword("gca"),
	)
	if err != nil {
		return config.DatabaseConfig{}, nil, fmt.Errorf("start mysql container: %w", err)
	}
	cleanup := func() { _ = container.Terminate(context.Background()) }

	host, err := container.Host(ctx)
	if err != nil {
		cleanup()
		return config.DatabaseConfig{}, nil, err
	}
	port, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		cleanup()
		return config.DatabaseConfig{}, nil, err
	}

	portNum, _ := strconv.Atoi(port.Port())
	dbCfg := config.DatabaseConfig{
		Username: "gca", Password: "gca", Host: host, Port: portNum, Name: "gca_test",
		Pool: config.PoolConfig{Idle: 5, Max: 20},
	}
	if err := MigrateUp(dbCfg); err != nil {
		cleanup()
		return config.DatabaseConfig{}, nil, err
	}
	return dbCfg, cleanup, nil
}

// MigrateUp menerapkan semua migrasi di db/migrations memakai golang-migrate
// sebagai library, persis seperti yang dilakukan CLI `migrate up`.
func MigrateUp(dbCfg config.DatabaseConfig) error {
	m, err := newMigrate(dbCfg)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDown membatalkan semua migrasi. Dipakai untuk menguji bahwa file
// .down.sql benar.
func MigrateDown(dbCfg config.DatabaseConfig) error {
	m, err := newMigrate(dbCfg)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// newMigrate membuat instance golang-migrate untuk database dbCfg.
// multiStatements=true diperlukan agar satu file migrasi boleh berisi lebih
// dari satu statement SQL.
func newMigrate(dbCfg config.DatabaseConfig) (*migrate.Migrate, error) {
	source, err := iofs.New(os.DirFS(filepath.Join(RepoRoot(), "db", "migrations")), ".")
	if err != nil {
		return nil, fmt.Errorf("open migrations: %w", err)
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?multiStatements=true",
		dbCfg.Username, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.Name)
	m, err := migrate.NewWithSourceInstance("iofs", source, "mysql://"+dsn)
	if err != nil {
		return nil, fmt.Errorf("init migrate: %w", err)
	}
	return m, nil
}

// RunMain adalah isi TestMain standar untuk package integration test:
// menyalakan MySQL sekali, menjalankan semua test, lalu mematikan container.
// Konfigurasi database disimpan di variabel paket supaya NewWebApp bisa
// memakainya.
func RunMain(m *testing.M) int {
	ctx := context.Background()
	dbCfg, cleanup, err := StartMySQL(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration test setup failed:", err)
		return 1
	}
	defer cleanup()
	sharedDB = dbCfg
	return m.Run()
}

// sharedDB adalah konfigurasi database milik container yang dinyalakan
// RunMain untuk package test yang sedang berjalan.
var sharedDB config.DatabaseConfig

// SharedDatabase mengembalikan konfigurasi database container bersama.
func SharedDatabase() config.DatabaseConfig { return sharedDB }
