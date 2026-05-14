package main

import (
	"context"
	stdlog "log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	"github.com/tenzoshare/tenzoshare/shared/pkg/jwtkeys"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
	"github.com/tenzoshare/tenzoshare/shared/pkg/telemetry"
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
		NotifyCompleteUploads:   true, // must be true for CompleteUploads channel to fire
		NotifyCreatedUploads:    true,
		NotifyTerminatedUploads: true,
	})
	if err != nil {
		log.Fatal("failed to create tusd handler", zap.Error(err))
	}

	// ── Drain all tusd event channels (must run even when NATS is unavailable) ─
	go watchTUSEvents(ctx, tusHandler, js, log)

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
		// ProxyHeader + TrustProxy tell Fiber to read c.IP() from X-Real-IP when
		// the connection arrives from a trusted private-network proxy (Traefik).
		ProxyHeader:      "X-Real-IP",
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Private: true},
		BodyLimit:        -1, // tusd controls its own limits
	})

	pubKey, err := jwtkeys.ParsePublicKey(cfg.JWT.PublicKeyPEM)
	if err != nil {
		log.Fatal("failed to parse JWT public key", zap.Error(err))
	}

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders(cfg.App.DevMode))
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))
	app.Use(middleware.RequestLogger(log))

	telemetry.Register(app, "upload")
	app.Get("/api/v1/uploads/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "upload"})
	})

	// Log and audit any failed upload requests (auth rejections, 4xx/5xx from tusd).
	app.Use("/api/v1/uploads", uploadAuditLogger(log, js))

	// All tusd methods (POST, PATCH, HEAD, OPTIONS, DELETE) under /api/v1/uploads
	tusRoutes := app.Group("/api/v1/uploads", middleware.JWTAuth(pubKey))
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

// watchTUSEvents drains all tusd event channels. It MUST always run (even when
// NATS is unavailable) because the channels are unbuffered — leaving them
// unread would cause tusd to block on every event.
func watchTUSEvents(ctx context.Context, h *handler.Handler, js *jetstream.Client, log *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-h.CreatedUploads:
			if !ok {
				return
			}
			info := event.Upload
			log.Info("upload initiated",
				zap.String("upload_id", info.ID),
				zap.String("owner_id", info.MetaData["owner_id"]),
				zap.String("filename", info.MetaData["filename"]),
				zap.Int64("size", info.Size),
			)
			if js != nil {
				_ = js.Publish(ctx, "AUDIT.upload", map[string]any{
					"action":    "upload.initiated",
					"user_id":   info.MetaData["owner_id"],
					"subject":   info.ID,
					"success":   true,
					"timestamp": time.Now(),
					"payload": map[string]any{
						"filename":  info.MetaData["filename"],
						"size":      info.Size,
						"upload_id": info.ID,
					},
				})
			}

		case event, ok := <-h.CompleteUploads:
			if !ok {
				return
			}
			info := event.Upload
			log.Info("upload completed",
				zap.String("upload_id", info.ID),
				zap.String("owner_id", info.MetaData["owner_id"]),
				zap.String("filename", info.MetaData["filename"]),
				zap.Int64("size", info.Size),
			)
			if js != nil {
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
				}
				_ = js.Publish(ctx, "AUDIT.upload", map[string]any{
					"action":    "upload.completed",
					"user_id":   info.MetaData["owner_id"],
					"subject":   info.ID,
					"success":   true,
					"timestamp": time.Now(),
					"payload": map[string]any{
						"filename":  info.MetaData["filename"],
						"filetype":  info.MetaData["filetype"],
						"size":      info.Size,
						"upload_id": info.ID,
					},
				})
			}

		case event, ok := <-h.TerminatedUploads:
			if !ok {
				return
			}
			info := event.Upload
			log.Warn("upload terminated",
				zap.String("upload_id", info.ID),
				zap.String("owner_id", info.MetaData["owner_id"]),
				zap.String("filename", info.MetaData["filename"]),
				zap.Int64("offset", info.Offset),
				zap.Int64("size", info.Size),
			)
			if js != nil {
				_ = js.Publish(ctx, "AUDIT.upload", map[string]any{
					"action":    "upload.terminated",
					"user_id":   info.MetaData["owner_id"],
					"subject":   info.ID,
					"success":   false,
					"timestamp": time.Now(),
					"payload": map[string]any{
						"filename":  info.MetaData["filename"],
						"size":      info.Size,
						"offset":    info.Offset,
						"upload_id": info.ID,
					},
				})
			}
		}
	}
}

// uploadAuditLogger returns a Fiber middleware that logs and audits any
// non-2xx response on the /api/v1/uploads routes, capturing auth rejections,
// size-limit errors, and any error tusd writes to the HTTP response.
func uploadAuditLogger(log *zap.Logger, js *jetstream.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil || status >= 400 {
			uploadID := c.Params("id")
			userID, _ := c.Locals("userID").(string)
			log.Warn("upload request failed",
				zap.String("method", c.Method()),
				zap.String("path", c.Path()),
				zap.Int("status", status),
				zap.String("upload_id", uploadID),
				zap.String("user_id", userID),
				zap.String("ip", c.IP()),
				zap.NamedError("reason", err),
			)
			if js != nil {
				ev := map[string]any{
					"action":    "upload.failed",
					"user_id":   userID,
					"subject":   uploadID,
					"success":   false,
					"timestamp": time.Now(),
					"payload": map[string]any{
						"status":    status,
						"method":    c.Method(),
						"client_ip": c.IP(),
					},
				}
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_ = js.Publish(ctx, "AUDIT.upload", ev)
				}()
			}
		}
		return err
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
