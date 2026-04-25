package main

import (
	"context"
	"encoding/json"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/database"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/logger"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

// ── Domain types ──────────────────────────────────────────────────────────────

type UserRow struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	EmailVerified       bool       `json:"email_verified"`
	FailedLoginAttempts int        `json:"failed_login_attempts"`
	LockedUntil         *time.Time `json:"locked_until"`
	CreatedAt           time.Time  `json:"created_at"`
}

type SystemStats struct {
	TotalUsers     int   `json:"total_users"`
	TotalTransfers int   `json:"total_transfers"`
	TotalFiles     int   `json:"total_files"`
	TotalStorageB  int64 `json:"total_storage_bytes"`
}

type ServiceHealthItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// ── Global DB pool ────────────────────────────────────────────────────────────

var db *pgxpool.Pool

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("failed to load config: %v", err)
	}
	cfg.Server.Port = getEnvOr("PORT", "8087")

	log, err := logger.New(cfg.App.LogLevel, cfg.App.DevMode)
	if err != nil {
		stdlog.Fatalf("failed to initialize logger: %v", err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err = database.Connect(ctx, database.DefaultConfig(cfg.Database.DSN))
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	app := fiber.New(fiber.Config{
		AppName:      "tenzoshare-admin",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler,
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "admin"})
	})

	v1 := app.Group("/api/v1/admin", middleware.JWTAuth(cfg.JWT.Secret), middleware.RequireRole("admin"))
	v1.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "admin"})
	})
	v1.Get("/users", handleListUsers)
	v1.Patch("/users/:id", handleUpdateUser)
	v1.Get("/stats", handleGetStats)
	v1.Get("/system/health", handleSystemHealth)

	go func() {
		log.Info("admin service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down admin service")
	if err := app.Shutdown(); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GET /api/v1/admin/users?limit=50&offset=0&search=<email>&role=<role>
func handleListUsers(c fiber.Ctx) error {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	search := c.Query("search")
	role := c.Query("role")

	args := []any{}
	where := ""
	idx := 1
	add := func(cond string, val any) {
		if where == "" {
			where = "WHERE "
		} else {
			where += " AND "
		}
		where += cond
		args = append(args, val)
		idx++
	}
	if search != "" {
		add("email ILIKE $1", "%"+search+"%")
		idx = 2
	}
	if role != "" {
		add("role = $"+itoa(idx), role)
	}

	var total int
	if err := db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users "+where, args...).Scan(&total); err != nil {
		return apperrors.Internal("count users", err)
	}

	dataSQL := "SELECT id, email, role, is_active, email_verified, failed_login_attempts, locked_until, created_at FROM auth.users " +
		where + " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := db.Query(c.Context(), dataSQL, args...)
	if err != nil {
		return apperrors.Internal("list users", err)
	}
	defer rows.Close()

	users := make([]UserRow, 0)
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.IsActive, &u.EmailVerified,
			&u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt); err != nil {
			return apperrors.Internal("scan user row", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return apperrors.Internal("iterate users", err)
	}

	return c.JSON(fiber.Map{"users": users, "total": total, "limit": limit, "offset": offset})
}

// PATCH /api/v1/admin/users/:id  body: {role?, is_active?}
func handleUpdateUser(c fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return apperrors.BadRequest("invalid JSON body")
	}
	if body.Role == nil && body.IsActive == nil {
		return apperrors.BadRequest("provide role or is_active to update")
	}
	if body.Role != nil && *body.Role != "admin" && *body.Role != "user" {
		return apperrors.BadRequest("role must be 'admin' or 'user'")
	}

	if body.Role != nil {
		if _, err := db.Exec(c.Context(),
			"UPDATE auth.users SET role = $1, updated_at = now() WHERE id = $2",
			*body.Role, id); err != nil {
			return apperrors.Internal("update user role", err)
		}
	}
	if body.IsActive != nil {
		if _, err := db.Exec(c.Context(),
			"UPDATE auth.users SET is_active = $1, updated_at = now() WHERE id = $2",
			*body.IsActive, id); err != nil {
			return apperrors.Internal("update user active", err)
		}
	}

	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/v1/admin/stats
func handleGetStats(c fiber.Ctx) error {
	var s SystemStats
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users").Scan(&s.TotalUsers)
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM transfer.transfers WHERE is_revoked = false").Scan(&s.TotalTransfers)
	_ = db.QueryRow(c.Context(), "SELECT count(*), coalesce(sum(size_bytes),0) FROM storage.files WHERE deleted_at IS NULL").
		Scan(&s.TotalFiles, &s.TotalStorageB)
	return c.JSON(s)
}

// GET /api/v1/admin/system/health
func handleSystemHealth(c fiber.Ctx) error {
	type serviceInfo struct{ name, path string }
	services := []serviceInfo{
		{"Auth", "/api/v1/auth/health"},
		{"Transfer", "/api/v1/transfers/health"},
		{"Storage", "/api/v1/files/health"},
		{"Upload", "/api/v1/uploads/health"},
		{"Notification", "/api/v1/notification/health"},
		{"Audit", "/api/v1/audit/health"},
		{"Admin", "/api/v1/admin/health"},
	}

	// Call via the gateway (Traefik on port 80 in the Docker network)
	gatewayBase := getEnvOr("GATEWAY_URL", "http://tenzoshare-gateway:80")
	client := &http.Client{Timeout: 3 * time.Second}

	results := make([]ServiceHealthItem, 0, len(services))
	for _, svc := range services {
		start := time.Now()
		resp, err := client.Get(gatewayBase + svc.path)
		latencyMs := time.Since(start).Milliseconds()
		item := ServiceHealthItem{Name: svc.name, LatencyMs: latencyMs}
		if err != nil || resp.StatusCode >= 400 {
			item.Status = "down"
		} else {
			item.Status = "up"
		}
		if resp != nil {
			resp.Body.Close()
		}
		results = append(results, item)
	}

	return c.JSON(fiber.Map{"services": results})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func itoa(n int) string { return strconv.Itoa(n) }

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
