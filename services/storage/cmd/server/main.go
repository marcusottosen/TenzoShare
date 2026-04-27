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

	"github.com/tenzoshare/tenzoshare/services/storage/internal/backends"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/handlers"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/repository"
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

	// ── NATS JetStream (optional — graceful degradation if unavailable) ────────
	var js *jetstream.Client
	if cfg.NATS.URL != "" {
		js, err = jetstream.New(cfg.NATS.URL)
		if err != nil {
			log.Warn("failed to connect to NATS — audit events will not be published", zap.Error(err))
		} else {
			if err2 := js.EnsureStreams(ctx); err2 != nil {
				log.Warn("failed to ensure NATS streams", zap.Error(err2))
			}
			log.Info("connected to NATS JetStream")
		}
	}

	h := handlers.New(repo, minioBackend, js, log, cfg.App.EncryptionKey)

	pubKey, err := jwtkeys.ParsePublicKey(cfg.JWT.PublicKeyPEM)
	if err != nil {
		log.Fatal("failed to parse JWT public key", zap.Error(err))
	}

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-storage",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "storage"})
	})
	app.Get("/api/v1/files/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "storage"})
	})

	auth := middleware.JWTAuth(pubKey)
	v1 := app.Group("/api/v1/files") // no group-level middleware — Fiber's Group.Use applies prefix-wide
	v1.Post("/", auth, h.Upload)
	v1.Get("/usage", auth, h.GetMyUsage)
	v1.Get("/", auth, h.ListFiles)
	v1.Get("/:id", auth, h.GetFile)
	v1.Delete("/:id", auth, h.DeleteFile)
	v1.Get("/:id/presign", auth, h.PresignURL)
	// Download accepts either a Bearer RS256 JWT (authenticated users/services)
	// or a ?token= HS256 download token issued by PresignURL (browser-navigable).
	v1.Get("/:id/download", middleware.OptionalJWTAuth(pubKey), h.Download)

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
