package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	LastLoginAt         *time.Time `json:"last_login_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type DayStat struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type StorageDayStat struct {
	Day   string `json:"day"`
	Bytes int64  `json:"bytes"`
}

type TransferBreakdown struct {
	Active    int `json:"active"`
	Exhausted int `json:"exhausted"`
	Expired   int `json:"expired"`
	Revoked   int `json:"revoked"`
}

type SystemStats struct {
	TotalUsers        int               `json:"total_users"`
	NewUsers30d       int               `json:"new_users_30d"`
	TotalTransfers    int               `json:"total_transfers"`
	TotalFiles        int               `json:"total_files"`
	TotalStorageB     int64             `json:"total_storage_bytes"`
	TransfersPerDay   []DayStat         `json:"transfers_per_day"`
	UsersPerDay       []DayStat         `json:"users_per_day"`
	StoragePerDay     []StorageDayStat  `json:"storage_per_day"`
	TransferBreakdown TransferBreakdown `json:"transfer_breakdown"`
}

type ServiceHealthItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// ── Storage Insights structs ──────────────────────────────────────────────────

type ContentTypeStat struct {
	ContentType string `json:"content_type"`
	Count       int64  `json:"count"`
	SizeBytes   int64  `json:"size_bytes"`
}

type PurgeReasonStat struct {
	Reason     string `json:"reason"`
	Count      int64  `json:"count"`
	FreedBytes int64  `json:"freed_bytes"`
}

type PurgeDayStat struct {
	Day        string `json:"day"`
	Count      int64  `json:"count"`
	FreedBytes int64  `json:"freed_bytes"`
}

type StorageInsights struct {
	TotalFiles           int64             `json:"total_files"`
	TotalStorageBytes    int64             `json:"total_storage_bytes"`
	DeletedFiles         int64             `json:"deleted_files"`
	PurgedFiles          int64             `json:"purged_files"`
	FreedBytes           int64             `json:"freed_bytes"`
	UniqueOwners         int64             `json:"unique_owners"`
	ContentTypeBreakdown []ContentTypeStat `json:"content_type_breakdown"`
	PurgeReasonBreakdown []PurgeReasonStat `json:"purge_reason_breakdown"`
	PurgePerDay          []PurgeDayStat    `json:"purge_per_day"`
	StoragePerDay        []StorageDayStat  `json:"storage_per_day"`
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
	v1.Get("/storage/usage", handleListStorageUsage)
	v1.Get("/storage/config", handleGetStorageConfig)
	v1.Put("/storage/config", handlePutStorageConfig)
	v1.Get("/storage/files", handleListStorageFiles)
	v1.Delete("/storage/files/:id", handleAdminDeleteFile)
	v1.Post("/storage/purge", handleTriggerPurge)
	v1.Get("/storage/purge-log", handleListPurgeLog)
	v1.Get("/storage/insights", handleStorageInsights)
	v1.Get("/transfers", handleListTransfers)
	v1.Get("/transfers/:id", handleGetTransfer)
	v1.Post("/transfers/:id/revoke", handleRevokeTransfer)
	v1.Get("/audit/config", handleGetAuditConfig)
	v1.Put("/audit/config", handlePutAuditConfig)
	v1.Post("/audit/purge", handleTriggerAuditPurge)
	v1.Get("/audit/stats", handleGetAuditStats)

	go func() {
		log.Info("admin service starting", zap.String("port", cfg.Server.Port))
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Error("server error", zap.Error(err))
		}
	}()

	// Background audit log purge — runs once at startup then daily.
	go func() {
		runAuditPurge(log)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runAuditPurge(log)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	log.Info("shutting down admin service")
	if err := app.Shutdown(); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// userSortClause returns a safe ORDER BY clause for the users table.
func userSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "email":
		return "email " + sortDir
	case "role":
		return "role " + sortDir
	case "is_active":
		return "is_active " + sortDir
	default:
		return "created_at " + sortDir
	}
}

// transferSortClause returns a safe ORDER BY clause for the transfers query.
func transferSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "owner_email":
		return "owner_email " + sortDir
	case "name":
		return "t.name " + sortDir
	case "download_count":
		return "t.download_count " + sortDir
	case "file_count":
		return "file_count " + sortDir
	case "total_size_bytes":
		return "total_size_bytes " + sortDir
	case "expires_at":
		return "t.expires_at " + sortDir + " NULLS LAST"
	case "status":
		return "CASE WHEN t.is_revoked THEN 2 WHEN t.expires_at IS NOT NULL AND t.expires_at <= now() THEN 1 ELSE 0 END " + sortDir
	default:
		return "t.created_at " + sortDir
	}
}

// GET /api/v1/admin/users?limit=50&offset=0&search=<email>&role=<role>&sort_by=<col>&sort_dir=asc|desc
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
	orderBy := userSortClause(c.Query("sort_by", "created_at"), c.Query("sort_dir", "desc"))

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

	dataSQL := "SELECT id, email, role, is_active, email_verified, failed_login_attempts, locked_until, last_login_at, created_at, updated_at FROM auth.users " +
		where + " ORDER BY " + orderBy + " LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
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
			&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
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

	// Scalar totals
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users").Scan(&s.TotalUsers)
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users WHERE created_at >= now() - interval '30 days'").Scan(&s.NewUsers30d)
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM transfer.transfers WHERE is_revoked = false").Scan(&s.TotalTransfers)
	_ = db.QueryRow(c.Context(), "SELECT count(*), coalesce(sum(size_bytes),0) FROM storage.files WHERE deleted_at IS NULL").
		Scan(&s.TotalFiles, &s.TotalStorageB)

	// Transfer status breakdown — uses per-file download counts table for accuracy
	_ = db.QueryRow(c.Context(), `
		SELECT
		    count(*) FILTER (WHERE
		        NOT t.is_revoked
		        AND (t.expires_at IS NULL OR t.expires_at > now())
		        AND (t.max_downloads = 0 OR t.max_downloads IS NULL OR EXISTS (
		            SELECT 1 FROM transfer.transfer_files tf
		            LEFT JOIN transfer.file_download_counts fdc
		                ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		            WHERE tf.transfer_id = t.id AND COALESCE(fdc.count, 0) < t.max_downloads
		        ))
		    ),
		    count(*) FILTER (WHERE
		        NOT t.is_revoked
		        AND t.max_downloads > 0
		        AND NOT EXISTS (
		            SELECT 1 FROM transfer.transfer_files tf
		            LEFT JOIN transfer.file_download_counts fdc
		                ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		            WHERE tf.transfer_id = t.id AND COALESCE(fdc.count, 0) < t.max_downloads
		        )
		    ),
		    count(*) FILTER (WHERE NOT t.is_revoked AND t.expires_at IS NOT NULL AND t.expires_at <= now()),
		    count(*) FILTER (WHERE t.is_revoked)
		FROM transfer.transfers t`,
	).Scan(&s.TransferBreakdown.Active, &s.TransferBreakdown.Exhausted, &s.TransferBreakdown.Expired, &s.TransferBreakdown.Revoked)

	// Transfers created per day — last 14 days
	s.TransfersPerDay = make([]DayStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT to_char(date_trunc('day', created_at), 'Mon DD') as day, count(*)
		FROM transfer.transfers
		WHERE created_at >= now() - interval '14 days'
		GROUP BY date_trunc('day', created_at), day
		ORDER BY date_trunc('day', created_at)`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var d DayStat
			if err := rows.Scan(&d.Day, &d.Count); err == nil {
				s.TransfersPerDay = append(s.TransfersPerDay, d)
			}
		}
	}

	// New users per day — last 14 days
	s.UsersPerDay = make([]DayStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT to_char(date_trunc('day', created_at), 'Mon DD') as day, count(*)
		FROM auth.users
		WHERE created_at >= now() - interval '14 days'
		GROUP BY date_trunc('day', created_at), day
		ORDER BY date_trunc('day', created_at)`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var d DayStat
			if err := rows.Scan(&d.Day, &d.Count); err == nil {
				s.UsersPerDay = append(s.UsersPerDay, d)
			}
		}
	}

	// Storage added per day — last 14 days (running bytes uploaded)
	s.StoragePerDay = make([]StorageDayStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT to_char(date_trunc('day', created_at), 'Mon DD') as day, coalesce(sum(size_bytes), 0)
		FROM storage.files
		WHERE created_at >= now() - interval '14 days' AND deleted_at IS NULL
		GROUP BY date_trunc('day', created_at), day
		ORDER BY date_trunc('day', created_at)`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var d StorageDayStat
			if err := rows.Scan(&d.Day, &d.Bytes); err == nil {
				s.StoragePerDay = append(s.StoragePerDay, d)
			}
		}
	}

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
	TotalSizeBytes int64   `json:"total_size_bytes"`
	IsExhausted    bool    `json:"-"` // populated by DB subquery; not exposed directly
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
	} else if r.IsExhausted {
		r.Status = "exhausted"
	} else if expiresAt != nil && expiresAt.Before(time.Now()) {
		r.Status = "expired"
	} else {
		r.Status = "active"
	}
}

