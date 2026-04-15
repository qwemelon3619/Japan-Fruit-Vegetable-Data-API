package tests

import (
	"testing"

	"japan_data_project/internal/platform/config"
)

func TestConfigLoad_UsesEnvOverrides(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/this-file-does-not-exist.env")
	t.Setenv("APP_NAME", "unit-test-app")
	t.Setenv("APP_ENV", "test")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_PORT", "19090")
	t.Setenv("POSTGRES_HOST", "db.internal")
	t.Setenv("POSTGRES_PORT", "6543")
	t.Setenv("POSTGRES_USER", "tester")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "analytics_test")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("POSTGRES_TIMEZONE", "UTC")

	cfg := config.Load()

	if cfg.App.Name != "unit-test-app" {
		t.Fatalf("APP_NAME not applied: got=%q", cfg.App.Name)
	}
	if cfg.App.Env != "test" {
		t.Fatalf("APP_ENV not applied: got=%q", cfg.App.Env)
	}
	if cfg.App.LogLevel != "debug" {
		t.Fatalf("LOG_LEVEL not applied: got=%q", cfg.App.LogLevel)
	}
	if cfg.App.LogFormat != "json" {
		t.Fatalf("LOG_FORMAT not applied: got=%q", cfg.App.LogFormat)
	}
	if cfg.App.HTTPPort != 19090 {
		t.Fatalf("HTTP_PORT not applied: got=%d", cfg.App.HTTPPort)
	}
	if cfg.Database.Host != "db.internal" {
		t.Fatalf("POSTGRES_HOST not applied: got=%q", cfg.Database.Host)
	}
	if cfg.Database.Port != 6543 {
		t.Fatalf("POSTGRES_PORT not applied: got=%d", cfg.Database.Port)
	}
	if cfg.Database.User != "tester" {
		t.Fatalf("POSTGRES_USER not applied: got=%q", cfg.Database.User)
	}
	if cfg.Database.Password != "secret" {
		t.Fatalf("POSTGRES_PASSWORD not applied: got=%q", cfg.Database.Password)
	}
	if cfg.Database.DBName != "analytics_test" {
		t.Fatalf("POSTGRES_DB not applied: got=%q", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "require" {
		t.Fatalf("POSTGRES_SSLMODE not applied: got=%q", cfg.Database.SSLMode)
	}
	if cfg.Database.TimeZone != "UTC" {
		t.Fatalf("POSTGRES_TIMEZONE not applied: got=%q", cfg.Database.TimeZone)
	}
}

func TestConfigLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/this-file-does-not-exist.env")
	t.Setenv("HTTP_PORT", "not-a-number")
	t.Setenv("POSTGRES_PORT", "bad")

	cfg := config.Load()
	if cfg.App.HTTPPort != 8080 {
		t.Fatalf("invalid HTTP_PORT must fallback to 8080, got=%d", cfg.App.HTTPPort)
	}
	if cfg.Database.Port != 5432 {
		t.Fatalf("invalid POSTGRES_PORT must fallback to 5432, got=%d", cfg.Database.Port)
	}
}

func TestConfigLoad_EmptyValuesUseDefaults(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/this-file-does-not-exist.env")
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("POSTGRES_PORT", "")

	cfg := config.Load()

	if cfg.App.Name != "japan-data-platform" {
		t.Fatalf("empty APP_NAME must fallback: got=%q", cfg.App.Name)
	}
	if cfg.App.Env != "local" {
		t.Fatalf("empty APP_ENV must fallback: got=%q", cfg.App.Env)
	}
	if cfg.App.HTTPPort != 8080 {
		t.Fatalf("empty HTTP_PORT must fallback to 8080, got=%d", cfg.App.HTTPPort)
	}
	if cfg.Database.Host != "localhost" {
		t.Fatalf("empty POSTGRES_HOST must fallback: got=%q", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Fatalf("empty POSTGRES_PORT must fallback to 5432, got=%d", cfg.Database.Port)
	}
}
