package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/handler"
	"github.com/tenzoshare/tenzoshare/services/admin/internal/repository"
	adminserver "github.com/tenzoshare/tenzoshare/services/admin/internal/server"
	"github.com/tenzoshare/tenzoshare/services/admin/internal/service"
	"github.com/tenzoshare/tenzoshare/shared/pkg/cache"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/database"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
)

func main() {
	// ── Load configuration ───────────────────────────────────────────────────

	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("failed to load config: %v", err)
	}
	cfg.Server.Port = getEnvOr("PORT", "8087")

	// ── Initialize logger ────────────────────────────────────────────────────

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	// ── Setup graceful shutdown context ──────────────────────────────────────

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Connect to database ──────────────────────────────────────────────────

	db, err := database.Connect(ctx, database.DefaultConfig(cfg.Database.DSN))
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("database connection established")

	// ── Run migrations ───────────────────────────────────────────────────────

	// Note: migrations are run by the bootstrap script or manually
	// Uncomment if you want to auto-migrate on startup:
	// if err := database.RunMigrations(ctx, db, svcmigrations.Migrations, "admin"); err != nil {
	// 	log.Fatal("failed to run migrations", zap.Error(err))
	// }

	// ── Connect to NATS JetStream (optional) ─────────────────────────────────

	var js *jetstream.Client
	if cfg.NATS.URL != "" {
		js, err = jetstream.New(cfg.NATS.URL)
		if err != nil {
			log.Warn("failed to connect to NATS — audit events will not be published", zap.Error(err))
		} else {
			defer js.Close()
			log.Info("NATS JetStream connection established")
		}
	}

	// ── Connect to cache (optional) ──────────────────────────────────────────

	var cacheClient *cache.Client
	if cfg.Redis.Addr != "" {
		cacheClient, err = cache.New(cfg.Redis)
		if err != nil {
			log.Fatal("failed to connect to cache", zap.Error(err))
		}
		defer cacheClient.Close()
		log.Info("cache connection established")
	}

	// ── Initialize layers ────────────────────────────────────────────────────

	repo := repository.New(db, js)
	svc := service.New(repo, cfg, log)
	h := handler.New(svc)

	// ── Create Fiber app ─────────────────────────────────────────────────────

	app, err := adminserver.New(&adminserver.Config{
		Cfg:         cfg,
		Handler:     h,
		CacheClient: cacheClient,
		Log:         log,
	})
	if err != nil {
		log.Fatal("failed to create server", zap.Error(err))
	}

	// ── Start server in background ───────────────────────────────────────────

	go func() {
		addr := ":" + cfg.Server.Port
		log.Info("starting admin service", zap.String("port", cfg.Server.Port))
		if err := app.Listen(addr); err != nil {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Background audit log purge ───────────────────────────────────────────

	go runAuditPurgeScheduler(ctx, repo, log)

	// ── Wait for shutdown signal ─────────────────────────────────────────────

	<-ctx.Done()
	log.Info("shutting down admin service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
}

// ── Background audit purge scheduler ─────────────────────────────────────────

func runAuditPurgeScheduler(ctx context.Context, repo *repository.Repository, log *zap.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run once at startup
	runAuditPurge(ctx, repo, log)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runAuditPurge(ctx, repo, log)
		}
	}
}

func runAuditPurge(ctx context.Context, repo *repository.Repository, log *zap.Logger) {
	cfg, err := repo.GetAuditConfig(ctx)
	if err != nil {
		log.Error("failed to get audit config", zap.Error(err))
		return
	}

	if !cfg.RetentionEnabled {
		return
	}

	deleted, err := repo.PurgeAuditLogs(ctx, cfg.RetentionDays)
	if err != nil {
		log.Error("failed to purge audit logs", zap.Error(err))
		return
	}

	if deleted > 0 {
		log.Info("audit log purge completed", zap.Int64("deleted", deleted))
	}
}

// ── Helper ───────────────────────────────────────────────────────────────────

func getEnvOr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
