// Package config loads all service configuration from environment variables.
// Services call config.Load() at startup; missing required vars return an error.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the top-level configuration container.
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	NATS     NATSConfig
	SMTP     SMTPConfig
	JWT      JWTConfig
	S3       S3Config
}

// AppConfig holds application-level settings.
type AppConfig struct {
	// BaseURL is the externally reachable base URL (used in email links, OIDC
	// redirect URIs, etc.). Set via BASE_URL env var.
	BaseURL string
	// DevMode disables HSTS, relaxes CORS, and sets Secure:false on cookies.
	// Set via DEV_MODE env var.
	DevMode  bool
	LogLevel string
	// Pepper is an application-level secret appended to passwords before hashing
	// with Argon2id. Loaded from PASSWORD_PEPPER env var.
	Pepper string
	// EncryptionKey is a 32-byte master key for AES-256-GCM file encryption.
	// Loaded from STORAGE_ENCRYPTION_KEY env var (64 hex chars).
	EncryptionKey string
}

// ServerConfig holds HTTP server tuning parameters.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds PostgreSQL connection parameters.
type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
	// DSN is the assembled connection string (postgres://user:pass@host:port/db?sslmode=X).
	DSN string
}

// RedisConfig holds Redis/Valkey connection parameters.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	// Addr is host:port, assembled from Host and Port.
	Addr string
}

// SMTPConfig holds email delivery configuration for the notification service.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	// UseTLS enables STARTTLS when true; set false for MailHog/local dev.
	UseTLS bool
}

// NATSConfig holds NATS JetStream connection parameters.
type NATSConfig struct {
	URL string
}
type JWTConfig struct {
	// PrivateKeyPEM is the RSA private key PEM (PKCS#8 or PKCS#1) for signing
	// JWT tokens. Only needed by services that issue tokens (auth, transfer).
	// Loaded from JWT_PRIVATE_KEY env var (newlines escaped as \n).
	PrivateKeyPEM string
	// PublicKeyPEM is the RSA public key PEM for verifying JWT tokens.
	// Required by all services. Loaded from JWT_PUBLIC_KEY env var.
	PublicKeyPEM string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

// S3Config holds S3-compatible object storage parameters.
type S3Config struct {
	Endpoint  string
	PublicURL string // optional public URL used to rewrite presigned URLs for browser access
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// Load reads all configuration from environment variables.
// Returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{}

	// App
	cfg.App.BaseURL = getEnv("BASE_URL", "http://localhost")
	cfg.App.DevMode = getEnvBool("DEV_MODE", true)
	cfg.App.LogLevel = getEnv("LOG_LEVEL", "info")

	// Server
	cfg.Server.Port = getEnv("PORT", "8080")
	cfg.Server.ReadTimeout = getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second)
	cfg.Server.WriteTimeout = getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second)
	cfg.Server.IdleTimeout = getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second)

	// Database
	cfg.Database.Host = getEnv("POSTGRES_HOST", "localhost")
	cfg.Database.Port = getEnv("POSTGRES_PORT", "5432")
	cfg.Database.Name = getEnv("POSTGRES_DB", "tenzoshare")
	cfg.Database.User = getEnv("POSTGRES_USER", "tenzoshare")
	cfg.Database.SSLMode = getEnv("POSTGRES_SSLMODE", "disable")
	dbPass, err := requireEnv("POSTGRES_PASSWORD")
	if err != nil {
		return nil, err
	}
	cfg.Database.Password = dbPass
	cfg.Database.DSN = fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.Name, cfg.Database.SSLMode,
	)

	// Redis
	cfg.Redis.Host = getEnv("REDIS_HOST", "localhost")
	cfg.Redis.Port = getEnv("REDIS_PORT", "6379")
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.DB = getEnvInt("REDIS_DB", 0)
	cfg.Redis.Addr = cfg.Redis.Host + ":" + cfg.Redis.Port

	// NATS
	cfg.NATS.URL = getEnv("NATS_URL", "nats://localhost:4222")

	// JWT — RS256 asymmetric keys
	cfg.JWT.PrivateKeyPEM = normalisePEM(os.Getenv("JWT_PRIVATE_KEY"))
	cfg.JWT.PublicKeyPEM = normalisePEM(os.Getenv("JWT_PUBLIC_KEY"))
	cfg.JWT.AccessTTL = getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute)
	cfg.JWT.RefreshTTL = getEnvDuration("JWT_REFRESH_TTL", 168*time.Hour)

	// Password pepper
	cfg.App.Pepper = getEnv("PASSWORD_PEPPER", "")

	// AES-256-GCM master encryption key (optional — only required for storage service)
	cfg.App.EncryptionKey = getEnv("STORAGE_ENCRYPTION_KEY", "")

	// S3 / MinIO
	cfg.S3.Endpoint = getEnv("S3_ENDPOINT", "http://localhost:9000")
	cfg.S3.PublicURL = getEnv("S3_PUBLIC_URL", "")
	cfg.S3.Region = getEnv("S3_REGION", "us-east-1")
	cfg.S3.Bucket = getEnv("S3_BUCKET", "tenzoshare")
	cfg.S3.AccessKey = getEnv("S3_ACCESS_KEY", "")
	cfg.S3.SecretKey = getEnv("S3_SECRET_KEY", "")
	cfg.S3.UseSSL = getEnvBool("S3_USE_SSL", false)

	// SMTP (notification service)
	cfg.SMTP.Host = getEnv("SMTP_HOST", "localhost")
	cfg.SMTP.Port = getEnv("SMTP_PORT", "1025")
	cfg.SMTP.Username = getEnv("SMTP_USERNAME", "")
	cfg.SMTP.Password = getEnv("SMTP_PASSWORD", "")
	cfg.SMTP.From = getEnv("SMTP_FROM", "noreply@tenzoshare.io")
	cfg.SMTP.UseTLS = getEnvBool("SMTP_USE_TLS", false)

	return cfg, nil
}

// requireEnv returns the value of an env var or an error if it is unset/empty.
func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}

// normalisePEM converts escaped \n sequences back to real newlines so PEM
// keys can be stored as single-line env var values.
func normalisePEM(s string) string {
	if s == "" {
		return ""
	}
	// Replace literal backslash-n with actual newline
	result := ""
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '\\' && s[i+1] == 'n' {
			result += "\n"
			i += 2
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
