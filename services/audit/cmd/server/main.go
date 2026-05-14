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

	"github.com/tenzoshare/tenzoshare/services/audit/internal/consumer"
	"github.com/tenzoshare/tenzoshare/services/audit/internal/handlers"
	"github.com/tenzoshare/tenzoshare/services/audit/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/cache"
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

	// Redis — used for JWT revocation checks; non-fatal if unavailable
	cacheClient, err := cache.New(cfg.Redis)
	if err != nil {
		log.Warn("redis unavailable — token revocation disabled", zap.Error(err))
		cacheClient = nil
	}

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
		// ProxyHeader + TrustProxy tell Fiber to read c.IP() from X-Real-IP when
		// the connection arrives from a trusted private-network proxy (Traefik).
		ProxyHeader:      "X-Real-IP",
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Private: true},
	})

	pubKey, err := jwtkeys.ParsePublicKey(cfg.JWT.PublicKeyPEM)
	if err != nil {
		log.Fatal("failed to parse JWT public key", zap.Error(err))
	}

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders(cfg.App.DevMode))
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))
	app.Use(middleware.RequestLogger(log))

	telemetry.Register(app, "audit")

	v1 := app.Group("/api/v1")
	audit := v1.Group("/audit")
	audit.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "audit"})
	})

	// Revocation check — rejects tokens whose JTI is in the Redis blacklist.
	// If Redis is unavailable, cacheClient is nil and TokenRevocation passes all requests.
	var revocationCheck fiber.Handler
	if cacheClient != nil {
		revocationCheck = middleware.TokenRevocation(cacheClient.IsTokenRevoked)
	} else {
		revocationCheck = middleware.TokenRevocation(nil)
	}

	// Authenticated routes
	protected := audit.Group("", middleware.JWTAuth(pubKey), revocationCheck)
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
