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

	// ── Build AWS S3 client for MinIO ──────────────────────────────────────────
	s3Client, err := buildS3Client(ctx, cfg)
	if err != nil {
		log.Fatal("failed to build s3 client", zap.Error(err))
	}

	// ── tusd S3 store ──────────────────────────────────────────────────────────
	store := tuss3.New(cfg.S3.Bucket, s3Client)
	store.UseIn(nil) // register store capabilities

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
