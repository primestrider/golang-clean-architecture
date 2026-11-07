package config

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDatabase membuka koneksi GORM ke MySQL dan mengatur connection pool.
//
// Log query diarahkan ke slog. Query lambat (> 200 ms) dan error selalu
// dicatat; query biasa hanya dicatat saat log level debug supaya log
// production tidak banjir.
func NewDatabase(cfg *Config, log *slog.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(DSN(cfg.Database)), &gorm.Config{
		Logger: newGormLogger(cfg, log),
		// TranslateError mengubah error khusus MySQL (misalnya duplicate key
		// 1062) menjadi error GORM generik seperti gorm.ErrDuplicatedKey,
		// sehingga repository tidak bergantung pada kode error MySQL.
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.Database.Pool.Idle)
	sqlDB.SetMaxOpenConns(cfg.Database.Pool.Max)
	sqlDB.SetConnMaxLifetime(cfg.Database.Pool.Lifetime)

	return db, nil
}

// DSN menyusun data source name MySQL. parseTime=True diperlukan agar kolom
// waktu terbaca sebagai time.Time, dan loc=UTC menghindari kebingungan zona
// waktu antara server aplikasi dan database.
func DSN(c DatabaseConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		c.Username, c.Password, c.Host, c.Port, c.Name)
}

// newGormLogger membuat adapter logger GORM berbasis slog.
func newGormLogger(cfg *Config, log *slog.Logger) logger.Interface {
	level := logger.Warn
	if parseLevel(cfg.Log.Level) <= slog.LevelDebug {
		level = logger.Info
	}
	return logger.NewSlogLogger(log.With("component", "gorm"), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		LogLevel:                  level,
	})
}
