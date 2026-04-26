package main

import (
	"context"
	"encoding/json"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	"github.com/tenzoshare/tenzoshare/shared/pkg/database"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jwtkeys"
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
	UpdatedAt           time.Time  `json:"updated_at"`
}

type SystemStats struct {
	TotalUsers     int   `json:"total_users"`
	NewUsers30d    int   `json:"new_users_30d"`
	TotalTransfers int   `json:"total_transfers"`
	TotalFiles     int   `json:"total_files"`
	TotalStorageB  int64 `json:"total_storage_bytes"`
}

type ServiceHealthItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// ── Global DB pool and config ─────────────────────────────────────────────────

var db *pgxpool.Pool
var cfg *config.Config

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	var err error
	cfg, err = config.Load()
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

	pubKey, err := jwtkeys.ParsePublicKey(cfg.JWT.PublicKeyPEM)
	if err != nil {
		log.Fatal("failed to parse JWT public key", zap.Error(err))
	}

	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS(cfg.App.DevMode, allowedOrigins))

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "admin"})
	})

	v1 := app.Group("/api/v1/admin", middleware.JWTAuth(pubKey), middleware.RequireRole("admin"))
	v1.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "admin"})
	})
	v1.Get("/users", handleListUsers)
	v1.Post("/users", handleCreateUser)
	v1.Patch("/users/:id", handleUpdateUser)
	v1.Delete("/users/:id", handleDeleteUser)
	v1.Post("/users/:id/unlock", handleUnlockUser)
	v1.Post("/users/:id/verify", handleVerifyEmail)
	v1.Get("/stats", handleGetStats)
	v1.Get("/system/health", handleSystemHealth)
	v1.Get("/transfers", handleListTransfers)
	v1.Get("/transfers/:id", handleGetTransfer)
	v1.Post("/transfers/:id/revoke", handleRevokeTransfer)

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

	dataSQL := "SELECT id, email, role, is_active, email_verified, failed_login_attempts, locked_until, created_at, updated_at FROM auth.users " +
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
			&u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return apperrors.Internal("scan user row", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return apperrors.Internal("iterate users", err)
	}

	return c.JSON(fiber.Map{"users": users, "total": total, "limit": limit, "offset": offset})
}

// POST /api/v1/admin/users  body: {email, password, role}
func handleCreateUser(c fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return apperrors.BadRequest("invalid JSON body")
	}
	if body.Email == "" || body.Password == "" {
		return apperrors.BadRequest("email and password are required")
	}
	if body.Role == "" {
		body.Role = "user"
	}
	if body.Role != "admin" && body.Role != "user" {
		return apperrors.BadRequest("role must be 'admin' or 'user'")
	}
	if len(body.Password) < 8 {
		return apperrors.BadRequest("password must be at least 8 characters")
	}

	hash, err := crypto.HashPassword(body.Password, cfg.App.Pepper)
	if err != nil {
		return apperrors.Internal("hash password", err)
	}

	var u UserRow
	err = db.QueryRow(c.Context(),
		`INSERT INTO auth.users (email, password_hash, role, is_active, email_verified)
		 VALUES ($1, $2, $3, true, false)
		 RETURNING id, email, role, is_active, email_verified, failed_login_attempts, locked_until, created_at, updated_at`,
		body.Email, hash, body.Role,
	).Scan(&u.ID, &u.Email, &u.Role, &u.IsActive, &u.EmailVerified,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		// unique constraint violation
		if isUniqueViolation(err) {
			return apperrors.BadRequest("email already in use")
		}
		return apperrors.Internal("create user", err)
	}

	return c.Status(fiber.StatusCreated).JSON(u)
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

