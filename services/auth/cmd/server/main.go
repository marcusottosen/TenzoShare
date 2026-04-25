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

	"github.com/tenzoshare/tenzoshare/services/auth/internal/handlers"
	"github.com/tenzoshare/tenzoshare/services/auth/internal/repository"
	"github.com/tenzoshare/tenzoshare/services/auth/internal/service"
	"github.com/tenzoshare/tenzoshare/shared/pkg/cache"
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

	// Redis — used for IP rate limiting; non-fatal if unavailable at startup
	cacheClient, err := cache.New(cfg.Redis)
	if err != nil {
		log.Warn("redis unavailable — rate limiting disabled", zap.Error(err))
		cacheClient = nil
	}

	// NATS JetStream — used for audit event publishing; non-fatal if unavailable
	jsClient, err := jetstream.New(cfg.NATS.URL)
	if err != nil {
		log.Warn("nats unavailable — audit publishing disabled", zap.Error(err))
		jsClient = nil
	} else {
		if err := jsClient.EnsureStreams(ctx); err != nil {
			log.Warn("failed to ensure NATS streams", zap.Error(err))
		}
		defer jsClient.Close()
	}

	repo := repository.NewUserRepository(pool)
	svc := service.New(repo, cfg, cacheClient, jsClient, log)

	bootstrapAdminEmail := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	bootstrapAdminPassword := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if bootstrapAdminEmail != "" && bootstrapAdminPassword != "" {
		if err := svc.EnsureBootstrapAdmin(ctx, bootstrapAdminEmail, bootstrapAdminPassword); err != nil {
			log.Fatal("failed to ensure bootstrap admin", zap.Error(err))
		}
	} else {
		log.Warn("bootstrap admin not configured; set BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD")
	}

	h := handlers.New(svc)

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-auth",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "auth"})
	})

	v1 := app.Group("/api/v1/auth")
	v1.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "auth"})
	})

	// public
	v1.Post("/register", h.Register)
	v1.Post("/login", h.Login)
	v1.Post("/login/mfa", h.LoginWithMFA)
	v1.Post("/refresh", h.Refresh)
	v1.Post("/password-reset/request", h.PasswordResetRequest)
	v1.Post("/password-reset/confirm", h.PasswordResetConfirm)

	// authenticated
	protected := v1.Group("", middleware.JWTAuth(cfg.JWT.Secret))
	protected.Post("/logout", h.Logout)
	protected.Get("/me", h.Me)
	protected.Post("/mfa/setup", h.MFASetup)
	protected.Post("/mfa/verify", h.MFAVerify)

	go func() {
		log.Info("auth service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down auth service")
	if err := app.Shutdown(); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}