// GET /api/v1/admin/transfers?limit=50&offset=0&status=all|active|expired|revoked&sort_by=<col>&sort_dir=asc|desc
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
		where = `WHERE t.is_revoked = false AND (t.expires_at IS NULL OR t.expires_at > now())
		  AND (t.max_downloads = 0 OR t.max_downloads IS NULL OR EXISTS (
		      SELECT 1 FROM transfer.transfer_files tf
		      LEFT JOIN transfer.file_download_counts fdc
		          ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		      WHERE tf.transfer_id = t.id AND COALESCE(fdc.count, 0) < t.max_downloads
		  ))`
	case "exhausted":
		where = `WHERE t.is_revoked = false AND t.max_downloads > 0
		  AND NOT EXISTS (
		      SELECT 1 FROM transfer.transfer_files tf
		      LEFT JOIN transfer.file_download_counts fdc
		          ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		      WHERE tf.transfer_id = t.id AND COALESCE(fdc.count, 0) < t.max_downloads
		  )`
	case "expired":
		where = "WHERE t.is_revoked = false AND t.expires_at IS NOT NULL AND t.expires_at <= now()"
	case "revoked":
		where = "WHERE t.is_revoked = true"
	}

	transferOrderBy := transferSortClause(c.Query("sort_by", "created_at"), c.Query("sort_dir", "desc"))

	query := `
		SELECT t.id, COALESCE(u.email, '—') AS owner_email, t.name,
		       COALESCE(t.description, '') AS description,
		       COALESCE(t.recipient_email, '') AS recipient_email,
		       t.slug, t.is_revoked, t.expires_at, t.download_count,
		       t.max_downloads, t.created_at, t.password_hash,
		       (SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id) AS file_count,
		       COALESCE((SELECT sum(f.size_bytes)
		                 FROM transfer.transfer_files tf
		                 JOIN storage.files f ON f.id = tf.file_id
		                 WHERE tf.transfer_id = t.id AND f.deleted_at IS NULL), 0) AS total_size_bytes,
		       (t.max_downloads > 0 AND NOT EXISTS (
		           SELECT 1 FROM transfer.transfer_files tf
		           LEFT JOIN transfer.file_download_counts fdc
		               ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		           WHERE tf.transfer_id = t.id AND COALESCE(fdc.count, 0) < t.max_downloads
		       )) AS is_exhausted
		FROM transfer.transfers t
		LEFT JOIN auth.users u ON t.owner_id = u.id
		` + where + `
		ORDER BY ` + transferOrderBy + `
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
			&expiresAt, &r.DownloadCount, &maxDownloads, &createdAt, &passwordHash, &r.FileCount, &r.TotalSizeBytes, &r.IsExhausted); err != nil {
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
		       (SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id) AS file_count,
		       COALESCE((SELECT sum(f.size_bytes)
		                 FROM transfer.transfer_files tf
		                 JOIN storage.files f ON f.id = tf.file_id
		                 WHERE tf.transfer_id = t.id AND f.deleted_at IS NULL), 0) AS total_size_bytes,
		       (t.max_downloads > 0 AND NOT EXISTS (
		           SELECT 1 FROM transfer.transfer_files tf
		           LEFT JOIN transfer.file_download_counts fdc
		               ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		           WHERE tf.transfer_id = t.id AND COALESCE(fdc.count, 0) < t.max_downloads
		       )) AS is_exhausted
		FROM transfer.transfers t
		LEFT JOIN auth.users u ON t.owner_id = u.id
		WHERE t.id = $1`, id,
	).Scan(&r.ID, &r.OwnerEmail, &r.Name, &r.Description,
		&r.RecipientEmail, &r.Slug, &r.IsRevoked,
		&expiresAt, &r.DownloadCount, &maxDownloads, &createdAt, &passwordHash, &r.FileCount, &r.TotalSizeBytes, &r.IsExhausted)
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

