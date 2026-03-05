package logging

import (
	"log/slog"
	"os"
	"strings"
)

func Init() {
	level := getLogLevel()
	setLogFormat(level)
}

func getLogLevel() slog.Level {
	level := os.Getenv("LOG_LEVEL")

	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setLogFormat(level slog.Level) {
	format := os.Getenv("LOG_FORMAT")
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch strings.ToLower(format) {
	case "text", "":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
	case "json":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, opts)))
	default:
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
	}
}
