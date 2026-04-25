package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/storage/internal/backends"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/handlers"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/database"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("failed to load config: %v", err)
	}
	cfg.Server.Port = getEnvOr("PORT", "8083")

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, database.DefaultConfig(cfg.Database.DSN))
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	minioBackend, err := backends.NewMinIO(ctx, cfg)
	if err != nil {
		log.Fatal("failed to connect to object storage", zap.Error(err))
	}

	repo := repository.NewFileRepository(pool)
	h := handlers.New(repo, minioBackend)

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-storage",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "storage"})
	})
	app.Get("/api/v1/files/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "storage"})
	})

	auth := middleware.JWTAuth(cfg.JWT.Secret)
	v1 := app.Group("/api/v1/files", auth)
	v1.Post("/", h.Upload)
	v1.Get("/", h.ListFiles)
	v1.Get("/:id", h.GetFile)
	v1.Delete("/:id", h.DeleteFile)
	v1.Get("/:id/presign", h.PresignURL)

	go func() {
		log.Info("storage service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down storage service")
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