// ── Storage usage ─────────────────────────────────────────────────────────────

// StorageUserUsage is the per-user shape returned by the admin storage usage endpoint.
type StorageUserUsage struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	FileCount  int64  `json:"file_count"`
	TotalBytes int64  `json:"total_bytes"`
}

// storageUsageSortClause returns a safe ORDER BY expression for the storage usage query.
func storageUsageSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "email":
		return "u.email " + sortDir
	case "file_count":
		return "file_count " + sortDir
	default: // total_bytes
		return "total_bytes " + sortDir
	}
}

// GET /api/v1/admin/storage/usage?limit=50&offset=0&sort_by=total_bytes&sort_dir=desc
// Returns all users with their aggregated file count and total storage in bytes.
// Users with no files are included (showing 0 for both counters).
func handleListStorageUsage(c fiber.Ctx) error {
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

	orderBy := storageUsageSortClause(c.Query("sort_by", "total_bytes"), c.Query("sort_dir", "desc"))

	rows, err := db.Query(c.Context(), `
		SELECT u.id,
		       u.email,
		       count(f.id)                    AS file_count,
		       coalesce(sum(f.size_bytes), 0) AS total_bytes
		FROM auth.users u
		LEFT JOIN storage.files f ON f.owner_id = u.id AND f.deleted_at IS NULL
		GROUP BY u.id, u.email
		ORDER BY `+orderBy+`
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return apperrors.Internal("list storage usage", err)
	}
	defer rows.Close()

	usage := make([]StorageUserUsage, 0)
	for rows.Next() {
		var u StorageUserUsage
		if err := rows.Scan(&u.UserID, &u.Email, &u.FileCount, &u.TotalBytes); err != nil {
			return apperrors.Internal("scan storage usage row", err)
		}
		usage = append(usage, u)
	}
	if err := rows.Err(); err != nil {
		return apperrors.Internal("iterate storage usage", err)
	}

	var total int
	_ = db.QueryRow(c.Context(), "SELECT count(*) FROM auth.users").Scan(&total)

	return c.JSON(fiber.Map{
		"usage":  usage,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
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

// ── Storage config ───────────────────────────────────────────────────────────

// StorageConfig is the singleton storage policy row.
type StorageConfig struct {
	QuotaEnabled        bool   `json:"quota_enabled"`
	QuotaBytesPerUser   int64  `json:"quota_bytes_per_user"`
	MaxUploadSizeBytes  int64  `json:"max_upload_size_bytes"`
	RetentionEnabled    bool   `json:"retention_enabled"`
	RetentionDays       int    `json:"retention_days"`
	OrphanRetentionDays int    `json:"orphan_retention_days"`
	UpdatedAt           string `json:"updated_at"`
	UpdatedBy           string `json:"updated_by"`
}

// GET /api/v1/admin/storage/config
func handleGetStorageConfig(c fiber.Ctx) error {
	var sc StorageConfig
	var updatedAt time.Time
	err := db.QueryRow(c.Context(), `
		SELECT quota_enabled, quota_bytes_per_user, max_upload_size_bytes,
		       retention_enabled, retention_days, orphan_retention_days,
		       updated_at, updated_by
		FROM storage.storage_settings WHERE id = 1`,
	).Scan(&sc.QuotaEnabled, &sc.QuotaBytesPerUser, &sc.MaxUploadSizeBytes,
		&sc.RetentionEnabled, &sc.RetentionDays, &sc.OrphanRetentionDays,
		&updatedAt, &sc.UpdatedBy)
	if err != nil {
		return apperrors.Internal("get storage config", err)
	}
	sc.UpdatedAt = updatedAt.Format(time.RFC3339)
	return c.JSON(sc)
}

// PUT /api/v1/admin/storage/config — PATCH-style: only fields present in body are updated
func handlePutStorageConfig(c fiber.Ctx) error {
	var body struct {
		QuotaEnabled        *bool  `json:"quota_enabled"`
		QuotaBytesPerUser   *int64 `json:"quota_bytes_per_user"`
		MaxUploadSizeBytes  *int64 `json:"max_upload_size_bytes"`
		RetentionEnabled    *bool  `json:"retention_enabled"`
		RetentionDays       *int   `json:"retention_days"`
		OrphanRetentionDays *int   `json:"orphan_retention_days"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return apperrors.BadRequest("invalid JSON body")
	}

	if body.QuotaBytesPerUser != nil && *body.QuotaBytesPerUser < 0 {
		return apperrors.BadRequest("quota_bytes_per_user must be >= 0")
	}
	if body.MaxUploadSizeBytes != nil && *body.MaxUploadSizeBytes < 0 {
		return apperrors.BadRequest("max_upload_size_bytes must be >= 0")
	}
	if body.RetentionDays != nil && *body.RetentionDays < 1 {
		return apperrors.BadRequest("retention_days must be >= 1")
	}
	if body.OrphanRetentionDays != nil && *body.OrphanRetentionDays < 1 {
		return apperrors.BadRequest("orphan_retention_days must be >= 1")
	}

	callerID, _ := c.Locals("userID").(string)
	callerEmail := callerID
	if callerID != "" {
		_ = db.QueryRow(c.Context(), "SELECT email FROM auth.users WHERE id = $1", callerID).Scan(&callerEmail)
	}

	_, err := db.Exec(c.Context(), `
		UPDATE storage.storage_settings SET
		    quota_enabled          = COALESCE($1, quota_enabled),
		    quota_bytes_per_user   = COALESCE($2, quota_bytes_per_user),
		    max_upload_size_bytes  = COALESCE($3, max_upload_size_bytes),
		    retention_enabled      = COALESCE($4, retention_enabled),
		    retention_days         = COALESCE($5, retention_days),
		    orphan_retention_days  = COALESCE($6, orphan_retention_days),
		    updated_at             = now(),
		    updated_by             = $7
		WHERE id = 1`,
		body.QuotaEnabled, body.QuotaBytesPerUser, body.MaxUploadSizeBytes,
		body.RetentionEnabled, body.RetentionDays, body.OrphanRetentionDays,
		callerEmail,
	)
	if err != nil {
		return apperrors.Internal("update storage config", err)
	}
	return handleGetStorageConfig(c)
}

