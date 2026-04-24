package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("failed to load config: %v", err)
	}
	cfg.Server.Port = getEnvOr("PORT", "8082")

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-transfer",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "transfer"})
	})

	v1 := app.Group("/api/v1")
	transfers := v1.Group("/transfers")
	transfers.Post("/", handleCreateTransfer)
	transfers.Get("/:id", handleGetTransfer)
	transfers.Get("/", handleListTransfers)
	transfers.Delete("/:id", handleRevokeTransfer)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("transfer service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down transfer service")
	if err := app.Shutdown(); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

func handleCreateTransfer(c fiber.Ctx) error { return fiber.ErrNotImplemented }
func handleGetTransfer(c fiber.Ctx) error    { return fiber.ErrNotImplemented }
func handleListTransfers(c fiber.Ctx) error  { return fiber.ErrNotImplemented }
func handleRevokeTransfer(c fiber.Ctx) error { return fiber.ErrNotImplemented }

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
