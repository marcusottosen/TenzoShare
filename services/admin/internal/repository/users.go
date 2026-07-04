package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/models"
)

// ── User repository ──────────────────────────────────────────────────────────

type UserFilters struct {
	Search  string
	Role    string
	Limit   int
	Offset  int
	SortBy  string
	SortDir string
}

// ListUsers returns a paginated list of users with optional filtering.
func (r *Repository) ListUsers(ctx context.Context, filters UserFilters) ([]models.UserRow, int, error) {
	args := []any{}
	where := ""
	idx := 1

	add := func(cond string, val any) {
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += cond
		args = append(args, val)
		idx++
	}

	if filters.Search != "" {
		add("u.email ILIKE $"+itoa(idx), "%"+filters.Search+"%")
	}
	if filters.Role != "" {
		add("u.role = $"+itoa(idx), filters.Role)
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM auth.users u" + where
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	orderBy := userSortClause(filters.SortBy, filters.SortDir)
	dataSQL := "SELECT u.id, u.email, u.role, u.is_active, u.email_verified, COALESCE(m.is_enabled, false) AS mfa_enabled, u.failed_login_attempts, u.locked_until, u.last_login_at, u.created_at, u.updated_at FROM auth.users u LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id " +
		where + " ORDER BY " + orderBy + " LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	users := make([]models.UserRow, 0)
	for rows.Next() {
		var u models.UserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.IsActive, &u.EmailVerified, &u.MFAEnabled,
			&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return users, total, nil
}

// GetUserByID returns a single user by ID.
func (r *Repository) GetUserByID(ctx context.Context, userID string) (*models.UserRow, error) {
	var u models.UserRow
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.role, u.is_active, u.email_verified, 
		       COALESCE(m.is_enabled, false) AS mfa_enabled, 
		       u.failed_login_attempts, u.locked_until, u.last_login_at, 
		       u.created_at, u.updated_at
		FROM auth.users u
		LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(&u.ID, &u.Email, &u.Role, &u.IsActive, &u.EmailVerified, &u.MFAEnabled,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// GetUserByEmail returns a single user by email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.UserRow, error) {
	var u models.UserRow
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.role, u.is_active, u.email_verified, 
		       COALESCE(m.is_enabled, false) AS mfa_enabled, 
		       u.failed_login_attempts, u.locked_until, u.last_login_at, 
		       u.created_at, u.updated_at
		FROM auth.users u
		LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id
		WHERE u.email = $1
	`, email).Scan(&u.ID, &u.Email, &u.Role, &u.IsActive, &u.EmailVerified, &u.MFAEnabled,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// CreateUser inserts a new user with hashed password.
func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, role string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role, is_active, email_verified)
		VALUES ($1, $2, $3, true, false)
		RETURNING id
	`, email, passwordHash, role).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	return userID, nil
}

// UpdateUser updates user fields. Pass nil to skip updating a field.
func (r *Repository) UpdateUser(ctx context.Context, userID string, role *string, isActive *bool) error {
	updates := []string{}
	args := []any{}
	idx := 1

	if role != nil {
		updates = append(updates, fmt.Sprintf("role = $%d", idx))
		args = append(args, *role)
		idx++
	}
	if isActive != nil {
		updates = append(updates, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *isActive)
		idx++
	}

	if len(updates) == 0 {
		return nil
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", idx))
	args = append(args, time.Now())
	idx++

	args = append(args, userID)
	sql := fmt.Sprintf("UPDATE auth.users SET %s WHERE id = $%d", joinStr(updates, ", "), idx)

	if _, err := r.db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// DeleteUser removes a user and all related data.
func (r *Repository) DeleteUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM auth.users WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// UnlockUser clears failed login attempts and locked_until.
func (r *Repository) UnlockUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users 
		SET failed_login_attempts = 0, locked_until = NULL, updated_at = $1
		WHERE id = $2
	`, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("unlock user: %w", err)
	}
	return nil
}

// ResetUserMFA deletes the user's MFA secret.
func (r *Repository) ResetUserMFA(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM auth.mfa_secrets WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("reset user mfa: %w", err)
	}
	return nil
}

// VerifyUserEmail marks a user's email as verified.
func (r *Repository) VerifyUserEmail(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users 
		SET email_verified = true, updated_at = $1
		WHERE id = $2
	`, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("verify user email: %w", err)
	}
	return nil
}

// SetUserPassword updates a user's password hash.
func (r *Repository) SetUserPassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users 
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`, passwordHash, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("set user password: %w", err)
	}
	return nil
}

// ── User quota ───────────────────────────────────────────────────────────────

// ListUserQuotas returns all per-user quota overrides.
func (r *Repository) ListUserQuotas(ctx context.Context) ([]models.UserQuota, error) {
	rows, err := r.db.Query(ctx, `
		SELECT q.user_id, u.email, q.quota_bytes, q.updated_at, q.updated_by
		FROM admin.user_quotas q
		JOIN auth.users u ON u.id = q.user_id
		ORDER BY q.updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list user quotas: %w", err)
	}
	defer rows.Close()

	quotas := make([]models.UserQuota, 0)
	for rows.Next() {
		var q models.UserQuota
		if err := rows.Scan(&q.UserID, &q.Email, &q.QuotaBytes, &q.UpdatedAt, &q.UpdatedBy); err != nil {
			return nil, fmt.Errorf("scan quota: %w", err)
		}
		quotas = append(quotas, q)
	}

	return quotas, rows.Err()
}

// GetUserQuota returns a user's quota override (nil if none).
func (r *Repository) GetUserQuota(ctx context.Context, userID string) (*models.UserQuota, error) {
	var q models.UserQuota
	err := r.db.QueryRow(ctx, `
		SELECT q.user_id, u.email, q.quota_bytes, q.updated_at, q.updated_by
		FROM admin.user_quotas q
		JOIN auth.users u ON u.id = q.user_id
		WHERE q.user_id = $1
	`, userID).Scan(&q.UserID, &q.Email, &q.QuotaBytes, &q.UpdatedAt, &q.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("get user quota: %w", err)
	}
	return &q, nil
}

// SetUserQuota creates or updates a user's quota override.
func (r *Repository) SetUserQuota(ctx context.Context, userID string, quotaBytes *int64, adminID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO admin.user_quotas (user_id, quota_bytes, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET quota_bytes = EXCLUDED.quota_bytes, 
		    updated_at = NOW(), 
		    updated_by = EXCLUDED.updated_by
	`, userID, quotaBytes, adminID)
	if err != nil {
		return fmt.Errorf("set user quota: %w", err)
	}
	return nil
}

// DeleteUserQuota removes a user's quota override.
func (r *Repository) DeleteUserQuota(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM admin.user_quotas WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("delete user quota: %w", err)
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func userSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "email":
		return "u.email " + sortDir
	case "role":
		return "u.role " + sortDir
	case "is_active":
		return "u.is_active " + sortDir
	case "last_login_at":
		return "u.last_login_at " + sortDir + " NULLS LAST"
	default:
		return "u.created_at " + sortDir
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func joinStr(s []string, sep string) string {
	result := ""
	for i, str := range s {
		if i > 0 {
			result += sep
		}
		result += str
	}
	return result
}
