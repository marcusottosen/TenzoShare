// Package logger provides a shared structured logger backed by uber-go/zap.
// In dev mode it uses zap's human-readable development config;
// in production it uses JSON output for log aggregation (Loki).
// Set LOG_FORMAT=json to force JSON output regardless of devMode (e.g. Docker + DEV_MODE=true).
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a configured *zap.Logger.
// level is a zap level string: "debug", "info", "warn", "error".
// devMode selects between a human-readable development encoder and JSON production output.
// The LOG_FORMAT=json environment variable overrides devMode to always emit JSON,
// which is useful when running in Docker with DEV_MODE=true but Loki log aggregation active.
func New(level string, devMode bool) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	// LOG_FORMAT=json forces JSON output even in dev mode so that Promtail/Loki
	// can parse structured fields (level, msg, caller, user_id, request_id, etc.).
	forceJSON := os.Getenv("LOG_FORMAT") == "json"

	if devMode && !forceJSON {
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapLevel)
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return cfg.Build()
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	return cfg.Build()
}