// ── Storage file management ───────────────────────────────────────────────────

// AdminFileRow represents a file record with share/retention context.
type AdminFileRow struct {
	ID             string  `json:"id"`
	OwnerID        string  `json:"owner_id"`
	OwnerEmail     string  `json:"owner_email"`
	Filename       string  `json:"filename"`
	ContentType    string  `json:"content_type"`
	SizeBytes      int64   `json:"size_bytes"`
	CreatedAt      string  `json:"created_at"`
	ShareCount     int     `json:"share_count"`
	ActiveShares   int     `json:"active_shares"`
	LastShareExpAt *string `json:"last_share_expires_at"`
	EligiblePurge  bool    `json:"eligible_purge"`
}

// GET /api/v1/admin/storage/files?limit=50&offset=0&sort_by=size_bytes|created_at&sort_dir=asc|desc&filter=all|orphan|eligible
func handleListStorageFiles(c fiber.Ctx) error {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, _ := strconv.Atoi(v); n >= 0 {
			offset = n
		}
	}

	sortCols := map[string]string{
		"size_bytes": "f.size_bytes",
		"created_at": "f.created_at",
		"filename":   "f.filename",
		"owner":      "u.email",
		"shares":     "coalesce(fs.share_count,0)",
	}
	sortCol := "f.created_at"
	if v, ok := sortCols[c.Query("sort_by")]; ok {
		sortCol = v
	}
	sortDir := "DESC"
	if c.Query("sort_dir") == "asc" {
		sortDir = "ASC"
	}

	filter := c.Query("filter") // "all" | "orphan" | "eligible"

	// Fetch retention settings to determine eligibility
	var retEnabled bool
	var retDays, orphDays int
	_ = db.QueryRow(c.Context(),
		`SELECT retention_enabled, retention_days, orphan_retention_days FROM storage.storage_settings WHERE id = 1`,
	).Scan(&retEnabled, &retDays, &orphDays)
	if retDays <= 0 {
		retDays = 30
	}
	if orphDays <= 0 {
		orphDays = 90
	}

	whereClause := "WHERE f.deleted_at IS NULL"
	switch filter {
	case "orphan":
		whereClause += " AND coalesce(fs.share_count,0) = 0"
	case "eligible":
		whereClause += fmt.Sprintf(` AND (
			(coalesce(fs.share_count,0) > 0 AND coalesce(fs.active_shares,0) = 0
			 AND coalesce(fs.last_exp, now() - interval '1 second') < now() - interval '%d days')
			OR (coalesce(fs.share_count,0) = 0 AND f.created_at < now() - interval '%d days')
		)`, retDays, orphDays)
	}

	query := fmt.Sprintf(`
		WITH file_shares AS (
		    SELECT tf.file_id,
		           count(*) AS share_count,
		           count(*) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > now())) AS active_shares,
		           max(t.expires_at) AS last_exp
		    FROM transfer.transfer_files tf
		    JOIN transfer.transfers t ON t.id = tf.transfer_id
		    GROUP BY tf.file_id
		)
		SELECT f.id, f.owner_id, coalesce(u.email,''), f.filename, f.content_type, f.size_bytes, f.created_at,
		       coalesce(fs.share_count,0), coalesce(fs.active_shares,0), fs.last_exp,
		       -- eligible_purge: no active shares AND last expiry > retDays OR orphan > orphDays
		       CASE
		           WHEN coalesce(fs.share_count,0) > 0
		                AND coalesce(fs.active_shares,0) = 0
		                AND coalesce(fs.last_exp, now() - interval '1 second') < now() - interval '%d days'
		                THEN true
		           WHEN coalesce(fs.share_count,0) = 0
		                AND f.created_at < now() - interval '%d days'
		                THEN true
		           ELSE false
		       END AS eligible_purge
		FROM storage.files f
		LEFT JOIN auth.users u ON u.id = f.owner_id::uuid
		LEFT JOIN file_shares fs ON fs.file_id::uuid = f.id
		%s
		ORDER BY %s %s
		LIMIT %d OFFSET %d
	`, retDays, orphDays, whereClause, sortCol, sortDir, limit, offset)

	rows, err := db.Query(c.Context(), query)
	if err != nil {
		return apperrors.Internal("list storage files", err)
	}
	defer rows.Close()

	var files []AdminFileRow
	for rows.Next() {
		var row AdminFileRow
		var createdAt time.Time
		var lastExp *time.Time
		if err := rows.Scan(
			&row.ID, &row.OwnerID, &row.OwnerEmail, &row.Filename, &row.ContentType, &row.SizeBytes,
			&createdAt, &row.ShareCount, &row.ActiveShares, &lastExp, &row.EligiblePurge,
		); err != nil {
			return apperrors.Internal("scan storage file row", err)
		}
		row.CreatedAt = createdAt.Format(time.RFC3339)
		if lastExp != nil {
			s := lastExp.Format(time.RFC3339)
			row.LastShareExpAt = &s
		}
		files = append(files, row)
	}
	if err := rows.Err(); err != nil {
		return apperrors.Internal("iterate storage files", err)
	}

	// Total count for pagination
	countQuery := fmt.Sprintf(`
		WITH file_shares AS (
		    SELECT tf.file_id,
		           count(*) AS share_count,
		           count(*) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > now())) AS active_shares,
		           max(t.expires_at) AS last_exp
		    FROM transfer.transfer_files tf
		    JOIN transfer.transfers t ON t.id = tf.transfer_id
		    GROUP BY tf.file_id
		)
		SELECT count(*) FROM storage.files f
		LEFT JOIN file_shares fs ON fs.file_id::uuid = f.id
		%s
	`, whereClause)
	var total int
	_ = db.QueryRow(c.Context(), countQuery).Scan(&total)

	return c.JSON(fiber.Map{"files": files, "total": total, "limit": limit, "offset": offset})
}

