package config

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DevJWTSecret adalah secret JWT bawaan untuk pengembangan lokal. Nilai ini
// sengaja ada di config.json supaya aplikasi bisa langsung jalan, dan
// sengaja ditolak oleh Validate ketika app.env = production.
const DevJWTSecret = "dev-only-insecure-secret-change-me-32bytes" //nolint:gosec // nilai dev yang disengaja, ditolak di production

// Config adalah seluruh konfigurasi aplikasi dalam bentuk struct bertipe.
//
// Nilai dibaca dari config.json (default non-rahasia), lalu setiap key bisa
// ditimpa environment variable berprefix APP_ dengan titik diganti garis
// bawah. Contoh: database.password ditimpa oleh APP_DATABASE_PASSWORD.
// Secret (password, JWT secret, access key) seharusnya hanya datang dari env.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Server    ServerConfig    `mapstructure:"server"`
	Admin     AdminConfig     `mapstructure:"admin"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	Log       LogConfig       `mapstructure:"log"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	Outbox    OutboxConfig    `mapstructure:"outbox"`
	Consumer  ConsumerConfig  `mapstructure:"consumer"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Email     EmailConfig     `mapstructure:"email"`
	OTel      OTelConfig      `mapstructure:"otel"`
	Docs      DocsConfig      `mapstructure:"docs"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Shutdown  ShutdownConfig  `mapstructure:"shutdown"`
}

// AppConfig berisi identitas aplikasi. Env bernilai "development" atau
// "production" dan menentukan aturan validasi yang lebih ketat.
type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

// ServerConfig mengatur server HTTP publik.
type ServerConfig struct {
	Port int `mapstructure:"port"`
	// TrustedProxies adalah daftar IP/CIDR load balancer yang header
	// X-Forwarded-For-nya boleh dipercaya. Kosong berarti header itu diabaikan,
	// sehingga client tidak bisa memalsukan IP untuk mengakali rate limit.
	TrustedProxies []string      `mapstructure:"trusted_proxies"`
	CORSOrigins    []string      `mapstructure:"cors_origins"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	// BodyLimit adalah ukuran body maksimum dalam byte.
	BodyLimit int `mapstructure:"body_limit"`
}

// AdminConfig mengatur server admin internal (health check dan metrics).
// Port-nya dipisah dari port publik supaya metrics tidak terekspos ke internet.
type AdminConfig struct {
	// Port dipakai binary web, WorkerPort dipakai binary worker, supaya
	// keduanya bisa berjalan di mesin yang sama.
	Port       int `mapstructure:"port"`
	WorkerPort int `mapstructure:"worker_port"`
}

// GRPCConfig mengatur server gRPC opsional.
type GRPCConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

// LogConfig mengatur logger. Level: debug, info, warn, error.
// Format: json (production) atau text (enak dibaca saat development).
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DatabaseConfig mengatur koneksi MySQL.
type DatabaseConfig struct {
	Username string     `mapstructure:"username"`
	Password string     `mapstructure:"password"`
	Host     string     `mapstructure:"host"`
	Port     int        `mapstructure:"port"`
	Name     string     `mapstructure:"name"`
	Pool     PoolConfig `mapstructure:"pool"`
}

// PoolConfig mengatur connection pool database.
type PoolConfig struct {
	Idle     int           `mapstructure:"idle"`
	Max      int           `mapstructure:"max"`
	Lifetime time.Duration `mapstructure:"lifetime"`
}

// JWTConfig mengatur access token JWT dan masa berlaku refresh token.
type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	Issuer     string        `mapstructure:"issuer"`
	Audience   string        `mapstructure:"audience"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// RateLimitConfig mengatur batas jumlah request per menit.
type RateLimitConfig struct {
	// GlobalPerMinute berlaku untuk semua endpoint, per IP.
	GlobalPerMinute int `mapstructure:"global_per_minute"`
	// AuthPerMinute membatasi login/registrasi per email, dan refresh per IP.
	AuthPerMinute int `mapstructure:"auth_per_minute"`
	// AuthIPPerMinute membatasi login/registrasi per IP, berapa pun email
	// yang dicoba (mencegah credential stuffing dari satu IP).
	AuthIPPerMinute int `mapstructure:"auth_ip_per_minute"`
}

// KafkaConfig mengatur koneksi Kafka. Jika Enabled false, event tetap
// ditulis ke outbox dan diproses langsung oleh handler lokal di worker.
type KafkaConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	Brokers     []string `mapstructure:"brokers"`
	GroupID     string   `mapstructure:"group_id"`
	OffsetReset string   `mapstructure:"offset_reset"`
}

