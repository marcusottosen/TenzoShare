package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("setenv %s: %v", k, err)
		}
		t.Cleanup(func() { os.Unsetenv(k) })
	}
}

// ── Default values ────────────────────────────────────────────────────────────

func TestLoad_Defaults(t *testing.T) {
	// POSTGRES_PASSWORD is required; set a placeholder so Load doesn't fail.
	setEnv(t, map[string]string{"POSTGRES_PASSWORD": "testpass"})

	// Unset optional vars to verify defaults
	vars := []string{
		"BASE_URL", "DEV_MODE", "LOG_LEVEL", "PORT",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB",
		"POSTGRES_USER", "POSTGRES_SSLMODE",
	}
	for _, v := range vars {
		old := os.Getenv(v)
		os.Unsetenv(v)
		defer os.Setenv(v, old)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.App.BaseURL != "http://localhost" {
		t.Errorf("BaseURL default = %q, want %q", cfg.App.BaseURL, "http://localhost")
	}
	if cfg.Server.Port != "8080" {
		t.Errorf("Port default = %q, want %q", cfg.Server.Port, "8080")
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("DB Host default = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.Database.Port != "5432" {
		t.Errorf("DB Port default = %q, want %q", cfg.Database.Port, "5432")
	}
	if cfg.Database.Name != "tenzoshare" {
		t.Errorf("DB Name default = %q, want %q", cfg.Database.Name, "tenzoshare")
	}
}

// ── Custom values are read from environment ───────────────────────────────────

func TestLoad_CustomBaseURL(t *testing.T) {
	setEnv(t, map[string]string{
		"BASE_URL":          "https://share.example.com",
		"POSTGRES_PASSWORD": "testpass",
	})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.App.BaseURL != "https://share.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.App.BaseURL, "https://share.example.com")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	setEnv(t, map[string]string{"PORT": "9090", "POSTGRES_PASSWORD": "testpass"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Server.Port, "9090")
	}
}

func TestLoad_PepperFromEnv(t *testing.T) {
	setEnv(t, map[string]string{"PASSWORD_PEPPER": "supersecretpeppervalue", "POSTGRES_PASSWORD": "testpass"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.App.Pepper != "supersecretpeppervalue" {
		t.Errorf("Pepper = %q, want %q", cfg.App.Pepper, "supersecretpeppervalue")
	}
}

func TestLoad_DevMode_True(t *testing.T) {
	setEnv(t, map[string]string{"DEV_MODE": "true", "POSTGRES_PASSWORD": "testpass"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !cfg.App.DevMode {
		t.Error("DevMode should be true")
	}
}

func TestLoad_DevMode_False(t *testing.T) {
	setEnv(t, map[string]string{"DEV_MODE": "false", "POSTGRES_PASSWORD": "testpass"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.App.DevMode {
		t.Error("DevMode should be false")
	}
}

// ── DSN assembly ──────────────────────────────────────────────────────────────

func TestLoad_DSNAssembled(t *testing.T) {
	setEnv(t, map[string]string{
		"POSTGRES_HOST":     "dbhost",
		"POSTGRES_PORT":     "5432",
		"POSTGRES_DB":       "mydb",
		"POSTGRES_USER":     "myuser",
		"POSTGRES_PASSWORD": "mypass",
		"POSTGRES_SSLMODE":  "require",
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Database.DSN == "" {
		t.Fatal("DSN should not be empty after loading postgres config")
	}
}

// ── Redis addr assembly ───────────────────────────────────────────────────────

func TestLoad_RedisAddrAssembled(t *testing.T) {
	setEnv(t, map[string]string{
		"REDIS_HOST":        "redishost",
		"REDIS_PORT":        "6380",
		"POSTGRES_PASSWORD": "testpass",
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Redis.Addr == "" {
		t.Fatal("Redis.Addr should be assembled from REDIS_HOST + REDIS_PORT")
	}
}

// ── JWT TTL parsing ───────────────────────────────────────────────────────────

func TestLoad_JWTAccessTTL(t *testing.T) {
	setEnv(t, map[string]string{"JWT_ACCESS_TTL": "30m", "POSTGRES_PASSWORD": "testpass"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.JWT.AccessTTL != 30*time.Minute {
		t.Errorf("JWT.AccessTTL = %v, want %v", cfg.JWT.AccessTTL, 30*time.Minute)
	}
}

func TestLoad_JWTRefreshTTL(t *testing.T) {
	setEnv(t, map[string]string{"JWT_REFRESH_TTL": "168h", "POSTGRES_PASSWORD": "testpass"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Errorf("JWT.RefreshTTL = %v, want %v", cfg.JWT.RefreshTTL, 168*time.Hour)
	}
}
