# Production-Ready Boilerplate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the Go Clean Architecture template into a production-ready, well-documented boilerplate covering production readiness, security, data/event reliability, testability, observability, and common backend features.

**Architecture:** Keep manual wiring in `internal/config` and the delivery → usecase → repository/gateway layering. Use cases depend only on interfaces declared in `internal/usecase/ports.go`, run transactions through a context-carried `Transactor`, and return typed `apperror.Error` values that each delivery (HTTP Fiber v3, gRPC, Kafka consumer) maps to its own protocol. Events go through a transactional outbox drained by a single leader relay in the worker. When Kafka is disabled, the relay dispatches straight to local handlers.

**Tech Stack:** Go 1.27.3, Fiber v3, GORM + MySQL 8, golang-migrate, log/slog, Viper, golang-jwt/jwt v5, bcrypt, IBM/sarama, redis/go-redis v9, minio-go v7, gocron v2, wneessen/go-mail, gRPC + buf + go-grpc-middleware v2, prometheus/client_golang, OpenTelemetry (OTLP), testify + mockery, testcontainers-go, golangci-lint v2, govulncheck, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-11-06-production-ready-boilerplate-design.md` (read it; section refs below are `§x.y`).

## Global Constraints

- Work on branch `feature/production-ready`, created from `upgrade-go-1.27.3`. Commit after every task. Run all phases without stopping; the owner reviews at the end.
- `go 1.27.3` in go.mod. Module path: `github.com/primestrider/golang-clean-architecture`.
- Every package has a `doc.go` that explains its layer role. Every exported identifier, and every non-trivial unexported one, has an Indonesian godoc comment (`// Nama ...`) that explains *what* and *why*. Identifiers stay in English. README is in Indonesian.
- No DI container. All wiring stays explicit in `internal/config`.
- Minimal mode (MySQL only, plus the worker for events) must work. Kafka, Redis, storage, email, OTel, gRPC and docs are each toggled by an `enabled` flag, with the fallback described in the spec.
- Never log or emit passwords, password hashes, refresh tokens or token hashes. 5xx responses never include internal detail.
- Timestamps are `BIGINT` epoch milliseconds (`autoCreateTime:milli`).
- Success envelope is `{"data":…, "paging":…}`. Error body is RFC 9457 `application/problem+json` (§5.1).
- All HTTP routes live under `/api/v1`.
- Pinned values: access TTL 15m; refresh TTL 7d; refresh token 32 random bytes, base64url, stored as SHA-256 hex; password 8–72 bytes; global rate limit 100/min/IP; auth limit 5/min (login/register per IP+email, refresh per IP); request timeout 10s; body limit 1 MB (avatar route 2 MB); shutdown timeout 15s; outbox poll 1s, batch 100, max_attempts 10; consumer retries 3 with exponential backoff; cache TTL 5m; presign TTL 15m; admin port 9090 (web) / 9091 (worker); gRPC port 50051.
- A task is done only when `go build ./...`, `go vet ./...`, `golangci-lint run` (from Task 9 onward), unit tests (`go test ./internal/...`) and the tests that task touches all pass.
- Integration tests use build tag `integration` and testcontainers (from Task 8 onward). Until Task 8, verify with build, vet and unit tests only. The local MySQL rejects passwordless root.

## Review Focus

1. **Concurrent refresh with the same token.** Two requests that race must yield exactly one new pair. The other must get 401 and revoke the family; two valid pairs must never come out. Lock the token row with `SELECT … FOR UPDATE`. Test: Task 13.
2. **Kafka down while the system runs.** API writes still succeed, outbox rows stay `pending`, nothing is lost, and publishing resumes when the broker returns. Test: Task 19.
3. **Spoofed `X-Forwarded-For` without a trusted proxy.** The header is ignored, so it cannot bypass the rate limit. Test: Task 14.
4. **Stale `version` on a soft-deleted record.** Returns 404, not 409. Test: Task 16.
5. **Malformed or oversized JSON, or the wrong Content-Type.** Returns 400/413/415 as problem+json, never 500. Test: Task 14.

---

## Target File Map

```
cmd/web/main.go            HTTP + gRPC + admin server, graceful shutdown
cmd/worker/main.go         consumers, outbox relay, scheduler, admin server
cmd/cli/main.go            create-admin command
internal/apperror/         typed domain errors
internal/config/           config.go (typed Config, Load), logger.go, database.go, fiber.go, kafka.go,
                           redis.go, storage.go, otel.go, web.go (BootstrapWeb), worker.go (BootstrapWorker)
internal/validation/       validator wrapper -> apperror field errors
internal/observability/    slog context handler, metrics registry, tracing helpers
internal/security/         password.go, jwt.go, refreshtoken.go
internal/entity/           user, contact, address, refresh_token, audit_log, outbox_event, processed_event
internal/model/            request/response DTOs, event envelope, converter/
internal/usecase/          ports.go, user, auth, contact, address, admin, notification use cases (+ _test.go)
internal/repository/       transactor.go, repository.go (generic), one file per aggregate
internal/outbox/           relay.go, dispatcher_kafka.go, dispatcher_local.go
internal/delivery/http/    controllers, problem.go, route/, middleware/, docs/
internal/delivery/messaging/ registry.go, consumer.go (retry/DLQ/idempotency), handlers
internal/delivery/grpc/    server.go, interceptors.go, contact_service.go, pb/ (generated)
internal/delivery/admin/   server.go (health + metrics)
internal/gateway/cache/    redis_cache.go
internal/gateway/storage/  minio_storage.go
internal/gateway/email/    smtp_mailer.go, log_mailer.go, templates/welcome.html
internal/scheduler/        scheduler.go, jobs.go
api/openapi.yaml, api/proto/contact/v1/contact.proto, buf.yaml, buf.gen.yaml
db/migrations/             rewritten clean set
test/                      integration tests (//go:build integration), testsupport/
Makefile, Dockerfile, docker-compose.yml, deploy/prometheus.yml, .env.example,
.golangci.yml, .mockery.yaml, .github/workflows/ci.yml, .github/dependabot.yml
```

---

## Phase 1 — Foundation

### Task 1: Module rename, Fiber v3, package docs

**Files:**
- Modify: `go.mod`, every `.go` file (import paths), all of `internal/delivery/http/**`, `internal/config/fiber.go`, `test/*.go`
- Create: `doc.go` in every package under `internal/`

**Interfaces:**
- Produces: module path `github.com/primestrider/golang-clean-architecture`. Fiber v3 handler signature `func(c fiber.Ctx) error`.

- [ ] **Step 1:** Create branch `feature/production-ready` from `upgrade-go-1.27.3`.
- [ ] **Step 2:** `go mod edit -module github.com/primestrider/golang-clean-architecture` and rewrite every import.
- [ ] **Step 3:** Replace `github.com/gofiber/fiber/v2` with `v3` (latest v3.x). Convert handlers to `fiber.Ctx`, `BodyParser` to `c.Bind().Body()`, `UserContext()` to `c.Context()`, and `app.Listen` / `app.Test` to the v3 signatures.
- [ ] **Step 4:** Add `doc.go` (Indonesian package doc) to each package, and godoc to every existing exported identifier.
- [ ] **Step 5:** Verify: `go build ./... && go vet ./...` gives no output. `go vet -tags integration ./test/` compiles.
- [ ] **Step 6:** Commit `refactor: rename module, migrate to Fiber v3, add package docs`.