// OutboxConfig mengatur relay outbox di worker.
type OutboxConfig struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
	BatchSize    int           `mapstructure:"batch_size"`
	MaxAttempts  int           `mapstructure:"max_attempts"`
}

// ConsumerConfig mengatur retry consumer sebelum pesan dikirim ke DLQ.
type ConsumerConfig struct {
	MaxRetries     int           `mapstructure:"max_retries"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
}

// RedisConfig mengatur Redis opsional (cache, rate limit, lock scheduler).
type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// StorageConfig mengatur object storage S3-compatible (MinIO, AWS S3, R2).
type StorageConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	// PublicEndpoint dipakai untuk membuat presigned URL bila client
	// mengakses storage lewat alamat yang berbeda dari aplikasi
	// (misalnya "minio:9000" di Docker vs "localhost:9000" di browser).
	PublicEndpoint string        `mapstructure:"public_endpoint"`
	AccessKey      string        `mapstructure:"access_key"`
	SecretKey      string        `mapstructure:"secret_key"`
	Bucket         string        `mapstructure:"bucket"`
	UseSSL         bool          `mapstructure:"use_ssl"`
	PresignTTL     time.Duration `mapstructure:"presign_ttl"`
}

// EmailConfig mengatur pengiriman email lewat SMTP.
type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	TLS      bool   `mapstructure:"tls"`
}

// OTelConfig mengatur tracing OpenTelemetry.
type OTelConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	Endpoint    string  `mapstructure:"endpoint"`
	ServiceName string  `mapstructure:"service_name"`
	SampleRatio float64 `mapstructure:"sample_ratio"`
}

// DocsConfig mengatur Swagger UI. Wajib mati di production.
type DocsConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// SchedulerConfig berisi jadwal cron untuk job pembersihan di worker.
type SchedulerConfig struct {
	TokenCleanupCron  string `mapstructure:"token_cleanup_cron"`
	OutboxCleanupCron string `mapstructure:"outbox_cleanup_cron"`
}

// ShutdownConfig mengatur batas waktu graceful shutdown.
type ShutdownConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

// Load membaca config.json dari direktori yang diberikan (atau, jika kosong,
// dari ".", "..", dan "../.." supaya bisa dipanggil dari root proyek maupun
// dari folder test), menerapkan override env APP_*, lalu memvalidasi hasilnya.
//
// File config bersifat opsional: di container, seluruh nilai boleh datang
// dari environment variable.
func Load(paths ...string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("json")
	if len(paths) == 0 {
		paths = []string{".", "..", "../.."}
	}
	for _, p := range paths {
		v.AddConfigPath(p)
	}

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// AutomaticEnv hanya bekerja untuk key yang sudah dikenal viper. Key yang
	// tidak ada di file perlu didaftarkan manual agar tetap bisa diisi lewat env.
	if err := bindEnvs(v, reflect.TypeFor[Config](), ""); err != nil {
		return nil, err
	}

	if err := v.ReadInConfig(); err != nil {
		if _, notFound := errors.AsType[viper.ConfigFileNotFoundError](err); !notFound {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	cfg := new(Config)
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// bindEnvs menelusuri tag mapstructure pada struct secara rekursif dan
// mendaftarkan setiap key ke viper.BindEnv.
func bindEnvs(v *viper.Viper, t reflect.Type, prefix string) error {
	for field := range t.Fields() {
		key := field.Tag.Get("mapstructure")
		if prefix != "" {
			key = prefix + "." + key
		}
		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeFor[time.Duration]() {
			if err := bindEnvs(v, field.Type, key); err != nil {
				return err
			}
			continue
		}
		if err := v.BindEnv(key); err != nil {
			return fmt.Errorf("bind env %s: %w", key, err)
		}
	}
	return nil
}

// IsProduction melaporkan apakah aplikasi berjalan di environment production.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// Validate memeriksa konfigurasi dan mengembalikan semua pelanggaran
// sekaligus. Aplikasi menolak start bila ada pelanggaran (fail fast), lebih
// baik daripada berjalan dengan konfigurasi yang tidak aman.
func (c *Config) Validate() error {
	var errs []error
	if c.JWT.Secret == "" {
		errs = append(errs, errors.New("jwt.secret is required"))
	}
	if c.IsProduction() {
		if len(c.JWT.Secret) < 32 || c.JWT.Secret == DevJWTSecret {
			errs = append(errs, errors.New("jwt.secret must be at least 32 bytes and not the development default in production"))
		}
		if slices.Contains(c.Server.CORSOrigins, "*") {
			errs = append(errs, errors.New("server.cors_origins must not contain \"*\" in production"))
		}
		if c.Docs.Enabled {
			errs = append(errs, errors.New("docs.enabled must be false in production"))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}