// DELETE /api/v1/admin/storage/files/:id — admin force-delete a file
func handleAdminDeleteFile(c fiber.Ctx) error {
	id := c.Params("id")

	// Get file metadata before deleting (for the purge log)
	var objectKey, ownerID, filename string
	var sizeBytes int64
	err := db.QueryRow(c.Context(),
		`SELECT object_key, owner_id, filename, size_bytes FROM storage.files WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&objectKey, &ownerID, &filename, &sizeBytes)
	if err != nil {
		return apperrors.NotFound("file not found")
	}

	callerID, _ := c.Locals("userID").(string)
	callerEmail := callerID
	if callerID != "" {
		_ = db.QueryRow(c.Context(), "SELECT email FROM auth.users WHERE id = $1", callerID).Scan(&callerEmail)
	}

	// Soft-delete the DB record
	_, err = db.Exec(c.Context(), `UPDATE storage.files SET deleted_at = now() WHERE id = $1`, id)
	if err != nil {
		return apperrors.Internal("delete file record", err)
	}

	// Log to purge audit table
	_, _ = db.Exec(c.Context(),
		`INSERT INTO storage.file_purge_log (file_id, owner_id, filename, size_bytes, reason, purged_by)
		 VALUES ($1, $2, $3, $4, 'admin_purge', $5)`,
		id, ownerID, filename, sizeBytes, callerEmail,
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// maxFilesPerPurge is the hard safety cap on the number of files a single manual
// purge request can delete. Prevents accidental mass-deletion from a bad query.
const maxFilesPerPurge = 500

// POST /api/v1/admin/storage/purge — trigger retention purge immediately
func handleTriggerPurge(c fiber.Ctx) error {
	var retDays, orphDays int
	err := db.QueryRow(c.Context(),
		`SELECT retention_days, orphan_retention_days FROM storage.storage_settings WHERE id = 1`,
	).Scan(&retDays, &orphDays)
	if err != nil {
		return apperrors.Internal("get storage config", err)
	}
	if retDays <= 0 {
		retDays = 30
	}
	if orphDays <= 0 {
		orphDays = 90
	}

	callerID, _ := c.Locals("userID").(string)
	callerEmail := callerID
	if callerID != "" {
		_ = db.QueryRow(c.Context(), "SELECT email FROM auth.users WHERE id = $1", callerID).Scan(&callerEmail)
	}

	// Find eligible files and soft-delete them (object deletion happens in storage service worker)
	rows, err := db.Query(c.Context(), fmt.Sprintf(`
		WITH shared_files AS (
		    SELECT DISTINCT tf.file_id FROM transfer.transfer_files tf
		),
		last_share_expiry AS (
		    SELECT tf.file_id,
		           bool_and(t.is_revoked OR (t.expires_at IS NOT NULL AND t.expires_at < now())) AS all_done,
		           max(COALESCE(t.expires_at, now() - interval '1 second')) AS latest_expiry
		    FROM transfer.transfer_files tf
		    JOIN transfer.transfers t ON t.id = tf.transfer_id
		    GROUP BY tf.file_id
		)
		SELECT f.id, f.owner_id, f.filename, f.size_bytes, 'retention_expired' AS reason
		FROM storage.files f
		JOIN last_share_expiry lse ON lse.file_id::uuid = f.id
		WHERE f.deleted_at IS NULL AND lse.all_done = true
		  AND lse.latest_expiry < now() - interval '%d days'

		UNION ALL

		SELECT f.id, f.owner_id, f.filename, f.size_bytes, 'orphan_expired' AS reason
		FROM storage.files f
		WHERE f.deleted_at IS NULL
		  AND f.id NOT IN (SELECT file_id FROM shared_files)
		  AND f.created_at < now() - interval '%d days'
	`, retDays, orphDays))
	if err != nil {
		return apperrors.Internal("find eligible files", err)
	}
	defer rows.Close()

	type purgeCandidate struct {
		id, ownerID, filename, reason string
		sizeBytes                     int64
	}
	var candidates []purgeCandidate
	for rows.Next() {
		var p purgeCandidate
		if err := rows.Scan(&p.id, &p.ownerID, &p.filename, &p.sizeBytes, &p.reason); err != nil {
			continue
		}
		candidates = append(candidates, p)
	}

	// Safety cap: never delete more than maxFilesPerPurge files in a single request.
	capped := false
	if len(candidates) > maxFilesPerPurge {
		capped = true
		candidates = candidates[:maxFilesPerPurge]
	}

	deleted := 0
	var freedBytes int64
	for _, p := range candidates {
		tag, err := db.Exec(c.Context(), `UPDATE storage.files SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, p.id)
		if err != nil || tag.RowsAffected() == 0 {
			continue
		}
		_, _ = db.Exec(c.Context(),
			`INSERT INTO storage.file_purge_log (file_id, owner_id, filename, size_bytes, reason, purged_by)
                         VALUES ($1, $2, $3, $4, $5, $6)`,
			p.id, p.ownerID, p.filename, p.sizeBytes, p.reason, callerEmail,
		)
		deleted++
		freedBytes += p.sizeBytes
	}

	return c.JSON(fiber.Map{
		"deleted":     deleted,
		"freed_bytes": freedBytes,
		"capped":      capped,
		"cap":         maxFilesPerPurge,
	})
}

