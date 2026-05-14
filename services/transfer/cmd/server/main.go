package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	"github.com/tenzoshare/tenzoshare/shared/pkg/telemetry"
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

	repo := repository.NewTransferRepository(pool, log)
	requestRepo := repository.NewRequestRepository(pool)

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
	storageURL := getEnvOr("STORAGE_SERVICE_URL", "http://tenzoshare-storage:8083")
	requestSvc := service.NewRequestService(requestRepo, cfg, jsClient, log, storageURL)
	h, err := handlers.New(svc, requestSvc, cfg.JWT.PrivateKeyPEM, storageURL)
	if err != nil {
		log.Fatal("failed to initialise transfer handler", zap.Error(err))
	}

	pubKey, err := jwtkeys.ParsePublicKey(cfg.JWT.PublicKeyPEM)
	if err != nil {
		log.Fatal("failed to parse JWT public key", zap.Error(err))
	}

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-transfer",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
		// ProxyHeader + TrustProxy tell Fiber to read c.IP() from X-Real-IP when
		// the connection arrives from a trusted private-network proxy (Traefik).
		ProxyHeader:      "X-Real-IP",
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Private: true},
	})

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders(cfg.App.DevMode))
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))
	app.Use(middleware.RequestLogger(log))

	telemetry.Register(app, "transfer")

	// Public: access a transfer by slug (downloaders, no auth required).
	// POST is used so the optional password is sent in the JSON body rather than
	// a URL query parameter (which would appear in logs, history, and Referer headers).
	app.Post("/api/v1/t/:slug", h.Access)
	app.Post("/api/v1/t/:slug/files/:fileId/download", h.DownloadURL)
	app.Get("/api/v1/transfers/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "transfer"})
	})

	auth := middleware.JWTAuth(pubKey)
	v1 := app.Group("/api/v1/transfers", auth)
	v1.Post("/", h.Create)
	v1.Get("/", h.List)
	v1.Get("/:id", h.Get)
	v1.Get("/:id/recipients", h.ListRecipients)
	v1.Delete("/:id", h.Revoke)

	// File request endpoints (auth required — owner manages requests)
	requests := app.Group("/api/v1/requests", auth)
	requests.Post("/", h.CreateFileRequest)
	requests.Get("/", h.ListFileRequests)
	requests.Get("/:id", h.GetFileRequest)
	requests.Delete("/:id", h.DeactivateFileRequest)

	// Public file request endpoints (no auth — guests view and upload)
	app.Get("/api/v1/r/:slug", h.GetPublicFileRequest)
	app.Post("/api/v1/r/:slug/upload", h.UploadToRequest)

	// Background goroutine: expire stale transfers every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := repo.ExpireStale(ctx)
				if err != nil {
					log.Warn("expire stale transfers", zap.Error(err))
				} else if n > 0 {
					log.Info("expired stale transfers", zap.Int64("count", n))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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
