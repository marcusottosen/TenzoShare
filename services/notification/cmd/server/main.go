package main

import (
	"context"
	"encoding/json"
	stdlog "log"
	"os"
	"os/signal"
	"strings"
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

	// Branding fetcher — reads from admin service, cached 5 min.
	// Non-fatal: defaults are used when admin is unreachable.
	adminURL := getEnvOr("ADMIN_SERVICE_URL", "http://tenzoshare-admin:8087")
	branding := email.NewBrandingFetcher(adminURL, log)
	go branding.Get() // warm up cache in background

	// Email sender
	sender := email.New(cfg.SMTP, log, branding)

	// NATS consumer — runs in background goroutine
	authServiceURL := getEnvOr("AUTH_SERVICE_URL", "http://tenzoshare-auth:8081")
	cons := consumer.New(jsClient, sender, log, cfg.App.Pepper, cfg.App.BaseURL, authServiceURL, adminURL)
	go func() {
		if err := cons.Start(ctx); err != nil {
			log.Error("notification consumer exited with error", zap.Error(err))
		}
	}()

	// CONFIG.smtp subscriber — receives live SMTP config updates from the admin service.
	// Uses DeliverLastPolicy so the sender is updated immediately on (re)start if the
	// admin has previously saved SMTP settings.
	go func() {
		err := jsClient.SubscribeLast(ctx, "CONFIG", "notification-smtp-config", "CONFIG.smtp",
			func(_ string, data []byte) error {
				var update smtpConfigUpdate
				if err := json.Unmarshal(data, &update); err != nil {
					log.Warn("failed to parse CONFIG.smtp message", zap.Error(err))
					return nil // don't NAK — bad message would loop
				}
				sender.UpdateConfig(config.SMTPConfig{
					Host:     update.Host,
					Port:     update.Port,
					Username: update.Username,
					Password: update.Password,
					From:     update.From,
					UseTLS:   update.UseTLS,
				})
				return nil
			},
		)
		if err != nil {
			log.Warn("CONFIG.smtp subscriber exited", zap.Error(err))
		}
	}()

	// HTTP server — health only
	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-notification",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))
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

// smtpConfigUpdate is the payload published by the admin service to CONFIG.smtp.
type smtpConfigUpdate struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	UseTLS   bool   `json:"use_tls"`
}
