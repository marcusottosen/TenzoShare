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

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-auth",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	registerRoutes(app)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

func registerRoutes(app *fiber.App) {
	v1 := app.Group("/api/v1")
	auth := v1.Group("/auth")

	auth.Post("/register", handleRegister)
	auth.Post("/login", handleLogin)
	auth.Post("/logout", handleLogout)
	auth.Post("/refresh", handleRefresh)
	auth.Post("/password-reset/request", handlePasswordResetRequest)
	auth.Post("/password-reset/confirm", handlePasswordResetConfirm)
	auth.Post("/mfa/setup", handleMFASetup)
	auth.Post("/mfa/verify", handleMFAVerify)
	auth.Get("/oidc/callback", handleOIDCCallback)

	// Health check — used by Traefik and Docker healthchecks
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "auth"})
	})
}

// Placeholder handlers — implemented in internal/handlers/
// Note: Fiber v3 uses `fiber.Ctx` (interface), not `*fiber.Ctx` (pointer)
func handleRegister(c fiber.Ctx) error             { return fiber.ErrNotImplemented }
func handleLogin(c fiber.Ctx) error                { return fiber.ErrNotImplemented }
func handleLogout(c fiber.Ctx) error               { return fiber.ErrNotImplemented }
func handleRefresh(c fiber.Ctx) error              { return fiber.ErrNotImplemented }
func handlePasswordResetRequest(c fiber.Ctx) error { return fiber.ErrNotImplemented }
func handlePasswordResetConfirm(c fiber.Ctx) error { return fiber.ErrNotImplemented }
func handleMFASetup(c fiber.Ctx) error             { return fiber.ErrNotImplemented }
func handleMFAVerify(c fiber.Ctx) error            { return fiber.ErrNotImplemented }
func handleOIDCCallback(c fiber.Ctx) error         { return fiber.ErrNotImplemented }
