package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/handlers"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/repository"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/service"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/database"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jwtkeys"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, database.DefaultConfig(cfg.Database.DSN))
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	repo := repository.NewTransferRepository(pool)

	// NATS JetStream — non-fatal if unavailable
	jsClient, err := jetstream.New(cfg.NATS.URL)
	if err != nil {
		log.Warn("nats unavailable — event publishing disabled", zap.Error(err))
		jsClient = nil
	} else {
		if err := jsClient.EnsureStreams(ctx); err != nil {
			log.Warn("failed to ensure NATS streams", zap.Error(err))
		}
		defer jsClient.Close()
	}

	svc := service.New(repo, cfg, jsClient, log)
	h := handlers.New(svc, cfg.JWT.PrivateKeyPEM, getEnvOr("STORAGE_SERVICE_URL", "http://tenzoshare-storage:8083"))

	pubKey, err := jwtkeys.ParsePublicKey(cfg.JWT.PublicKeyPEM)
	if err != nil {
		log.Fatal("failed to parse JWT public key", zap.Error(err))
	}

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-transfer",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "transfer"})
	})

	// Public: access a transfer by slug (downloaders, no auth required)
	app.Get("/api/v1/t/:slug", h.Access)
	app.Get("/api/v1/t/:slug/files/:fileId/download", h.DownloadURL)
	app.Get("/api/v1/transfers/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "transfer"})
	})

	auth := middleware.JWTAuth(pubKey)
	v1 := app.Group("/api/v1/transfers", auth)
	v1.Post("/", h.Create)
	v1.Get("/", h.List)
	v1.Get("/:id", h.Get)
	v1.Delete("/:id", h.Revoke)

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

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