// GET /api/v1/admin/storage/insights
// Returns aggregated storage statistics for the insights dashboard.
func handleStorageInsights(c fiber.Ctx) error {
	var s StorageInsights

	// Scalar totals
	_ = db.QueryRow(c.Context(), `
		SELECT
			count(*) FILTER (WHERE deleted_at IS NULL),
			coalesce(sum(size_bytes) FILTER (WHERE deleted_at IS NULL), 0),
			count(*) FILTER (WHERE deleted_at IS NOT NULL),
			count(DISTINCT owner_id) FILTER (WHERE deleted_at IS NULL)
		FROM storage.files`,
	).Scan(&s.TotalFiles, &s.TotalStorageBytes, &s.DeletedFiles, &s.UniqueOwners)

	_ = db.QueryRow(c.Context(), `
		SELECT count(*), coalesce(sum(size_bytes), 0)
		FROM storage.file_purge_log`,
	).Scan(&s.PurgedFiles, &s.FreedBytes)

	// Content-type breakdown (top 12 by size)
	s.ContentTypeBreakdown = make([]ContentTypeStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT content_type, count(*), coalesce(sum(size_bytes), 0)
		FROM storage.files
		WHERE deleted_at IS NULL
		GROUP BY content_type
		ORDER BY sum(size_bytes) DESC
		LIMIT 12`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var ct ContentTypeStat
			if err := rows.Scan(&ct.ContentType, &ct.Count, &ct.SizeBytes); err == nil {
				s.ContentTypeBreakdown = append(s.ContentTypeBreakdown, ct)
			}
		}
	}

	// Purge reason breakdown
	s.PurgeReasonBreakdown = make([]PurgeReasonStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT reason, count(*), coalesce(sum(size_bytes), 0)
		FROM storage.file_purge_log
		GROUP BY reason
		ORDER BY count(*) DESC`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var pr PurgeReasonStat
			if err := rows.Scan(&pr.Reason, &pr.Count, &pr.FreedBytes); err == nil {
				s.PurgeReasonBreakdown = append(s.PurgeReasonBreakdown, pr)
			}
		}
	}

	// Purge activity per day — last 30 days
	s.PurgePerDay = make([]PurgeDayStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT to_char(date_trunc('day', purged_at), 'Mon DD') as day,
		       count(*), coalesce(sum(size_bytes), 0)
		FROM storage.file_purge_log
		WHERE purged_at >= now() - interval '30 days'
		GROUP BY date_trunc('day', purged_at), day
		ORDER BY date_trunc('day', purged_at)`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var pd PurgeDayStat
			if err := rows.Scan(&pd.Day, &pd.Count, &pd.FreedBytes); err == nil {
				s.PurgePerDay = append(s.PurgePerDay, pd)
			}
		}
	}

	// Storage added per day — last 30 days
	s.StoragePerDay = make([]StorageDayStat, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT to_char(date_trunc('day', created_at), 'Mon DD') as day,
		       coalesce(sum(size_bytes), 0)
		FROM storage.files
		WHERE created_at >= now() - interval '30 days'
		GROUP BY date_trunc('day', created_at), day
		ORDER BY date_trunc('day', created_at)`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var sd StorageDayStat
			if err := rows.Scan(&sd.Day, &sd.Bytes); err == nil {
				s.StoragePerDay = append(s.StoragePerDay, sd)
			}
		}
	}

	return c.JSON(s)
}

