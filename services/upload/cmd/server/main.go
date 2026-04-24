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
	cfg.Server.Port = getEnvOr("PORT", "8084")

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-upload",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
		// Uploads can be very large; BodyLimit is handled by tusd, not Fiber.
		BodyLimit: -1,
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "upload"})
	})

	// Tus resumable upload endpoint — tusd handler will be mounted here.
	// All methods (POST, PATCH, HEAD, OPTIONS, DELETE) are handled by tusd.
	upload := app.Group("/api/v1/uploads")
	upload.All("/", handleTusUpload)
	upload.All("/:id", handleTusUpload)
	upload.All("/:id/*", handleTusUpload)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("upload service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down upload service")
	if err := app.Shutdown(); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

// handleTusUpload — placeholder; replaced by tusd.Handler in Phase 1.
func handleTusUpload(c fiber.Ctx) error { return fiber.ErrNotImplemented }

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
