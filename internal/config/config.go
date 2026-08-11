// Package config loads the runtime configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Environment names understood by the service.
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Config is the fully resolved configuration of the process.
type Config struct {
	AppName   string `env:"APP_NAME" envDefault:"RTU API"`
	AppEnv    string `env:"APP_ENV" envDefault:"development"`
	APIPrefix string `env:"API_PREFIX" envDefault:"/api/rtu/v1"`

	Host            string        `env:"HOST" envDefault:"0.0.0.0"`
	Port            int           `env:"PORT" envDefault:"5020"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"90s"`
	RequestTimeout  time.Duration `env:"HTTP_REQUEST_TIMEOUT" envDefault:"20s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"20s"`
	MaxBodyBytes    int64         `env:"HTTP_MAX_BODY_BYTES" envDefault:"10485760"`

	AWSRegion          string        `env:"AWS_REGION" envDefault:"ap-southeast-1"`
	S3Bucket           string        `env:"S3_BUCKET"`
	S3AppPrefix        string        `env:"S3_APP_PREFIX" envDefault:"mwa"`
	AWSAccessKeyID     string        `env:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string        `env:"AWS_SECRET_ACCESS_KEY"`
	S3SignedURLTTL     time.Duration `env:"S3_SIGNED_URL_TTL" envDefault:"24h"`

	DatabaseURL string `env:"DATABASE_URL"`
	DBHost      string `env:"DB_HOST"`
	DBPort      int    `env:"DB_PORT" envDefault:"5432"`
	DBUser      string `env:"DB_USER"`
	DBPassword  string `env:"DB_PASSWORD"`
	DBName      string `env:"DB_NAME"`
	DBSSLMode   string `env:"DB_SSLMODE"`

	DBMaxConns          int32         `env:"DB_MAX_CONNS" envDefault:"20"`
	DBMinConns          int32         `env:"DB_MIN_CONNS" envDefault:"2"`
	DBMaxConnLifetime   time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
	DBMaxConnIdleTime   time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"30m"`
	DBHealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"1m"`
	DBConnectTimeout    time.Duration `env:"DB_CONNECT_TIMEOUT" envDefault:"10s"`
	DBStatementCache    bool          `env:"DB_STATEMENT_CACHE" envDefault:"true"`

	// SchemaGuard rejects traffic with E500_004 while migrations are pending.
	SchemaGuard bool `env:"SCHEMA_GUARD" envDefault:"true"`

	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"*"`
	CORSAllowedHeaders []string `env:"CORS_ALLOWED_HEADERS" envSeparator:"," envDefault:"Accept,Authorization,Content-Type,X-Request-Id"`

	RateLimitEnabled  bool          `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
	RateLimitRequests int           `env:"RATE_LIMIT_REQUESTS" envDefault:"300"`
	RateLimitWindow   time.Duration `env:"RATE_LIMIT_WINDOW" envDefault:"1m"`

	// AuthEnabled gates every route under APIPrefix. Disabled by default in
	// development so local smoke tests stay frictionless; production startup
	// rejects AUTH_ENABLED=false.
	AuthEnabled   bool   `env:"AUTH_ENABLED" envDefault:"false"`
	AuthJWTSecret string `env:"AUTH_JWT_SECRET"`
	AuthJWTIssuer string `env:"AUTH_JWT_ISSUER"`

	MetricsEnabled bool `env:"METRICS_ENABLED" envDefault:"true"`

	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
}

// Load reads .env (when present) and then the process environment.
func Load() (*Config, error) {
	// Real environment variables always win over the file.
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse environment: %w", err)
	}

	cfg.AppEnv = strings.ToLower(cfg.AppEnv)
	switch cfg.AppEnv {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		return nil, fmt.Errorf("APP_ENV must be one of development, staging, production (got %q)", cfg.AppEnv)
	}

	cfg.APIPrefix = "/" + strings.Trim(cfg.APIPrefix, "/")
	if cfg.DBMinConns > cfg.DBMaxConns {
		return nil, fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", cfg.DBMinConns, cfg.DBMaxConns)
	}

	if err := cfg.resolveDatabaseURL(); err != nil {
		return nil, err
	}

	if err := cfg.validateProduction(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validateProduction() error {
	if !c.IsProduction() {
		if c.AuthEnabled && c.AuthJWTSecret == "" {
			return fmt.Errorf("AUTH_JWT_SECRET is required when AUTH_ENABLED=true")
		}
		return nil
	}

	if !c.AuthEnabled {
		return fmt.Errorf("AUTH_ENABLED must be true in production")
	}
	if c.AuthJWTSecret == "" {
		return fmt.Errorf("AUTH_JWT_SECRET is required in production")
	}
	if len(c.AuthJWTSecret) < 32 {
		return fmt.Errorf("AUTH_JWT_SECRET must be at least 32 characters in production")
	}
	for _, origin := range c.CORSAllowedOrigins {
		if origin == "*" {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must not contain '*' in production")
		}
	}
	if strings.Contains(c.DatabaseURL, "sslmode=disable") || c.DBSSLMode == "disable" {
		return fmt.Errorf("DB_SSLMODE must not be disable in production")
	}
	return nil
}

// databaseSSLMode returns the sslmode query parameter from DatabaseURL when
// present, otherwise the configured DB_SSLMODE field.
func (c *Config) databaseSSLMode() string {
	if c.DBSSLMode != "" {
		return c.DBSSLMode
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("sslmode")
}

// Addr is the listen address of the HTTP server.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// IsProduction reports whether the service runs with production semantics.
func (c *Config) IsProduction() bool { return c.AppEnv == EnvProduction }

// IsStaging reports whether the staging guard should be active.
func (c *Config) IsStaging() bool { return c.AppEnv == EnvStaging }

// S3Configured reports whether panel image uploads can reach S3.
func (c *Config) S3Configured() bool { return strings.TrimSpace(c.S3Bucket) != "" }

// Logger builds the structured logger described by LOG_LEVEL / LOG_FORMAT.
func (c *Config) Logger() *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(c.LogFormat, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With("service", c.AppName, "env", c.AppEnv)
}