// GET /api/v1/admin/storage/purge-log?limit=50&offset=0
func handleListPurgeLog(c fiber.Ctx) error {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, _ := strconv.Atoi(v); n >= 0 {
			offset = n
		}
	}

	rows, err := db.Query(c.Context(), `
		SELECT pl.file_id, pl.owner_id, coalesce(u.email, pl.owner_id::text), pl.filename, pl.size_bytes, pl.reason, pl.purged_by, pl.purged_at
		FROM storage.file_purge_log pl
		LEFT JOIN auth.users u ON u.id = pl.owner_id
		ORDER BY pl.purged_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return apperrors.Internal("list purge log", err)
	}
	defer rows.Close()

	type logRow struct {
		FileID    string `json:"file_id"`
		OwnerID   string `json:"owner_id"`
		Email     string `json:"email"`
		Filename  string `json:"filename"`
		SizeBytes int64  `json:"size_bytes"`
		Reason    string `json:"reason"`
		PurgedBy  string `json:"purged_by"`
		PurgedAt  string `json:"purged_at"`
	}
	var entries []logRow
	for rows.Next() {
		var e logRow
		var purgedAt time.Time
		if err := rows.Scan(&e.FileID, &e.OwnerID, &e.Email, &e.Filename, &e.SizeBytes, &e.Reason, &e.PurgedBy, &purgedAt); err != nil {
			return apperrors.Internal("scan purge log row", err)
		}
		e.PurgedAt = purgedAt.Format(time.RFC3339)
		entries = append(entries, e)
	}

	var total int
	_ = db.QueryRow(c.Context(), `SELECT count(*) FROM storage.file_purge_log`).Scan(&total)

	return c.JSON(fiber.Map{"entries": entries, "total": total, "limit": limit, "offset": offset})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ── Audit config ─────────────────────────────────────────────────────────────

type AuditConfig struct {
	RetentionEnabled bool   `json:"retention_enabled"`
	RetentionDays    int    `json:"retention_days"`
	UpdatedAt        string `json:"updated_at"`
	UpdatedBy        string `json:"updated_by"`
}

// GET /api/v1/admin/audit/config
func handleGetAuditConfig(c fiber.Ctx) error {
	var ac AuditConfig
	var updatedAt time.Time
	err := db.QueryRow(c.Context(), `
		SELECT retention_enabled, retention_days, updated_at, updated_by
		FROM audit.audit_settings WHERE id = 1`,
	).Scan(&ac.RetentionEnabled, &ac.RetentionDays, &updatedAt, &ac.UpdatedBy)
	if err != nil {
		return apperrors.Internal("get audit config", err)
	}
	ac.UpdatedAt = updatedAt.Format(time.RFC3339)
	return c.JSON(ac)
}

// PUT /api/v1/admin/audit/config
func handlePutAuditConfig(c fiber.Ctx) error {
	var body struct {
		RetentionEnabled *bool `json:"retention_enabled"`
		RetentionDays    *int  `json:"retention_days"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return apperrors.BadRequest("invalid JSON body")
	}
	if body.RetentionDays != nil && *body.RetentionDays < 1 {
		return apperrors.BadRequest("retention_days must be >= 1")
	}

	callerID, _ := c.Locals("userID").(string)
	callerEmail := callerID
	if callerID != "" {
		_ = db.QueryRow(c.Context(), "SELECT email FROM auth.users WHERE id = $1", callerID).Scan(&callerEmail)
	}

	_, err := db.Exec(c.Context(), `
		UPDATE audit.audit_settings SET
		    retention_enabled = COALESCE($1, retention_enabled),
		    retention_days    = COALESCE($2, retention_days),
		    updated_at        = now(),
		    updated_by        = $3
		WHERE id = 1`,
		body.RetentionEnabled, body.RetentionDays, callerEmail,
	)
	if err != nil {
		return apperrors.Internal("update audit config", err)
	}
	return handleGetAuditConfig(c)
}

