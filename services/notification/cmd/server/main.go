package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/notification/internal/consumer"
	"github.com/tenzoshare/tenzoshare/services/notification/internal/email"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
	"github.com/tenzoshare/tenzoshare/shared/pkg/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("failed to load config: %v", err)
	}
	cfg.Server.Port = getEnvOr("PORT", "8085")

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// NATS JetStream
	jsClient, err := jetstream.New(cfg.NATS.URL)
	if err != nil {
		log.Fatal("failed to connect to NATS", zap.Error(err))
	}
	defer jsClient.Close()

	if err := jsClient.EnsureStreams(ctx); err != nil {
		log.Warn("failed to ensure NATS streams", zap.Error(err))
	}

	// Email sender
	sender := email.New(cfg.SMTP, log)

	// NATS consumer — runs in background goroutine
	cons := consumer.New(jsClient, sender, log)
	go func() {
		if err := cons.Start(ctx); err != nil {
			log.Error("notification consumer exited with error", zap.Error(err))
		}
	}()

	// HTTP server — health only
	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-notification",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	telemetry.Register(app, "notification")
	app.Use(middleware.RequestLogger(log))
	app.Get("/api/v1/notification/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "notification"})
	})

	go func() {
		log.Info("notification service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down notification service")
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
