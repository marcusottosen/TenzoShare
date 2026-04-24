// Package logger provides a shared structured logger backed by uber-go/zap.
// In dev mode it uses zap's human-readable development config;
// in production it uses JSON output for log aggregation (Loki).
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a configured *zap.Logger.
// level is a zap level string: "debug", "info", "warn", "error".
// devMode selects between a human-readable development encoder and JSON production output.
func New(level string, devMode bool) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	if devMode {
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapLevel)
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return cfg.Build()
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	return cfg.Build()
}
