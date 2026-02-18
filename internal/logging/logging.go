package logging

import (
	"log/slog"
	"os"
	"strings"
)

func Init() {
	setLogFormat()
	setLogLevel()
}

func setLogFormat() {
	format := os.Getenv("LOG_FORMAT")

	switch strings.ToLower(format) {
	case "text", "":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	case "json":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	default:
		slog.Warn("unknown log format, using TEXT")
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	}
}

func setLogLevel() {
	level := os.Getenv("LOG_LEVEL")

	switch strings.ToLower(level) {
	case "debug":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	case "info", "":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "warn":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "error":
		slog.SetLogLoggerLevel(slog.LevelError)
	default:
		slog.Warn("unknown log level, using INFO")
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}
}
