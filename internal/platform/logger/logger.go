package logger

import (
	"log/slog"
	"os"
	"strings"

	"japan_data_project/internal/platform/config"
)

func New(cfg config.Config) *slog.Logger {
	level := parseLevel(cfg.App.LogLevel)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.EqualFold(cfg.App.LogFormat, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With(
		slog.String("app", cfg.App.Name),
		slog.String("env", cfg.App.Env),
	)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(v string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
