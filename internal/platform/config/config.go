package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
}

type AppConfig struct {
	Name      string
	Env       string
	LogLevel  string
	LogFormat string
	HTTPPort  int
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	TimeZone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

var loadOnce sync.Once

func Load() Config {
	loadOnce.Do(func() {
		loadDotEnv()
	})

	return Config{
		App: AppConfig{
			Name:      envOrDefault("APP_NAME", "japan-data-platform"),
			Env:       envOrDefault("APP_ENV", "local"),
			LogLevel:  envOrDefault("LOG_LEVEL", "info"),
			LogFormat: envOrDefault("LOG_FORMAT", "text"),
			HTTPPort:  envIntOrDefault("HTTP_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:            envOrDefault("POSTGRES_HOST", "localhost"),
			Port:            envIntOrDefault("POSTGRES_PORT", 5432),
			User:            envOrDefault("POSTGRES_USER", "postgres"),
			Password:        envOrDefault("POSTGRES_PASSWORD", ""),
			DBName:          envOrDefault("POSTGRES_DB", "analyticsdb"),
			SSLMode:         envOrDefault("POSTGRES_SSLMODE", "disable"),
			TimeZone:        envOrDefault("POSTGRES_TIMEZONE", "Asia/Tokyo"),
			MaxOpenConns:    envIntOrDefault("POSTGRES_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    envIntOrDefault("POSTGRES_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDurationOrDefault("POSTGRES_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: envDurationOrDefault("POSTGRES_CONN_MAX_IDLE_TIME", 10*time.Minute),
		},
	}
}

func loadDotEnv() {
	// Highest priority: explicit env file path.
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		_ = godotenv.Load(envFile)
		return
	}

	// Common case: run from project root.
	if err := godotenv.Load(".env"); err == nil {
		return
	}

	// Fallback: search parent directories for go.mod and sibling .env.
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			_ = godotenv.Load(filepath.Join(dir, ".env"))
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
