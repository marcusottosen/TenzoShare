package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v3"
	"github.com/tus/tusd/v2/pkg/handler"
	tuss3 "github.com/tus/tusd/v2/pkg/s3store"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.uber.org/zap"

	appconfig "github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

func main() {
	cfg, err := appconfig.Load()
	if err != nil {
		stdlog.Fatalf("failed to load config: %v", err)
	}
	cfg.Server.Port = getEnvOr("PORT", "8084")

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── NATS JetStream ─────────────────────────────────────────────────────────
	var js *jetstream.Client
	if cfg.NATS.URL != "" {
		js, err = jetstream.New(cfg.NATS.URL)
		if err != nil {
			log.Warn("failed to connect to NATS — upload events will not be published", zap.Error(err))
		} else {
			if err2 := js.EnsureStreams(ctx); err2 != nil {
				log.Warn("failed to ensure NATS streams", zap.Error(err2))
			}
			log.Info("connected to NATS JetStream")
		}
	}

	// ── Build AWS S3 client for MinIO ──────────────────────────────────────────
	s3Client, err := buildS3Client(ctx, cfg)
	if err != nil {
		log.Fatal("failed to build s3 client", zap.Error(err))
	}

	// ── tusd S3 store ──────────────────────────────────────────────────────────
	store := tuss3.New(cfg.S3.Bucket, s3Client)
	composer := handler.NewStoreComposer()
	store.UseIn(composer)

	tusHandler, err := handler.NewHandler(handler.Config{
		BasePath:                "/api/v1/uploads/",
		StoreComposer:           composer,
		RespectForwardedHeaders: true,
	})
	if err != nil {
		log.Fatal("failed to create tusd handler", zap.Error(err))
	}

	// ── Wire upload-completion events to NATS ──────────────────────────────────
	if js != nil {
		go publishCompletions(ctx, tusHandler, js, log)
	}

	// Wrap tusd net/http handler → fasthttp → fiber.Handler
	rawFasthttpHandler := fasthttpadaptor.NewFastHTTPHandler(tusHandler)
	tusHandlerFiber := func(c fiber.Ctx) error {
		if dc, ok := c.(*fiber.DefaultCtx); ok {
			rawFasthttpHandler(dc.RequestCtx())
		}
		return nil
	}

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-upload",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
		BodyLimit:    -1, // tusd controls its own limits
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "upload"})
	})
	app.Get("/api/v1/uploads/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "upload"})
	})

	// All tusd methods (POST, PATCH, HEAD, OPTIONS, DELETE) under /api/v1/uploads
	tusRoutes := app.Group("/api/v1/uploads", middleware.JWTAuth(cfg.JWT.Secret))
	tusRoutes.All("/", tusHandlerFiber)
	tusRoutes.All("/:id", tusHandlerFiber)

	go func() {
		log.Info("upload service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down upload service")
	if err := app.Shutdown(); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

// publishCompletions drains the tusd CompleteUploads channel and publishes each
// completed upload as an UPLOADS.completed event to NATS JetStream.
func publishCompletions(ctx context.Context, h *handler.Handler, js *jetstream.Client, log *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-h.CompleteUploads:
			if !ok {
				return
			}
			info := event.Upload
			payload := map[string]any{
				"upload_id":    info.ID,
				"size":         info.Size,
				"filename":     info.MetaData["filename"],
				"filetype":     info.MetaData["filetype"],
				"owner_id":     info.MetaData["owner_id"],
				"storage_type": "s3",
			}
			if err := js.Publish(ctx, "UPLOADS.completed", payload); err != nil {
				log.Warn("failed to publish UPLOADS.completed", zap.String("upload_id", info.ID), zap.Error(err))
			} else {
				log.Info("published UPLOADS.completed", zap.String("upload_id", info.ID))
			}
		}
	}
}

func buildS3Client(ctx context.Context, cfg *appconfig.Config) (*s3.Client, error) {
	endpoint := cfg.S3.Endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...any) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               endpoint,
				SigningRegion:     cfg.S3.Region,
				HostnameImmutable: true,
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3.AccessKey, cfg.S3.SecretKey, "",
		)),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	}), nil
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
