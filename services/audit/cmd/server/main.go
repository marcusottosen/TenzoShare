package main

import (
"context"
stdlog "log"
"os"
"os/signal"
"syscall"

"github.com/gofiber/fiber/v3"
"go.uber.org/zap"

"github.com/tenzoshare/tenzoshare/services/audit/internal/consumer"
"github.com/tenzoshare/tenzoshare/services/audit/internal/handlers"
"github.com/tenzoshare/tenzoshare/services/audit/internal/repository"
"github.com/tenzoshare/tenzoshare/shared/pkg/config"
"github.com/tenzoshare/tenzoshare/shared/pkg/database"
"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

func main() {
cfg, err := config.Load()
if err != nil {
stdlog.Fatalf("failed to load config: %v", err)
}
cfg.Server.Port = getEnvOr("PORT", "8086")

log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
if err != nil {
stdlog.Fatalf("failed to initialize logger: %v", err)
}
defer log.Sync() //nolint:errcheck

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

// Database
pool, err := database.Connect(ctx, database.DefaultConfig(cfg.Database.DSN))
if err != nil {
log.Fatal("failed to connect to database", zap.Error(err))
}
defer pool.Close()

// NATS JetStream
jsClient, err := jetstream.New(cfg.NATS.URL)
if err != nil {
log.Fatal("failed to connect to NATS", zap.Error(err))
}
defer jsClient.Close()

if err := jsClient.EnsureStreams(ctx); err != nil {
log.Warn("failed to ensure NATS streams", zap.Error(err))
}

// Repository + consumer + handler
repo := repository.New(pool)
cons := consumer.New(jsClient, repo, log)
h := handlers.New(repo)

// NATS consumer in background
go func() {
if err := cons.Start(ctx); err != nil {
log.Error("audit consumer exited with error", zap.Error(err))
}
}()

// HTTP server
app := fiber.New(fiber.Config{
AppName:      "tenzoshare-audit",
ReadTimeout:  cfg.Server.ReadTimeout,
WriteTimeout: cfg.Server.WriteTimeout,
ErrorHandler: middleware.ErrorHandler,
})

app.Get("/health", func(c fiber.Ctx) error {
return c.JSON(fiber.Map{"status": "ok", "service": "audit"})
})

v1 := app.Group("/api/v1")
audit := v1.Group("/audit")
audit.Get("/health", func(c fiber.Ctx) error {
return c.JSON(fiber.Map{"status": "ok", "service": "audit"})
})

// Authenticated routes
protected := audit.Group("", middleware.JWTAuth(cfg.JWT.Secret))
protected.Get("/events", h.ListEvents)

go func() {
log.Info("audit service starting", zap.String("port", cfg.Server.Port))
if err := app.Listen(":" + cfg.Server.Port); err != nil {
log.Error("server error", zap.Error(err))
}
}()

<-ctx.Done()
log.Info("shutting down audit service")
if err := app.Shutdown(); err != nil {
log.Error("shutdown error", zap.Error(err))
}
}

func getEnvOr(key, fallback string) string {
if v := os.Getenv(key); v != "" {
return v
}
return fallback
}