### Task 2: Typed config with env overrides

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`, `.env.example`
- Modify: `config.json` (all keys from the Interfaces block below with non-secret defaults), `.gitignore` (add `.env`)
- Delete: `internal/config/viper.go`

**Interfaces:**
- Produces:
  - `type Config struct` with nested sections `App{Name, Env}`, `Server{Port, TrustedProxies []string, CORSOrigins []string, RequestTimeout, BodyLimit}`, `Admin{Port}`, `GRPC{Enabled, Port}`, `Log{Level, Format}`, `Database{Username, Password, Host, Port, Name, Pool{Idle, Max, Lifetime}}`, `JWT{Secret, Issuer, Audience, AccessTTL, RefreshTTL}`, `RateLimit{GlobalPerMinute, AuthPerMinute}`, `Kafka{Enabled, Brokers []string, GroupID, OffsetReset}`, `Outbox{PollInterval, BatchSize, MaxAttempts}`, `Consumer{MaxRetries, InitialBackoff}`, `Redis{Enabled, Addr, Password, DB}`, `Storage{Enabled, Endpoint, PublicEndpoint, AccessKey, SecretKey, Bucket, UseSSL, PresignTTL}`, `Email{Enabled, Host, Port, Username, Password, From, TLS}`, `OTel{Enabled, Endpoint, ServiceName, SampleRatio}`, `Docs{Enabled}`, `Scheduler{TokenCleanupCron, OutboxCleanupCron}`, `Shutdown{Timeout}`. Durations are `time.Duration`.
  - `func Load(paths ...string) (*Config, error)`
  - `func (c *Config) Validate() error`
  - `func (c *Config) IsProduction() bool`

- [ ] **Step 1: Write failing tests** in `config_test.go`:
  - `TestLoad_EnvOverridesFile`: `t.Setenv("APP_DATABASE_PASSWORD","s3cret")` → `cfg.Database.Password == "s3cret"`.
  - `TestValidate_ProductionRejectsShortJWTSecret`: env=production, secret 31 bytes → error mentions `jwt.secret`.
  - `TestValidate_ProductionRejectsWildcardCORS`: `CORSOrigins = ["*"]` → error.
  - `TestValidate_ProductionRejectsDocsEnabled` → error.
  - `TestValidate_DevelopmentAllowsDefaults` → nil.
- [ ] **Step 2:** `go test ./internal/config/ -run 'TestLoad|TestValidate'` fails (undefined).
- [ ] **Step 3:** Implement with Viper: `SetEnvPrefix("APP")`, `SetEnvKeyReplacer(strings.NewReplacer(".", "_"))`, `AutomaticEnv`. Register every key with `SetDefault` so env-only keys reach `Unmarshal` (use `mapstructure` tags). `Load` calls `Validate`.
- [ ] **Step 4:** Tests pass. Switch both `main.go` files to `config.Load()`.
- [ ] **Step 5:** Commit `feat(config): typed config with APP_ env overrides and production validation`.

### Task 3: slog logging with context attributes

**Files:**
- Create: `internal/observability/log.go`, `internal/observability/log_test.go`, `internal/config/logger.go`
- Modify: `internal/config/database.go` (rename from `gorm.go`; GORM logger adapter on slog), every file that uses logrus
- Remove: `github.com/sirupsen/logrus` from go.mod

**Interfaces:**
- Produces:
  - `func NewLogger(cfg *Config) *slog.Logger` (in `config`): JSON when `Log.Format=="json"`, text otherwise.
  - `func NewContextHandler(next slog.Handler) slog.Handler`
  - `func WithRequestID(ctx, id string) context.Context`, `RequestID(ctx) string`
  - `func WithUserID(ctx, id string) context.Context`, `UserID(ctx) string`
  - All components take `*slog.Logger` and log with `*Context(ctx, …)`.

- [ ] **Step 1: Failing test** `TestContextHandler_AddsRequestAndUserID`: log through a handler wrapping a JSON handler into a buffer, with ctx carrying request id `r1` and user id `u1` → output contains `"request_id":"r1"` and `"user_id":"u1"`. Also: an active OTel span context adds `trace_id`/`span_id` (use `trace.ContextWithSpanContext` with a fixed span context).
- [ ] **Step 2:** Run: fails.
- [ ] **Step 3:** Implement. GORM adapter: slow threshold 200ms at warn, ordinary queries at debug, `IgnoreRecordNotFoundError: true`, `ParameterizedQueries: true`.
- [ ] **Step 4:** Tests pass. `go mod tidy` removes logrus.
- [ ] **Step 5:** Commit `feat(log): replace logrus with slog and context-aware handler`.

### Task 4: apperror, validation and Problem Details

**Files:**
- Create: `internal/apperror/apperror.go` (+ `_test.go`), `internal/validation/validation.go` (+ `_test.go`), `internal/delivery/http/problem.go` (+ `_test.go`)
- Modify: `internal/config/fiber.go` (error handler); delete `internal/config/validator.go`

**Interfaces:**
- Produces:
  - `type Code string` with constants `CodeValidation="VALIDATION_FAILED"`, `CodeNotFound="NOT_FOUND"`, `CodeConflict="CONFLICT"`, `CodeUnauthorized="UNAUTHORIZED"`, `CodeForbidden="FORBIDDEN"`, `CodeRateLimited="RATE_LIMITED"`, `CodePayloadTooLarge="PAYLOAD_TOO_LARGE"`, `CodeUnsupportedMedia="UNSUPPORTED_MEDIA_TYPE"`, `CodeBadRequest="BAD_REQUEST"`, `CodeUnavailable="UNAVAILABLE"`, `CodeInternal="INTERNAL"`.
  - `type FieldError struct{ Field, Message string }`
  - `type Error struct{ Code Code; Message string; Fields []FieldError; Err error }` with `Error()` and `Unwrap()`.
  - Constructors: `New(code, msg) *Error`, `Wrap(code, msg, err) *Error`, `NotFound(resource string) *Error`, `Conflict(msg) *Error`, `Unauthorized(msg) *Error`, `Forbidden(msg) *Error`, `Internal(err error) *Error` (message is always `"internal server error"`), `From(err error) *Error` (non-apperror becomes Internal).
  - `type Validator struct`; `func validation.New() *Validator`; `func (v *Validator) Struct(s any) error` returns `*apperror.Error{Code: CodeValidation}` whose `Fields` use the json tag name and English messages (e.g. `"must be a valid email"`, `"must be at most 100 characters"`, `"is required"`).
  - `func http.HTTPStatus(code apperror.Code) int` and `func http.NewErrorHandler(log *slog.Logger) fiber.ErrorHandler`. The handler also maps `*fiber.Error` (400/404/405/413/415) to the matching code.

- [ ] **Step 1: Failing tests:**
  - `TestValidator_FieldErrorsUseJSONNames`: struct `{Email string \`json:"email" validate:"email"\`}` with `"x"` → `Fields == [{Field:"email", Message:"must be a valid email"}]`.
  - `TestErrorHandler_ProblemDetails`: a Fiber app whose route returns `apperror.NotFound("contact")` → status 404, `Content-Type: application/problem+json`, body has `status:404`, `title`, `instance:"/x"`, `request_id`.
  - `TestErrorHandler_InternalHidesCause`: route returns `apperror.Internal(errors.New("dsn=root:pw"))` → body does not contain `"dsn"`, status 500.
  - `TestErrorHandler_ValidationIncludesFields`: body `errors[0].field == "email"`.
- [ ] **Step 2:** Run: fails.
- [ ] **Step 3:** Implement. The problem `type` is `https://problems.example.com/<kebab-code>`. 5xx errors are logged with the cause; 4xx at debug.
- [ ] **Step 4:** Tests pass.
- [ ] **Step 5:** Commit `feat(errors): typed apperror, field validation and RFC 9457 problem details`.

### Task 5: Transactor, repository ports, use-case refactor, /api/v1

**Files:**
- Create: `internal/repository/transactor.go` (+ `_test.go` is an integration test in Task 8), `internal/usecase/ports.go`
- Modify: all repositories, use cases, controllers, `internal/config/app.go` → rename to `web.go`, `route/route.go`
- Delete: `internal/gateway/messaging/*_producer.go` direct publishes in use cases (temporarily keep a no-op `EventPublisher` port; the outbox replaces it in Task 18)

**Interfaces:**
- Produces:
  - `type Transactor interface{ WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error }`
  - `func repository.NewTransactor(db *gorm.DB) *GormTransactor`
  - `func repository.DB(ctx context.Context, base *gorm.DB) *gorm.DB`: the tx from ctx if present, else `base.WithContext(ctx)`.
  - Generic `Repository[T]{DB *gorm.DB}` methods: `Create(ctx, *T) error`, `Update(ctx, *T) error`, `Delete(ctx, *T) error`, `FindByID(ctx, id string) (*T, error)`. Not found returns `apperror.NotFound(<entity name>)`.
  - `ContactRepository.FindByIDAndUserID(ctx, id, userID string) (*entity.Contact, error)`, `Search(ctx, *model.SearchContactRequest) ([]entity.Contact, int64, error)`
  - `AddressRepository.FindByIDAndContactID(ctx, id, contactID string) (*entity.Address, error)`, `FindAllByContactID(ctx, contactID string) ([]entity.Address, error)`
  - `ports.go` declares `UserRepository`, `ContactRepository`, `AddressRepository`, and `EventPublisher interface{ Publish(ctx context.Context, events ...model.Event) error }`, matching those signatures. Later tasks add methods there.
  - Use case constructors: `NewXxxUseCase(log *slog.Logger, v *validation.Validator, tx Transactor, …ports)`. Methods take `context.Context` and return `*apperror.Error`-compatible errors. They never import Fiber.
  - `func config.BootstrapWeb(deps *WebDeps) ` where `WebDeps{Config, Log, DB, App}`. Routes are prefixed `/api/v1`.

- [ ] **Step 1:** Implement the transactor and the ctx-based repositories. Refactor use cases: reads skip transactions; writes use `WithinTransaction`.
- [ ] **Step 2:** Update `test/*.go` paths to `/api/v1`. Keep them compiling (`go vet -tags integration ./test/`).
- [ ] **Step 3:** Verify: `grep -r "gofiber" internal/usecase` is empty; build/vet pass.
- [ ] **Step 4:** Commit `refactor: context transactor, repository ports, framework-free use cases, /api/v1`.

### Task 6: Admin server, health checks, graceful shutdown

**Files:**
- Create: `internal/delivery/admin/server.go` (+ `_test.go`)
- Modify: `cmd/web/main.go`, `cmd/worker/main.go`, `internal/delivery/messaging/consumer.go` (WaitGroup-based stop)

**Interfaces:**
- Produces:
  - `type Check func(ctx context.Context) error`
  - `func admin.NewServer(addr string, log *slog.Logger, checks map[string]Check) *Server`
  - `(*Server) Handle(pattern string, h http.Handler)` (Task 21 mounts `/metrics` here)
  - `(*Server) SetReady(bool)`, `Start() error`, `Shutdown(ctx) error`
  - Routes: `GET /health/live` → 200 `{"status":"ok"}`. `GET /health/ready` → 200, or 503 with `{"status":"unavailable","checks":{"db":"<err>"}}`.

- [ ] **Step 1: Failing tests:** `TestReady_FailsWhenCheckFails` (503 and the check name in body); `TestReady_503AfterSetReadyFalse`; `TestLive_AlwaysOK`.
- [ ] **Step 2:** Run: fails.
- [ ] **Step 3:** Implement. In both mains use `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`. Shutdown order follows §5.0: ready=false → stop intake → wait with `cfg.Shutdown.Timeout` → close clients → flush. Remove `time.Sleep` and `SIGKILL` from the worker.
- [ ] **Step 4:** Tests pass. Manual check: `go run ./cmd/web` with a reachable DB, then Ctrl+C logs `shutdown complete`. If no DB is available, skip and rely on the Task 8 integration test.
- [ ] **Step 5:** Commit `feat: admin server with health checks and graceful shutdown`.

## Phase 2 — Testing & CI

### Task 7: Mocks and use-case unit tests

**Files:**
- Create: `.mockery.yaml`, `internal/usecase/mocks/` (generated), `internal/usecase/contact_usecase_test.go`, `address_usecase_test.go`, `user_usecase_test.go`

**Interfaces:**
- Consumes: ports from Task 5. Tests use a `fakeTransactor` (in `internal/usecase/testhelpers_test.go`) that just calls `fn(ctx)`.

- [ ] **Step 1:** Configure mockery v3 (testify template, output `internal/usecase/mocks`, one mock per interface in `ports.go`). Run `mockery`.
- [ ] **Step 2: Write tests:**
  - `TestContactCreate_ValidationError` → `apperror.CodeValidation`, repo not called.
  - `TestContactGet_NotFound` → repo returns NotFound → error code NotFound.
  - `TestContactCreate_Success_PublishesEvent` → EventPublisher called once inside the transaction (Task 18 tightens this to assert `EventType == "contact.created"` once `model.Event` becomes the envelope struct; the `EventPublisher` signature stays the same).
  - `TestAddressCreate_ContactNotOwned` → NotFound.
  - `TestUserRegister_DuplicateEmail` → Conflict.
- [ ] **Step 3:** `go test ./internal/usecase/... -race` passes.
- [ ] **Step 4:** Commit `test: mockery mocks and use-case unit tests`.

### Task 8: Testcontainers integration harness

**Files:**
- Create: `test/testsupport/containers.go`, `test/testsupport/app.go`
- Modify: `test/*_test.go` (add `//go:build integration`, make each test independent), delete `test/init.go`, `test/helper_test.go` → becomes fixtures in testsupport

**Interfaces:**
- Produces:
  - `func testsupport.StartMySQL(t testing.TB) (dsn string)`: MySQL 8 container, runs `db/migrations` via golang-migrate as a library, cleans up with `t.Cleanup`.
  - `func testsupport.StartRedis(t) string`, `StartKafka(t) []string`
  - `func testsupport.NewWebApp(t, opts ...Option) *TestApp` where `TestApp{App *fiber.App, DB *gorm.DB, Cfg *config.Config}`
  - Fixtures: `CreateUser(t, app, email, password) *entity.User`, `Login(t, app, email, password) (access, refresh string)`, `CreateContact(t, app, userID) *entity.Contact`
  - `func (a *TestApp) Do(t, method, path string, body any, token string) (*http.Response, []byte)`
  - One container per package via `TestMain`. Each test truncates tables through `testsupport.Reset(t, db)`.

- [ ] **Step 1:** Implement the harness. Port the existing user/contact/address tests to it (paths `/api/v1/...`), with no test calling another test.
- [ ] **Step 2:** Add `TestTransactor_RollsBackOnError` (integration): inside `WithinTransaction` create a contact, then return an error → contact absent.
- [ ] **Step 3:** Run `go test -tags integration ./test/... ./internal/repository/...`. Expected: PASS (requires Docker). If Docker is unavailable locally, record that and rely on CI.
- [ ] **Step 4:** Commit `test: testcontainers-based independent integration tests`.

### Task 9: Lint, CI, vulnerability scan

**Files:**
- Create: `.golangci.yml`, `.github/workflows/ci.yml`, `.github/dependabot.yml`, `Makefile` (initial targets `build`, `test`, `test-integration`, `lint`, `vuln`, `mocks`)

- [ ] **Step 1:** golangci-lint v2 config enabling govet, staticcheck, errcheck, gosec, revive (rule `exported` on), bodyclose, sqlclosecheck, errorlint, gocritic. Exclude `internal/delivery/grpc/pb` and `mocks`.
- [ ] **Step 2:** Fix all findings. `golangci-lint run` exits 0.
- [ ] **Step 3:** CI jobs: `lint`; `unit` (`go test -race -coverprofile=coverage.out ./internal/...`); `integration` (`go test -tags integration ./...`); `vuln` (`govulncheck ./...`); `docker` (`docker build .`, added once the Dockerfile exists in Task 23; leave the job with an `if: hashFiles('Dockerfile') != ''`). Go version comes from `go.mod` (`go-version-file`). Dependabot covers gomod, github-actions and docker, weekly.
- [ ] **Step 4:** Run `govulncheck ./...`: no vulnerabilities reachable.
- [ ] **Step 5:** Commit `ci: golangci-lint, GitHub Actions, govulncheck, dependabot`.

## Phase 3 — Security

### Task 10: Rewritten migrations (final schema)

**Files:**
- Delete: the 6 files in `db/migrations/`
- Create: `db/migrations/000001_create_users.{up,down}.sql`, `000002_create_contacts`, `000003_create_addresses`, `000004_create_refresh_tokens`, `000005_create_audit_logs`, `000006_create_outbox_events`, `000007_create_processed_events`

**Schema (exact):**
- `users`: `id CHAR(36) PK`, `email VARCHAR(255) NOT NULL`, `name VARCHAR(100) NOT NULL`, `password VARCHAR(100) NOT NULL`, `role VARCHAR(20) NOT NULL DEFAULT 'user'`, `avatar_key VARCHAR(255) NULL`, `version INT NOT NULL DEFAULT 1`, `created_at BIGINT NOT NULL`, `updated_at BIGINT NOT NULL`, `deleted_at BIGINT NOT NULL DEFAULT 0`, `UNIQUE (email, deleted_at)`.
- `contacts`: the current columns plus `version`, `deleted_at`, `INDEX (user_id, deleted_at)`, FK `user_id`.
- `addresses`: the current columns plus `version`, `deleted_at`, `INDEX (contact_id, deleted_at)`, FK `contact_id`.
- `refresh_tokens`: `id CHAR(36) PK`, `user_id CHAR(36) NOT NULL`, `family_id CHAR(36) NOT NULL`, `token_hash CHAR(64) NOT NULL UNIQUE`, `expires_at BIGINT NOT NULL`, `revoked_at BIGINT NULL`, `replaced_by_id CHAR(36) NULL`, `user_agent VARCHAR(255)`, `ip VARCHAR(45)`, `created_at BIGINT NOT NULL`, `INDEX (user_id)`, `INDEX (family_id)`.
- `audit_logs`: `id CHAR(36) PK`, `actor_id CHAR(36) NULL`, `action VARCHAR(100) NOT NULL`, `entity_type VARCHAR(50) NOT NULL`, `entity_id CHAR(36) NOT NULL`, `request_id VARCHAR(64) NULL`, `changes JSON NULL`, `created_at BIGINT NOT NULL`, `INDEX (entity_type, entity_id)`.
- `outbox_events`: `id CHAR(36) PK`, `topic VARCHAR(100)`, `msg_key VARCHAR(100)`, `aggregate_id CHAR(36)`, `event_type VARCHAR(100)`, `payload JSON`, `headers JSON`, `status VARCHAR(20) NOT NULL DEFAULT 'pending'`, `attempts INT NOT NULL DEFAULT 0`, `last_error TEXT NULL`, `created_at BIGINT`, `published_at BIGINT NULL`, `INDEX (status, created_at)`.
- `processed_events`: `consumer_name VARCHAR(100)`, `event_id CHAR(36)`, `processed_at BIGINT`, `PRIMARY KEY (consumer_name, event_id)`.
- Engine InnoDB, `utf8mb4`.

- [ ] **Step 1:** Write the migrations. Verify with a container: the harness from Task 8 runs `up` and then `down` cleanly (`TestMigrations_UpDown`, integration).
- [ ] **Step 2:** Commit `feat(db): rewrite migrations into final schema`.

### Task 11: Security primitives

**Files:**
- Create: `internal/security/password.go`, `jwt.go`, `refreshtoken.go` (+ `_test.go` each)

**Interfaces:**
- Produces:
  - `func HashPassword(pw string) (string, error)` (bcrypt default cost)
  - `func ComparePassword(hash, pw string) bool`
  - `func CompareDummy(pw string)`: compares against a precomputed hash to equalize timing.
  - `type Claims struct{ UserID, Role string; jwt.RegisteredClaims }`
  - `type TokenIssuer struct`; `func NewTokenIssuer(secret []byte, issuer, audience string, ttl time.Duration, now func() time.Time) *TokenIssuer`
  - `(*TokenIssuer) Issue(userID, role string) (token string, expiresAt time.Time, err error)` (sets a uuid `jti`)
  - `(*TokenIssuer) Parse(token string) (*Claims, error)`: requires HS256 and validates iss, aud and exp. Errors map to `apperror.Unauthorized`.
  - `func NewRefreshToken() (plain string, hash string, err error)`, `func HashRefreshToken(plain string) string`

- [ ] **Step 1: Failing tests:**
  - `TestParse_RejectsAlgNone`, `TestParse_RejectsWrongAudience`, `TestParse_RejectsExpired` (inject `now`), `TestParse_RejectsRS256Header`.
  - `TestIssue_RoundTrip` (sub/role preserved).
  - `TestNewRefreshToken_43CharsBase64URLAndHashIs64Hex`.
  - `TestComparePassword`.
- [ ] **Step 2:** Run: fails. **Step 3:** Implement. **Step 4:** Pass.
- [ ] **Step 5:** Commit `feat(security): password hashing, JWT issuer/parser, refresh token primitives`.

### Task 12: New user model (UUID + email + role)

**Files:**
- Modify: `internal/entity/user_entity.go`, `internal/model/user_model.go`, `converter/user_converter.go`, `repository/user_repository.go`, `usecase/user_usecase.go`, `delivery/http/user_controller.go`, `ports.go`, unit and integration tests

**Interfaces:**
- Produces:
  - `entity.User{ID, Email, Name, Password, Role, AvatarKey *string, Version int, CreatedAt, UpdatedAt int64, DeletedAt soft_delete.DeletedAt}`
  - `UserRepository`: `Create`, `FindByID`, `FindByEmail(ctx, email) (*entity.User, error)`, `Update`, `List(ctx, page, size int) ([]entity.User, int64, error)`. A duplicate email on `Create` → `apperror.Conflict("email already registered")` (detect MySQL error 1062).
  - `RegisterUserRequest{Email (required,email,max=255), Password (required,min=8,max=72), Name (required,max=100)}`
  - `UpdateUserRequest{ID "-", Name (omitempty,max=100), CurrentPassword, NewPassword (omitempty,min=8,max=72), Version (required,min=1)}`. A non-empty `NewPassword` requires a matching `CurrentPassword`, else `CodeValidation` on field `current_password`.
  - `UserResponse{ID, Email, Name, Role, AvatarURL string(omitempty), Version, CreatedAt, UpdatedAt}`
  - Routes: `POST /api/v1/users` (register), `GET|PATCH /api/v1/users/_current`.
  - Emails are lower-cased and trimmed before validation.

- [ ] **Step 1: Failing unit tests:** `TestRegister_NormalizesEmail`; `TestRegister_PasswordTooShort` (field `password`); `TestUpdate_NewPasswordRequiresCurrent`; `TestUpdate_PasswordChangeRevokesAllRefreshTokens` (expects `RefreshTokenRepository.RevokeAllForUser`; declare that port here, implement in Task 13).
- [ ] **Step 2:** Run: fails. **Step 3:** Implement. **Step 4:** Unit and integration user tests pass.
- [ ] **Step 5:** Commit `feat(user): uuid ids, email login identity, roles`.

### Task 13: Auth use case — login, refresh rotation, logout

**Files:**
- Create: `internal/entity/refresh_token_entity.go`, `internal/repository/refresh_token_repository.go`, `internal/usecase/auth_usecase.go` (+ `_test.go`), `internal/model/auth_model.go`, `internal/delivery/http/auth_controller.go`, `test/auth_test.go`

**Interfaces:**
- Produces:
  - `RefreshTokenRepository`: `Create(ctx, *entity.RefreshToken) error`, `FindByHashForUpdate(ctx, hash string) (*entity.RefreshToken, error)` (`SELECT … FOR UPDATE`), `Revoke(ctx, id string, replacedBy *string) error`, `RevokeFamily(ctx, familyID string) error`, `RevokeAllForUser(ctx, userID string) error`, `DeleteExpiredBefore(ctx, cutoff int64) (int64, error)`
  - `AuthUseCase`: `Login(ctx, *model.LoginRequest) (*model.TokenResponse, error)`, `Refresh(ctx, *model.RefreshRequest) (*model.TokenResponse, error)`, `Logout(ctx, *model.LogoutRequest) error`
  - `LoginRequest{Email, Password, UserAgent "-", IP "-"}`, `RefreshRequest{RefreshToken (required), UserAgent, IP}`, `LogoutRequest{RefreshToken (required)}`
  - `TokenResponse{AccessToken, RefreshToken, TokenType:"Bearer", ExpiresIn int (seconds)}`
  - Routes: `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/logout`.
  - Ports used: `TokenIssuer interface{ Issue(userID, role string) (string, time.Time, error); Parse(string) (*security.Claims, error) }`.

- [ ] **Step 1: Failing unit tests:**
  - `TestLogin_UnknownEmail_SameErrorAsWrongPassword` (both `CodeUnauthorized`, message `"invalid email or password"`; `CompareDummy` invoked for unknown email via injectable func).
  - `TestRefresh_RotatesToken` (old revoked with `replaced_by_id`, new one in the same family).
  - `TestRefresh_ReusedTokenRevokesFamily` (revoked token → `RevokeFamily` called, `CodeUnauthorized`, warning logged).
  - `TestRefresh_Expired` → Unauthorized.
  - `TestLogout_RevokesFamily`.
- [ ] **Step 2: Failing integration test (Review Focus #1)** `TestRefresh_ConcurrentSameToken`: 2 goroutines refresh the same token → exactly one 200; the other 401; afterwards, refreshing with the winner's new token also fails (family revoked).
- [ ] **Step 3:** Run: fails. **Step 4:** Implement. Login and refresh run inside `WithinTransaction`. Audit writes come in Task 17.
- [ ] **Step 5:** Tests pass. **Step 6:** Commit `feat(auth): JWT access tokens with rotating refresh tokens and reuse detection`.

### Task 14: HTTP security middleware stack and rate limiting

**Files:**
- Create: `internal/delivery/http/middleware/auth.go` (replace `auth_middleware.go`), `role.go`, `ratelimit.go`, `stack.go`; `test/security_test.go`
- Modify: `internal/config/fiber.go` (`TrustedProxies`, `ProxyHeader`, `BodyLimit`), `route/route.go`

**Interfaces:**
- Produces:
  - `func middleware.NewAuth(issuer TokenIssuer) fiber.Handler`: requires `Authorization: Bearer <jwt>`. On success it stores `*model.Auth{UserID, Role}` in `c.Locals("auth")` and puts the user id into ctx via `observability.WithUserID`.
  - `func middleware.CurrentAuth(c fiber.Ctx) *model.Auth`
  - `func middleware.RequireRole(roles ...string) fiber.Handler` → `apperror.Forbidden`
  - `func middleware.NewRateLimiter(max int, window time.Duration, key func(fiber.Ctx) string, storage fiber.Storage) fiber.Handler` → 429 problem+json with `Retry-After`. `storage == nil` means memory.
  - `func middleware.Apply(app *fiber.App, cfg *config.Config, log *slog.Logger)` applies, in order: recover, requestid (accept an incoming `X-Request-ID` matching `^[A-Za-z0-9-]{1,64}$`, else generate one), access log, helmet, CORS (`cfg.Server.CORSOrigins`), global limiter (100/min/IP), timeout (10s). OTel and metrics are inserted by Tasks 21–22 at the positions given in §5.1.
  - Auth-route limiter: 5/min keyed by `ip + ":" + lower(email)` for login/register, and by ip for refresh.
  - Every non-JSON `Content-Type` on a JSON body route → 415.

- [ ] **Step 1: Failing integration tests:**
  - `TestAuth_MissingBearer401`, `TestAuth_AlgNoneToken401`.
  - `TestLoginRateLimit_SixthAttempt429WithRetryAfter`.
  - `TestRateLimit_IgnoresSpoofedXForwardedFor` (Review Focus #3): with no trusted proxies, 6 logins carrying a different `X-Forwarded-For` each still give 429 on the 6th.
  - `TestMalformedJSON400`, `TestOversizedBody413`, `TestWrongContentType415` (Review Focus #5; all problem+json).
  - `TestPanicRecovered500`: a test-only route that panics → 500 problem+json, process alive.
  - `TestSecurityHeadersPresent` (`X-Content-Type-Options: nosniff`).
  - `TestRequestIDEchoed`.
- [ ] **Step 2:** Run: fails. **Step 3:** Implement. **Step 4:** Pass.
- [ ] **Step 5:** Commit `feat(http): security middleware stack, bearer auth, rate limiting`.

### Task 15: RBAC admin endpoints and create-admin CLI

**Files:**
- Create: `internal/usecase/admin_usecase.go` (+ `_test.go`), `internal/delivery/http/admin_controller.go`, `cmd/cli/main.go`, `test/admin_test.go`; Makefile target `create-admin`

**Interfaces:**
- Produces:
  - `AdminUseCase.ListUsers(ctx, *model.ListUsersRequest{Page (min=1), Size (min=1,max=100)}) ([]model.UserResponse, *model.PageMetadata, error)`
  - `AdminUseCase.ChangeRole(ctx, *model.ChangeRoleRequest{ActorID, UserID, Role (oneof=user admin), Version (required,min=1)}) (*model.UserResponse, error)`: an admin cannot demote themself → Conflict.
  - Routes under `RequireRole("admin")`: `GET /api/v1/admin/users`, `PATCH /api/v1/admin/users/:id/role`.
  - `go run ./cmd/cli create-admin --email x@y.z` promotes an existing user to admin and prints the result. Exit 1 if the user is not found.

- [ ] **Step 1: Failing tests:** `TestAdminUsers_ForbiddenForUserRole` (403), `TestAdminUsers_OKForAdmin`, `TestChangeRole_CannotDemoteSelf`.
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(rbac): role-based admin endpoints and create-admin CLI`.

## Phase 4 — Data reliability

### Task 16: Soft delete and optimistic locking

**Files:**
- Modify: entities user/contact/address (`DeletedAt soft_delete.DeletedAt \`gorm:"softDelete:milli"\``, `Version int`), repositories, contact/address/user use cases and models, tests

**Interfaces:**
- Produces:
  - `Repository[T].UpdateWithVersion(ctx, entity *T, expectedVersion int) error`: runs `UPDATE … SET …, version = version + 1 WHERE id = ? AND version = ?`. 0 rows → re-check existence: missing → `NotFound`, else `apperror.Conflict("resource was modified by another request")`. On success the entity's `Version` is incremented.
  - `UpdateContactRequest` and `UpdateAddressRequest` gain `Version int \`json:"version" validate:"required,min=1"\``. Responses gain `version`.
  - `ContactUseCase.Delete` soft-deletes the contact and all its addresses in one transaction.

- [ ] **Step 1: Failing integration tests:**
  - `TestUpdateContact_StaleVersion409`.
  - `TestUpdateContact_VersionIncrements`.
  - `TestDeletedContact_NotListedAndGet404`.
  - `TestDeleteContact_SoftDeletesAddresses`.
  - `TestUpdate_StaleVersionOnSoftDeleted404` (Review Focus #4).
  - `TestRegister_EmailReusableAfterSoftDelete`.
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(data): soft delete and optimistic locking`.

### Task 17: Audit log

**Files:**
- Create: `internal/entity/audit_log_entity.go`, `internal/repository/audit_repository.go`, `internal/usecase/audit.go` (+ `_test.go`)
- Modify: user/auth/contact/address/admin use cases

**Interfaces:**
- Produces:
  - `AuditRepository.Create(ctx, *entity.AuditLog) error`
  - `func diffChanges(before, after any, exclude ...string) map[string][2]any` (in `usecase/audit.go`): compares json-tagged fields and always excludes `password`, `token_hash` and `refresh_token`.
  - `func (r *auditRecorder) Record(ctx, action, entityType, entityID string, before, after any) error`: takes actor and request id from ctx and is called inside the business transaction.
  - Actions: `user.registered`, `user.updated`, `user.password_changed`, `user.role_changed`, `auth.refresh_reuse_detected`, `contact.created|updated|deleted`, `address.created|updated|deleted`.

- [ ] **Step 1: Failing tests:** `TestDiffChanges_ExcludesPassword`, `TestDiffChanges_OnlyChangedFields`. Integration `TestUpdateContact_WritesAuditInSameTx` (forcing a failure after the audit write leaves no audit row).
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(audit): transactional audit trail without sensitive fields`.

### Task 18: Event envelope and outbox writes

**Files:**
- Create: `internal/model/event.go` (replace the per-entity event files), `internal/entity/outbox_event_entity.go`, `internal/repository/outbox_repository.go`, `internal/usecase/outbox_publisher.go`
- Delete: `internal/model/*_event.go`, `internal/gateway/messaging/*_producer.go`, `converter/*ToEvent`
- Modify: use cases (publish inside the transaction), `config/web.go`

**Interfaces:**
- Produces:
  - `type Event struct{ EventID, EventType string; OccurredAt time.Time; AggregateID, ActorID string; Data json.RawMessage }` (json snake_case per §5.3)
  - `func NewEvent(eventType, aggregateID, actorID string, data any) (Event, error)`
  - Topic mapping: `user.*` → `users`, `contact.*` → `contacts`, `address.*` → `addresses`. Defined once in `model.TopicFor(eventType string) string`.
  - `OutboxRepository`: `Insert(ctx, ...entity.OutboxEvent) error`, `FetchPending(ctx, limit int) ([]entity.OutboxEvent, error)` (status=pending ordered by created_at, id), `MarkPublished(ctx, id string, at int64) error`, `MarkFailedAttempt(ctx, id string, errMsg string, maxAttempts int) error` (sets status `failed` when attempts reach max), `CountByStatus(ctx) (map[string]int64, error)`, `DeletePublishedBefore(ctx, cutoff int64) (int64, error)`
  - `OutboxPublisher` implements `EventPublisher`: converts each `Event` into an `OutboxEvent` with `msg_key = aggregate_id`. Headers carry the trace context (filled in Task 22). The outbox is always written (§5.3).
  - Data payloads: user events carry `id, email, name, role` (never the password).

- [ ] **Step 1: Failing tests:** `TestNewEvent_Fields`, `TestTopicFor`. Integration `TestCreateContact_WritesOutboxRowInSameTx`. Unit `TestUserEventPayload_HasNoPassword`.
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(events): event envelope and transactional outbox`.

### Task 19: Outbox relay with leader lock and ordering

**Files:**
- Create: `internal/outbox/relay.go` (+ `_test.go`), `dispatcher_kafka.go`, `dispatcher_local.go`, `test/outbox_test.go`
- Modify: `internal/config/kafka.go` (producer only built when `Kafka.Enabled`), `config/worker.go`, `cmd/worker/main.go`

**Interfaces:**
- Consumes: `OutboxRepository` (Task 18). `messaging.Registry` from Task 20 for the local dispatcher. Build the relay first with a fake dispatcher; wire the local dispatcher in Task 20.
- Produces:
  - `type Dispatcher interface{ Dispatch(ctx context.Context, e entity.OutboxEvent) error }`
  - `type Locker interface{ TryLock(ctx context.Context) (bool, error); Unlock(ctx context.Context) error }`
  - `func NewMySQLLocker(db *gorm.DB, name string) *MySQLLocker`: holds one dedicated `*sql.Conn` and uses `GET_LOCK(name, 0)` / `RELEASE_LOCK`.
  - `func NewRelay(repo OutboxRepository, d Dispatcher, l Locker, cfg config.Outbox, log *slog.Logger) *Relay`
  - `(*Relay) Run(ctx) error`: loops every `PollInterval` and only processes while holding the lock.
  - `(*Relay) ProcessBatch(ctx) (published int, err error)`: fetches `BatchSize` pending rows. If a row fails, it marks a failed attempt and skips later rows with the same `aggregate_id` in this batch. If a row with status `failed` exists for an aggregate, pending rows for that aggregate are skipped (blocking per §5.3). Implement via `FetchPending` excluding aggregates that have a `failed` row.

- [ ] **Step 1: Failing unit tests (fake repo + dispatcher):**
  - `TestProcessBatch_PreservesOrderPerAggregate` (A1 fails → A2 not dispatched; B1 dispatched).
  - `TestProcessBatch_MarksFailedAfterMaxAttempts`.
  - `TestRun_SkipsWhenLockNotAcquired`.
- [ ] **Step 2: Failing integration tests:**
  - `TestMySQLLocker_OnlyOneHolder` (two lockers on the same DB).
  - `TestRelay_KafkaDownKeepsPendingThenPublishes` (Review Focus #2): start the relay with a dispatcher whose Kafka container is paused → rows stay pending with `attempts > 0`; unpause → rows become published.
- [ ] **Step 3–4:** Implement → pass.
- [ ] **Step 5:** Commit `feat(outbox): leader-elected relay with per-aggregate ordering`.

### Task 20: Consumer framework — registry, idempotency, retry, DLQ

**Files:**
- Create: `internal/delivery/messaging/registry.go` (+ `_test.go`), `internal/entity/processed_event_entity.go`, `internal/repository/processed_event_repository.go`
- Modify: `internal/delivery/messaging/consumer.go`, the `*_consumer.go` handlers, `outbox/dispatcher_local.go`, `config/worker.go`

**Interfaces:**
- Produces:
  - `type Handler func(ctx context.Context, e model.Event) error`
  - `type Registry struct`; `func NewRegistry(tx usecase.Transactor, processed ProcessedEventRepository, log *slog.Logger) *Registry`
  - `(*Registry) Register(consumerName, eventType string, h Handler)`
  - `(*Registry) Handle(ctx, e model.Event) error`: for each handler of `e.EventType`, inside one transaction, `processed.TryMark(ctx, consumerName, e.EventID)` (INSERT IGNORE, returns false when already present → skip), then `h`.
  - `ProcessedEventRepository.TryMark(ctx, consumer, eventID string) (bool, error)`, `DeleteBefore(ctx, cutoff int64) (int64, error)`
  - Kafka `ConsumerGroupHandler` decodes `model.Event` and calls `Registry.Handle` with retries (`cfg.Consumer.MaxRetries`, backoff `InitialBackoff * 2^n`). After exhausting retries it produces to `<topic>.dlq` with headers `x-error`, `x-original-topic`, `x-attempts`, then marks the offset. An undecodable message goes straight to the DLQ.
  - `LocalDispatcher{Registry}` implements `outbox.Dispatcher` by decoding the payload and calling `Registry.Handle`.
  - Kafka trace context is extracted from message headers (Task 22 fills the propagation).

- [ ] **Step 1: Failing tests:**
  - `TestRegistry_DuplicateEventProcessedOnce` (integration, Review Focus-adjacent: a redelivery after a worker restart).
  - `TestConsumer_RetriesThenDLQ` (unit with a sarama mocks producer: handler always errors → 1 + 3 attempts, one DLQ message with the headers).
  - `TestConsumer_PoisonMessageToDLQ`.
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(messaging): idempotent handler registry with retry and DLQ`.

## Phase 5 — Observability & tooling

### Task 21: Prometheus metrics

**Files:**
- Create: `internal/observability/metrics.go` (+ `_test.go`), `internal/delivery/http/middleware/metrics.go`
- Modify: admin server wiring in both mains, relay, consumer

**Interfaces:**
- Produces:
  - `type Metrics struct{ HTTPRequests *prometheus.CounterVec; HTTPDuration *prometheus.HistogramVec; OutboxPending, OutboxFailed prometheus.Gauge; OutboxPublishFailures prometheus.Counter; MessagesProcessed, MessagesDLQ *prometheus.CounterVec }`
  - `func NewMetrics(reg prometheus.Registerer) *Metrics`. It also registers the Go/process collectors and `collectors.NewDBStatsCollector(sqlDB, dbName)`.
  - HTTP labels are `method`, `route` (the Fiber route template, e.g. `/api/v1/contacts/:contactId`) and `status`.
  - `/metrics` is mounted on the admin server only.

- [ ] **Step 1: Failing tests:** `TestHTTPMetrics_UsesRouteTemplate` (requesting `/api/v1/contacts/<uuid>` records `route="/api/v1/contacts/:contactId"`); `TestMetricsNotOnPublicPort` (GET `/metrics` on the Fiber app → 404).
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(metrics): prometheus metrics on admin port`.

### Task 22: OpenTelemetry tracing

**Files:**
- Create: `internal/config/otel.go`, `internal/observability/tracing.go` (+ `_test.go`)
- Modify: Fiber stack, GORM setup (`gorm.io/plugin/opentelemetry/tracing`), outbox publisher (inject), consumer and local dispatcher (extract)

**Interfaces:**
- Produces:
  - `func config.NewTracerProvider(ctx, cfg *Config) (shutdown func(context.Context) error, err error)`: OTLP gRPC exporter, `ParentBased(TraceIDRatioBased(SampleRatio))`, W3C propagator. When disabled, it is a no-op and returns a no-op shutdown.
  - `func observability.InjectHeaders(ctx) map[string]string`, `ExtractContext(ctx, headers map[string]string) context.Context`
  - Fiber uses the gofiber contrib `otelfiber` middleware for v3. If it is unavailable for v3, use a small middleware that starts a server span named `METHOD route` and leave a comment.

- [ ] **Step 1: Failing test:** `TestInjectExtract_RoundTrip` (the trace id survives a map round trip). Integration: an outbox row created inside a traced request has a non-empty `traceparent` header.
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(tracing): OpenTelemetry across HTTP, DB and events`.

### Task 23: Docker, compose, Makefile

**Files:**
- Create: `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `deploy/prometheus.yml`
- Modify: `Makefile` (add `run-web`, `run-worker`, `migrate-up`, `migrate-down`, `migrate-create`, `proto`, `compose-up`, `compose-full`, `compose-down`), `.env.example`, CI docker job

**Decisions:**
- Dockerfile: builder `golang:1.27.3` with `CGO_ENABLED=0`, building `/out/web`, `/out/worker` and `/out/cli`. Runtime `gcr.io/distroless/static:nonroot`, `USER nonroot`, default `ENTRYPOINT ["/app/web"]`.
- Compose default services: `mysql` (8.4, healthcheck), `migrate` (`migrate/migrate` image, runs `up` and exits), `web`, `worker`.
- Profile `full`: `kafka` (`apache/kafka`, KRaft single node), `redis`, `minio` + `minio-init` (creates the bucket), `mailpit`, `jaeger`, `prometheus`.
- `full` services set their `APP_*_ENABLED=true` via an `env_file` or `.env`.

- [ ] **Step 1:** Write the files. Verify that `docker compose config` and `docker compose --profile full config` are valid and that `docker build -t gca .` succeeds.
- [ ] **Step 2:** `docker compose up -d` → `curl localhost:9090/health/ready` returns 200, and a register + login via `curl` works.
- [ ] **Step 3:** Commit `build: Dockerfile, docker-compose profiles, Makefile`.

## Phase 6 — Features

### Task 24: Redis — cache, limiter storage, lock

**Files:**
- Create: `internal/config/redis.go`, `internal/gateway/cache/redis_cache.go` (+ integration test), `internal/scheduler/redis_locker.go`
- Modify: contact use case (`Get`, `Update`, `Delete`), rate limiter wiring, readiness checks

**Interfaces:**
- Produces:
  - Port `Cache interface{ Get(ctx, key string, dst any) (bool, error); Set(ctx, key string, v any, ttl time.Duration) error; Delete(ctx, keys ...string) error }`. A `NoopCache` is used when Redis is disabled.
  - Key `contact:{userID}:{contactID}`, TTL 5m. Invalidate **after** commit on update and delete. Cache errors are logged, never failing the request.
  - Rate-limiter storage: `github.com/gofiber/storage/redis` (v3-compatible) when enabled, otherwise memory.
  - `redis.NewClient` is instrumented with `redisotel`. A readiness check `redis` is added when enabled.

- [ ] **Step 1: Failing tests:** unit `TestContactGet_CacheHitSkipsRepo`, `TestContactUpdate_InvalidatesCacheAfterCommit`, `TestContactGet_CacheErrorFallsBackToRepo`. Integration `TestRedisCache_SetGetDelete`.
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(redis): cache-aside, distributed rate limit storage`.

### Task 25: Scheduler jobs

**Files:**
- Create: `internal/scheduler/scheduler.go`, `jobs.go` (+ `_test.go`)
- Modify: `config/worker.go`, `cmd/worker/main.go`

**Interfaces:**
- Produces:
  - `func NewScheduler(cfg config.Scheduler, locker gocron.Locker /* nil = no lock */, log *slog.Logger, jobs Jobs) (gocron.Scheduler, error)`
  - `type Jobs struct{ RefreshTokens RefreshTokenRepository; Outbox OutboxRepository; Processed ProcessedEventRepository; Now func() time.Time }`
  - `(Jobs) CleanupRefreshTokens(ctx) error`: deletes tokens that expired, or were revoked, more than 30 days ago.
  - `(Jobs) CleanupOutbox(ctx) error`: deletes published outbox rows older than 7 days and processed_events older than 30 days.
  - Default crons: `TokenCleanupCron = "0 3 * * *"`, `OutboxCleanupCron = "30 3 * * *"`. The Redis locker is used when Redis is enabled.

- [ ] **Step 1: Failing tests:** `TestCleanupRefreshTokens_Uses30DayCutoff`, `TestCleanupOutbox_Uses7And30DayCutoffs` (fixed `Now`, mocks assert the cutoff millis).
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(scheduler): cleanup jobs with optional distributed lock`.

### Task 26: Avatar upload to S3/MinIO

**Files:**
- Create: `internal/config/storage.go`, `internal/gateway/storage/minio_storage.go` (+ integration test), `internal/usecase/avatar_usecase.go` (+ `_test.go`), `internal/delivery/http/avatar_controller.go`, `test/avatar_test.go`

**Interfaces:**
- Produces:
  - Port `ObjectStorage interface{ Put(ctx, key string, r io.Reader, size int64, contentType string) error; Delete(ctx, key string) error; PresignGet(ctx, key string, ttl time.Duration) (string, error) }`. Presigning uses `Storage.PublicEndpoint` when it is set.
  - `AvatarUseCase.Upload(ctx, userID string, file io.Reader, size int64) (*model.UserResponse, error)`: sniffs the first 512 bytes with `http.DetectContentType`; only `image/jpeg`, `image/png` and `image/webp` are allowed, else `CodeUnsupportedMedia`; size > 2 MB → `CodePayloadTooLarge`. Key `avatars/{userID}/{uuid}.{jpg|png|webp}`. Stores the object, updates `avatar_key` in a transaction, and deletes the old object after commit (failure only logged).
  - `AvatarUseCase.Remove(ctx, userID string) error`
  - The user response converter sets `AvatarURL` via `PresignGet` (15m) when storage is enabled and the key is set.
  - Routes (only when storage is enabled): `PUT /api/v1/users/_current/avatar` (multipart field `file`, body limit 2 MB for this route), `DELETE /api/v1/users/_current/avatar`.

- [ ] **Step 1: Failing tests:** `TestUpload_RejectsTextRenamedPNG` (bytes `"hello"` with filename `a.png` → 415); `TestUpload_TooLarge413`; `TestUpload_ReplacesAndDeletesOld` (mock storage: `Delete(oldKey)` after commit); integration `TestAvatar_PresignedURLFetchable` (the MinIO container serves the object through the URL).
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(storage): avatar upload with content sniffing and presigned URLs`.

### Task 27: Email gateway and welcome email

**Files:**
- Create: `internal/gateway/email/mailer.go`, `smtp_mailer.go`, `log_mailer.go`, `templates/welcome.html` (embedded), `internal/usecase/notification_usecase.go` (+ `_test.go`)
- Modify: `config/worker.go` (register handler `consumerName="welcome-email"`, `eventType="user.registered"`)

**Interfaces:**
- Produces:
  - Port `Mailer interface{ Send(ctx context.Context, msg email.Message) error }`
  - `type Message struct{ To, Subject, HTMLBody, TextBody string }`
  - `func NewSMTPMailer(cfg config.Email) (*SMTPMailer, error)` (go-mail, STARTTLS when `TLS`), `NewLogMailer(log)`
  - `NotificationUseCase.SendWelcome(ctx, e model.Event) error`: decodes `{id, email, name}` and renders the template with `html/template` (HTML escaping).

- [ ] **Step 1: Failing tests:** `TestSendWelcome_EscapesName` (name `<script>` is escaped in the HTML body); integration `TestWelcomeEmail_SentOnceViaLocalDispatcher` (Kafka disabled: register → run one relay batch → Mailpit API shows exactly 1 message; process the same event again → still 1).
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `feat(email): SMTP mailer and idempotent welcome email`.

### Task 28: OpenAPI 3.1 and Swagger UI

**Files:**
- Create: `api/openapi.yaml`, `internal/delivery/http/docs/docs.go` (+ `_test.go`; `//go:embed` copy via `api/embed.go` exposing `api.OpenAPI []byte`)
- Delete: `api/api-spec.json`

**Interfaces:**
- Produces:
  - `func docs.Register(app *fiber.App, spec []byte)`: `GET /openapi.yaml` and `GET /docs` (an HTML page loading `swagger-ui-dist@5.17.14` from cdn.jsdelivr.net, pinned). The route sets a CSP that allows that CDN only.
  - Registered only when `cfg.Docs.Enabled` (validation already forbids it in production).
  - The spec documents every `/api/v1` route, the Bearer security scheme, the `Problem` schema and the `version` fields.

- [ ] **Step 1: Failing tests:** `TestDocs_ServesSpec` (200, contains `openapi: 3.1`); `TestDocs_NotRegisteredWhenDisabled` (404); `TestOpenAPI_CoversAllRoutes` (parse the yaml paths and compare them with `app.GetRoutes()` for `/api/v1`, normalizing `:param` ↔ `{param}`).
- [ ] **Step 2–4:** Run (fails) → implement → pass.
- [ ] **Step 5:** Commit `docs(api): OpenAPI 3.1 spec and Swagger UI`.

### Task 29: gRPC ContactService

**Files:**
- Create: `api/proto/contact/v1/contact.proto`, `buf.yaml`, `buf.gen.yaml`, `internal/delivery/grpc/pb/...` (generated, committed), `internal/delivery/grpc/server.go`, `interceptors.go`, `errors.go`, `contact_service.go` (+ `_test.go` using bufconn)
- Modify: `cmd/web/main.go` (listen when `GRPC.Enabled`, `GracefulStop` on shutdown), Makefile `proto`

**Interfaces:**
- Produces:
  - Proto package `contact.v1`, `go_package = "github.com/primestrider/golang-clean-architecture/internal/delivery/grpc/pb/contact/v1;contactv1"`.
  - `service ContactService { rpc CreateContact; rpc GetContact; rpc UpdateContact (includes version); rpc DeleteContact; rpc ListContacts (page, size, name, email, phone) }`
  - `func grpc.NewServer(cfg *config.Config, log *slog.Logger, issuer TokenIssuer, contacts *usecase.ContactUseCase, metrics *observability.Metrics) *grpc.Server`. Chained interceptors: recovery, request id, logging, auth (metadata `authorization: Bearer`; health and reflection exempt), otelgrpc stats handler, prometheus. Registers `grpc.health.v1`; reflection only when not production.
  - `func toStatus(err error) error` maps apperror codes to `codes.InvalidArgument`/`NotFound`/`Aborted` (Conflict)/`Unauthenticated`/`PermissionDenied`/`ResourceExhausted`/`Internal`, attaching field violations as `errdetails.BadRequest`.

- [ ] **Step 1: Failing tests (bufconn):** `TestGRPC_Unauthenticated`, `TestGRPC_CreateAndGetContact`, `TestGRPC_ValidationReturnsInvalidArgumentWithFieldViolations`, `TestGRPC_HealthNoAuth`.
- [ ] **Step 2–4:** `buf generate` → implement → pass.
- [ ] **Step 5:** Commit `feat(grpc): ContactService over the shared use cases`.

## Phase 7 — Wrap-up

### Task 30: README, manual HTTP file, final verification

**Files:**
- Modify: `README.md` (Indonesian), `test/manual.http`, `test/http-client.env.json`, `architecture.png` description text in the README only (the image stays unchanged)

- [ ] **Step 1:** README sections: architecture and request flow (updated for the outbox), folder map, minimal vs full mode, running without `make` (raw PowerShell commands), config & env vars table, security model (tokens, rotation, rate limits, known trade-off of non-revocable access tokens), events (outbox, ordering, at-least-once, DLQ, parked events and how to reset them), observability, testing, migrations (warning: never edit applied migrations in a live system), adding a new feature (step-by-step checklist).
- [ ] **Step 2:** Update `manual.http` for the auth flow, contacts with version, avatar and admin.
- [ ] **Step 3: Final verification:** `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test -race ./internal/...`, `go test -tags integration ./...`, `govulncheck ./...`, `docker compose --profile full up -d` with a smoke test of register → login → create contact → the welcome email appears in Mailpit and a trace appears in Jaeger.
- [ ] **Step 4:** Spot-check documentation: `go doc ./internal/usecase` shows Indonesian docs for every exported symbol. `revive` `exported` passes.
- [ ] **Step 5:** Commit `docs: README and manual requests for the production-ready template`.