// GET /api/v1/admin/audit/stats
func handleGetAuditStats(c fiber.Ctx) error {
	var total int64
	var oldest, newest *time.Time
	_ = db.QueryRow(c.Context(), `
		SELECT count(*), min(created_at), max(created_at)
		FROM audit.audit_logs`,
	).Scan(&total, &oldest, &newest)

	type sourceCount struct {
		Source string `json:"source"`
		Count  int64  `json:"count"`
	}
	sources := make([]sourceCount, 0)
	if rows, err := db.Query(c.Context(), `
		SELECT source, count(*) FROM audit.audit_logs
		GROUP BY source ORDER BY count(*) DESC`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var sc sourceCount
			if err := rows.Scan(&sc.Source, &sc.Count); err == nil {
				sources = append(sources, sc)
			}
		}
	}

	oldestStr := ""
	if oldest != nil {
		oldestStr = oldest.Format(time.RFC3339)
	}
	newestStr := ""
	if newest != nil {
		newestStr = newest.Format(time.RFC3339)
	}

	return c.JSON(fiber.Map{
		"total_entries": total,
		"oldest_entry":  oldestStr,
		"newest_entry":  newestStr,
		"by_source":     sources,
	})
}

// POST /api/v1/admin/audit/purge — manually trigger audit log purge
func handleTriggerAuditPurge(c fiber.Ctx) error {
	var retDays int
	var enabled bool
	err := db.QueryRow(c.Context(),
		`SELECT retention_enabled, retention_days FROM audit.audit_settings WHERE id = 1`,
	).Scan(&enabled, &retDays)
	if err != nil {
		return apperrors.Internal("get audit config", err)
	}
	if !enabled {
		return c.JSON(fiber.Map{"deleted": 0, "message": "retention is disabled"})
	}
	if retDays < 1 {
		retDays = 365
	}
	tag, err := db.Exec(c.Context(),
		`DELETE FROM audit.audit_logs WHERE created_at < now() - ($1 || ' days')::interval`,
		retDays,
	)
	if err != nil {
		return apperrors.Internal("purge audit logs", err)
	}
	return c.JSON(fiber.Map{"deleted": tag.RowsAffected(), "retention_days": retDays})
}

// runAuditPurge is called from the background goroutine — best-effort, never panics.
func runAuditPurge(log *zap.Logger) {
	if db == nil {
		return
	}
	var enabled bool
	var retDays int
	err := db.QueryRow(context.Background(),
		`SELECT retention_enabled, retention_days FROM audit.audit_settings WHERE id = 1`,
	).Scan(&enabled, &retDays)
	if err != nil || !enabled || retDays < 1 {
		return
	}
	tag, err := db.Exec(context.Background(),
		`DELETE FROM audit.audit_logs WHERE created_at < now() - ($1 || ' days')::interval`,
		retDays,
	)
	if err != nil {
		log.Warn("audit purge failed", zap.Error(err))
		return
	}
	if tag.RowsAffected() > 0 {
		log.Info("audit purge complete", zap.Int64("deleted", tag.RowsAffected()), zap.Int("retention_days", retDays))
	}
}

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
