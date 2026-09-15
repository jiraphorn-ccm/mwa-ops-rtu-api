package config_test

import (
	"testing"
	"time"

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
	t.Setenv("JWT_SECRET", "")

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
		t.Fatal("AUTH_ENABLED=false should disable the flag")
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected DatabaseURL to be built from DB_* fields")
	}
}

func TestLoadAcceptsDatabaseURLOverride(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_ENABLED", "false")
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

func TestLoadJWTSecretAlias(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	setDBEnv(t)
	t.Setenv("AUTH_JWT_SECRET", "")
	t.Setenv("JWT_SECRET", "alias-secret-value-at-least-32-chars!!")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AuthJWTSecret != "alias-secret-value-at-least-32-chars!!" {
		t.Fatalf("AuthJWTSecret=%q", cfg.AuthJWTSecret)
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Fatalf("JWTAccessTTL=%s", cfg.JWTAccessTTL)
	}
}

func TestIsProduction(t *testing.T) {
	cfg := &config.Config{AppEnv: config.EnvProduction}
	if !cfg.IsProduction() {
		t.Fatal("expected production")
	}
}
