package server

import (
	"context"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/handler"
	"github.com/tenzoshare/tenzoshare/shared/pkg/cache"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jwtkeys"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
	"github.com/tenzoshare/tenzoshare/shared/pkg/telemetry"
)

// Config holds dependencies for server setup.
type Config struct {
	Cfg         *config.Config
	Handler     *handler.Handler
	CacheClient *cache.Client
	Log         *zap.Logger
}

// New creates and configures a new Fiber app with all routes and middleware.
func New(cfg *Config) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		AppName:          "tenzoshare-admin",
		ReadTimeout:      cfg.Cfg.Server.ReadTimeout,
		WriteTimeout:     cfg.Cfg.Server.WriteTimeout,
		ErrorHandler:     middleware.ErrorHandler,
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Private: true, Loopback: true},
		ProxyHeader:      "X-Real-IP",
	})

	// Parse JWT public key
	pubKey, err := jwtkeys.ParsePublicKey(cfg.Cfg.JWT.PublicKeyPEM)
	if err != nil {
		return nil, err
	}

	// Revocation check
	revocationCheck := middleware.TokenRevocation(func(ctx context.Context, jti string) bool {
		if cfg.CacheClient == nil {
			return false
		}
		return cfg.CacheClient.IsTokenRevoked(ctx, jti)
	})

	// Global middleware
	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders(cfg.Cfg.App.DevMode))
	app.Use(middleware.CORS(cfg.Cfg.App.DevMode, allowedOrigins))
	app.Use(middleware.RequestLogger(cfg.Log))

	// Telemetry
	telemetry.Register(app, "admin")

	// Public endpoints (no auth)
	app.Get("/api/v1/branding", cfg.Handler.GetBrandingPublic)
	app.Get("/api/v1/platform/config", cfg.Handler.GetPlatformConfigPublic)

	// Admin API routes
	v1 := app.Group("/api/v1/admin")

	// Health check (no auth)
	v1.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// Protected admin routes
	protected := v1.Group("",
		middleware.TokenAuth(pubKey, middleware.NewAPIKeyValidator(cfg.Handler.Service().Repository().DB())),
		revocationCheck,
		middleware.RequireRole("admin"),
	)

	// User management
	protected.Get("/users", cfg.Handler.ListUsers)
	protected.Post("/users", cfg.Handler.CreateUser)
	protected.Patch("/users/:id", cfg.Handler.UpdateUser)
	protected.Delete("/users/:id", cfg.Handler.DeleteUser)
	protected.Post("/users/:id/unlock", cfg.Handler.UnlockUser)
	protected.Post("/users/:id/verify", cfg.Handler.VerifyUserEmail)
	protected.Post("/users/:id/reset-password", cfg.Handler.ResetUserPassword)
	protected.Post("/users/:id/set-password", cfg.Handler.SetUserPassword)
	protected.Delete("/users/:id/mfa", cfg.Handler.ResetUserMFA)

	// User quotas
	protected.Get("/quotas", cfg.Handler.ListUserQuotas)
	protected.Get("/users/:id/quota", cfg.Handler.GetUserQuota)
	protected.Put("/users/:id/quota", cfg.Handler.PutUserQuota)

	// Stats & health
	protected.Get("/stats", cfg.Handler.GetStats)
	protected.Get("/system/health", cfg.Handler.GetSystemHealth)
	protected.Get("/system/metrics", cfg.Handler.GetSystemMetrics)

	// Storage management
	protected.Get("/storage/usage", cfg.Handler.ListStorageUsage)
	protected.Get("/storage/config", cfg.Handler.GetStorageConfig)
	protected.Put("/storage/config", cfg.Handler.PutStorageConfig)
	protected.Get("/storage/files", cfg.Handler.ListStorageFiles)
	protected.Delete("/storage/files/:id", cfg.Handler.DeleteFile)
	protected.Post("/storage/purge", cfg.Handler.TriggerPurge)
	protected.Get("/storage/purge-log", cfg.Handler.ListPurgeLog)
	protected.Get("/storage/insights", cfg.Handler.GetStorageInsights)

	// Transfer management
	protected.Get("/transfers", cfg.Handler.ListTransfers)
	protected.Get("/transfers/:id", cfg.Handler.GetTransfer)
	protected.Post("/transfers/:id/revoke", cfg.Handler.RevokeTransfer)

	// Configuration
	protected.Get("/audit/config", cfg.Handler.GetAuditConfig)
	protected.Put("/audit/config", cfg.Handler.PutAuditConfig)
	protected.Post("/audit/purge", cfg.Handler.TriggerAuditPurge)
	protected.Get("/audit/stats", cfg.Handler.GetAuditStats)

	protected.Get("/auth/config", cfg.Handler.GetAuthConfig)
	protected.Put("/auth/config", cfg.Handler.PutAuthConfig)

	protected.Get("/branding", cfg.Handler.GetBranding)
	protected.Put("/branding", cfg.Handler.PutBranding)

	protected.Get("/platform/config", cfg.Handler.GetPlatformConfig)
	protected.Put("/platform/config", cfg.Handler.PutPlatformConfig)

	protected.Get("/settings/smtp", cfg.Handler.GetSMTPSettings)
	protected.Put("/settings/smtp", cfg.Handler.PutSMTPSettings)
	protected.Post("/settings/smtp/test", cfg.Handler.TestSMTPSettings)

	return app, nil
}