// DELETE /api/v1/admin/users/:id
func handleDeleteUser(c fiber.Ctx) error {
	id := c.Params("id")

	// Prevent deleting the requesting admin's own account
	callerID, _ := c.Locals("userID").(string)
	if callerID == id {
		return apperrors.BadRequest("cannot delete your own account")
	}

	tag, err := db.Exec(c.Context(), "DELETE FROM auth.users WHERE id = $1", id)
	if err != nil {
		return apperrors.Internal("delete user", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("user not found")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// POST /api/v1/admin/users/:id/unlock
func handleUnlockUser(c fiber.Ctx) error {
	id := c.Params("id")
	tag, err := db.Exec(c.Context(),
		"UPDATE auth.users SET failed_login_attempts = 0, locked_until = NULL, updated_at = now() WHERE id = $1",
		id)
	if err != nil {
		return apperrors.Internal("unlock user", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("user not found")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// POST /api/v1/admin/users/:id/verify
func handleVerifyEmail(c fiber.Ctx) error {
	id := c.Params("id")
	tag, err := db.Exec(c.Context(),
		"UPDATE auth.users SET email_verified = true, updated_at = now() WHERE id = $1",
		id)
	if err != nil {
		return apperrors.Internal("verify email", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("user not found")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/v1/admin/stats
func handleGetStats(c fiber.Ctx) error {
	var s SystemStats
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users").Scan(&s.TotalUsers)
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users WHERE created_at >= now() - interval '30 days'").Scan(&s.NewUsers30d)
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM transfer.transfers WHERE is_revoked = false").Scan(&s.TotalTransfers)
	_ = db.QueryRow(c.Context(), "SELECT count(*), coalesce(sum(size_bytes),0) FROM storage.files WHERE deleted_at IS NULL").
		Scan(&s.TotalFiles, &s.TotalStorageB)
	return c.JSON(s)
}

// transferRow is the shared shape for list and detail responses.
type transferRow struct {
	ID             string  `json:"id"`
	OwnerEmail     string  `json:"owner_email"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RecipientEmail string  `json:"recipient_email"`
	Slug           string  `json:"slug"`
	IsRevoked      bool    `json:"is_revoked"`
	ExpiresAt      *string `json:"expires_at"`
	DownloadCount  int     `json:"download_count"`
	MaxDownloads   *int    `json:"max_downloads"`
	CreatedAt      string  `json:"created_at"`
	Status         string  `json:"status"`
	HasPassword    bool    `json:"has_password"`
	FileCount      int     `json:"file_count"`
}

type transferFile struct {
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type transferDetail struct {
	transferRow
	Files []transferFile `json:"files"`
}

func scanTransferRow(r *transferRow, expiresAt *time.Time, maxDownloads *int, createdAt time.Time, passwordHash *string) {
	r.CreatedAt = createdAt.Format(time.RFC3339)
	if expiresAt != nil {
		s := expiresAt.Format(time.RFC3339)
		r.ExpiresAt = &s
	}
	r.MaxDownloads = maxDownloads
	r.HasPassword = passwordHash != nil && *passwordHash != ""
	if r.IsRevoked {
		r.Status = "revoked"
	} else if expiresAt != nil && expiresAt.Before(time.Now()) {
		r.Status = "expired"
	} else {
		r.Status = "active"
	}
}

// GET /api/v1/admin/transfers?limit=50&offset=0&status=all|active|expired|revoked
func handleListTransfers(c fiber.Ctx) error {
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
	status := c.Query("status", "all")

	where := ""
	switch status {
	case "active":
		where = "WHERE t.is_revoked = false AND (t.expires_at IS NULL OR t.expires_at > now())"
	case "expired":
		where = "WHERE t.is_revoked = false AND t.expires_at IS NOT NULL AND t.expires_at <= now()"
	case "revoked":
		where = "WHERE t.is_revoked = true"
	}

	query := `
		SELECT t.id, COALESCE(u.email, '—') AS owner_email, t.name,
		       COALESCE(t.description, '') AS description,
		       COALESCE(t.recipient_email, '') AS recipient_email,
		       t.slug, t.is_revoked, t.expires_at, t.download_count,
		       t.max_downloads, t.created_at, t.password_hash,
		       (SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id) AS file_count
		FROM transfer.transfers t
		LEFT JOIN auth.users u ON t.owner_id = u.id
		` + where + `
		ORDER BY t.created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := db.Query(c.Context(), query, limit, offset)
	if err != nil {
		return apperrors.Internal("failed to list transfers", err)
	}
	defer rows.Close()

	transfers := make([]transferRow, 0)
	for rows.Next() {
		var r transferRow
		var expiresAt *time.Time
		var maxDownloads *int
		var createdAt time.Time
		var passwordHash *string
		if err := rows.Scan(&r.ID, &r.OwnerEmail, &r.Name, &r.Description,
			&r.RecipientEmail, &r.Slug, &r.IsRevoked,
			&expiresAt, &r.DownloadCount, &maxDownloads, &createdAt, &passwordHash, &r.FileCount); err != nil {
			continue
		}
		scanTransferRow(&r, expiresAt, maxDownloads, createdAt, passwordHash)
		transfers = append(transfers, r)
	}

	var total int
	countQuery := `SELECT count(*) FROM transfer.transfers t LEFT JOIN auth.users u ON t.owner_id = u.id ` + where
	_ = db.QueryRow(c.Context(), countQuery).Scan(&total)

	return c.JSON(fiber.Map{"transfers": transfers, "total": total})
}

// GET /api/v1/admin/transfers/:id
func handleGetTransfer(c fiber.Ctx) error {
	id := c.Params("id")

	var r transferRow
	var expiresAt *time.Time
	var maxDownloads *int
	var createdAt time.Time
	var passwordHash *string
	err := db.QueryRow(c.Context(), `
		SELECT t.id, COALESCE(u.email, '—') AS owner_email, t.name,
		       COALESCE(t.description, '') AS description,
		       COALESCE(t.recipient_email, '') AS recipient_email,
		       t.slug, t.is_revoked, t.expires_at, t.download_count,
		       t.max_downloads, t.created_at, t.password_hash,
		       (SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id) AS file_count
		FROM transfer.transfers t
		LEFT JOIN auth.users u ON t.owner_id = u.id
		WHERE t.id = $1`, id,
	).Scan(&r.ID, &r.OwnerEmail, &r.Name, &r.Description,
		&r.RecipientEmail, &r.Slug, &r.IsRevoked,
		&expiresAt, &r.DownloadCount, &maxDownloads, &createdAt, &passwordHash, &r.FileCount)
	if err != nil {
		return apperrors.NotFound("transfer not found")
	}
	scanTransferRow(&r, expiresAt, maxDownloads, createdAt, passwordHash)

	// Fetch files via cross-schema join
	fileRows, err := db.Query(c.Context(), `
		SELECT f.id, f.filename, f.content_type, f.size_bytes
		FROM transfer.transfer_files tf
		JOIN storage.files f ON f.id = tf.file_id
		WHERE tf.transfer_id = $1 AND f.deleted_at IS NULL
		ORDER BY f.filename`, id)
	if err != nil {
		return apperrors.Internal("list transfer files", err)
	}
	defer fileRows.Close()

	files := make([]transferFile, 0)
	for fileRows.Next() {
		var f transferFile
		if err := fileRows.Scan(&f.FileID, &f.Filename, &f.ContentType, &f.SizeBytes); err != nil {
			continue
		}
		files = append(files, f)
	}

	return c.JSON(transferDetail{transferRow: r, Files: files})
}

// POST /api/v1/admin/transfers/:id/revoke
func handleRevokeTransfer(c fiber.Ctx) error {
	id := c.Params("id")
	tag, err := db.Exec(c.Context(),
		"UPDATE transfer.transfers SET is_revoked = true WHERE id = $1", id)
	if err != nil {
		return apperrors.Internal("revoke transfer", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("transfer not found")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/v1/admin/system/health
// Calls each service's unauthenticated /health endpoint directly on the internal
// Docker network to avoid auth overhead and the circular self-call problem.
func handleSystemHealth(c fiber.Ctx) error {
	type serviceInfo struct{ name, url string }
	services := []serviceInfo{
		{"Auth", "http://tenzoshare-auth:8081/health"},
		{"Transfer", "http://tenzoshare-transfer:8082/health"},
		{"Storage", "http://tenzoshare-storage:8083/health"},
		{"Upload", "http://tenzoshare-upload:8084/health"},
		{"Notification", "http://tenzoshare-notification:8085/health"},
		{"Audit", "http://tenzoshare-audit:8086/health"},
		{"Admin", "http://tenzoshare-admin:8087/health"},
	}

	client := &http.Client{Timeout: 3 * time.Second}

	results := make([]ServiceHealthItem, 0, len(services))
	for _, svc := range services {
		start := time.Now()
		resp, err := client.Get(svc.url)
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

// isUniqueViolation checks for PostgreSQL unique constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return len(err.Error()) > 5 && err.Error()[:5] == "ERROR" &&
		(contains(err.Error(), "23505") || contains(err.Error(), "unique"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
