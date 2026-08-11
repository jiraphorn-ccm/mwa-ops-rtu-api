package config_test

import (
	"testing"

	"github.com/rtu-api/internal/config"
)

func setDBEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "rtu")
	t.Setenv("DB_PASSWORD", "rtu_password")
	t.Setenv("DB_NAME", "rtu")
	t.Setenv("DB_SSLMODE", "disable")
}

func setProductionDBEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "rtu")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "rtu")
	t.Setenv("DB_SSLMODE", "require")
}

func TestLoadProductionRejectsInsecureDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	setProductionDBEnv(t)
	t.Setenv("AUTH_ENABLED", "false")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected production without auth to fail")
	}
}

func TestLoadProductionRequiresJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	setProductionDBEnv(t)
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_JWT_SECRET", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected missing JWT secret to fail in production")
	}
}

func TestLoadProductionRejectsWildcardCORS(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	setProductionDBEnv(t)
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected wildcard CORS to fail in production")
	}
}

func TestLoadProductionRejectsDisableSSL(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	setProductionDBEnv(t)
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected disable sslmode to fail in production")
	}
}

func TestLoadDevelopmentFromDBFields(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	setDBEnv(t)
	t.Setenv("AUTH_ENABLED", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("development config should load: %v", err)
	}
	if cfg.AuthEnabled {
		t.Fatal("auth should default off in development")
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected DatabaseURL to be built from DB_* fields")
	}
}

func TestLoadAcceptsDatabaseURLOverride(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://u:p@db.local:5432/rtu?sslmode=disable")
	t.Setenv("DB_HOST", "ignored")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@db.local:5432/rtu?sslmode=disable" {
		t.Fatalf("got %q", cfg.DatabaseURL)
	}
}

func TestIsProduction(t *testing.T) {
	cfg := &config.Config{AppEnv: config.EnvProduction}
	if !cfg.IsProduction() {
		t.Fatal("expected production")
	}
}
