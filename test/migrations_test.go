//go:build integration

package test

import (
	"testing"

	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrations_UpDown memastikan semua migrasi bisa dibatalkan (down) dan
// diterapkan ulang (up) tanpa error, dan menghasilkan skema yang diharapkan.
func TestMigrations_UpDown(t *testing.T) {
	app := testsupport.NewWebApp(t)
	dbCfg := testsupport.SharedDatabase()

	require.NoError(t, testsupport.MigrateDown(dbCfg))
	t.Cleanup(func() { require.NoError(t, testsupport.MigrateUp(dbCfg)) })
	assert.Empty(t, tables(t, app), "down must drop every table")

	require.NoError(t, testsupport.MigrateUp(dbCfg))
	assert.ElementsMatch(t, []string{
		"users", "contacts", "addresses", "refresh_tokens",
		"audit_logs", "outbox_events", "processed_events",
	}, tables(t, app))

	var columns []string
	require.NoError(t, app.DB.Raw(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'users'`).Scan(&columns).Error)
	assert.Subset(t, columns, []string{"id", "email", "role", "avatar_key", "version", "deleted_at"})
	assert.NotContains(t, columns, "token")
}

func tables(t *testing.T, app *testsupport.TestApp) []string {
	t.Helper()
	var names []string
	require.NoError(t, app.DB.Raw(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name <> 'schema_migrations'`).Scan(&names).Error)
	return names
}
