package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string, role domain.Role) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, role, is_active, email_verified, created_at, updated_at
	`, email, passwordHash, string(role)).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.Conflict("email already registered")
		}
		return nil, apperrors.Internal("create user", err)
	}
	return &u, nil
}

func (r *UserRepository) UpsertAdminByEmail(ctx context.Context, email, passwordHash string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role, is_active, email_verified)
		VALUES ($1, $2, $3, true, true)
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    role = EXCLUDED.role,
		    is_active = true,
		    email_verified = true,
		    updated_at = now()
		RETURNING id, email, password_hash, role, is_active, email_verified, created_at, updated_at
	`, email, passwordHash, string(domain.RoleAdmin)).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.Internal("upsert bootstrap admin", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.role, u.is_active, u.email_verified,
		       u.created_at, u.updated_at,
		       COALESCE(m.is_enabled, false) AS mfa_enabled,
		       u.failed_login_attempts, u.locked_until
		FROM auth.users u
		LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id
		WHERE u.email = $1
	`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
		&u.MFAEnabled, &u.FailedLoginAttempts, &u.LockedUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user not found")
		}
		return nil, apperrors.Internal("get user by email", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.role, u.is_active, u.email_verified,
		       u.created_at, u.updated_at,
		       COALESCE(m.is_enabled, false) AS mfa_enabled,
		       u.failed_login_attempts, u.locked_until
		FROM auth.users u
		LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id
		WHERE u.id = $1
	`, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
		&u.MFAEnabled, &u.FailedLoginAttempts, &u.LockedUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user not found")
		}
		return nil, apperrors.Internal("get user by id", err)
	}
	return &u, nil
}

// ── Refresh tokens ────────────────────────────────────────────────────────────

func (r *UserRepository) StoreRefreshToken(ctx context.Context, userID, rawToken string, expiresAt time.Time) error {
	hash := hashToken(rawToken)
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hash, expiresAt)
	if err != nil {
		return apperrors.Internal("store refresh token", err)
	}
	return nil
}

func (r *UserRepository) ConsumeRefreshToken(ctx context.Context, rawToken string) (*domain.RefreshToken, error) {
	hash := hashToken(rawToken)
	var t domain.RefreshToken
	err := r.db.QueryRow(ctx, `
		DELETE FROM auth.refresh_tokens
		WHERE token_hash = $1 AND expires_at > now()
		RETURNING id, user_id, token_hash, expires_at, created_at
	`, hash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Unauthorized("invalid or expired refresh token")
		}
		return nil, apperrors.Internal("consume refresh token", err)
	}
	return &t, nil
}

func (r *UserRepository) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM auth.refresh_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return apperrors.Internal("revoke refresh tokens", err)
	}
	return nil
}

// ── MFA ───────────────────────────────────────────────────────────────────────

func (r *UserRepository) UpsertMFASecret(ctx context.Context, userID, encryptedSecret string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.mfa_secrets (user_id, secret, is_enabled)
		VALUES ($1, $2, false)
		ON CONFLICT (user_id) DO UPDATE
		  SET secret = EXCLUDED.secret, is_enabled = false
	`, userID, encryptedSecret)
	if err != nil {
		return apperrors.Internal("upsert mfa secret", err)
	}
	return nil
}

func (r *UserRepository) EnableMFA(ctx context.Context, userID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE auth.mfa_secrets SET is_enabled = true WHERE user_id = $1
	`, userID)
	if err != nil {
		return apperrors.Internal("enable mfa", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("mfa secret not found; call setup first")
	}
	return nil
}

func (r *UserRepository) GetMFASecret(ctx context.Context, userID string) (*domain.MFASecret, error) {
	var m domain.MFASecret
	err := r.db.QueryRow(ctx, `
		SELECT user_id, secret, is_enabled, created_at
		FROM auth.mfa_secrets WHERE user_id = $1
	`, userID).Scan(&m.UserID, &m.Secret, &m.IsEnabled, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("mfa not configured")
		}
		return nil, apperrors.Internal("get mfa secret", err)
	}
	return &m, nil
}

// ── Password reset ────────────────────────────────────────────────────────────

func (r *UserRepository) StorePasswordResetToken(ctx context.Context, userID, rawToken string, expiresAt time.Time) error {
	hash := hashToken(rawToken)
	// invalidate any existing tokens for this user first
	_, _ = r.db.Exec(ctx, `DELETE FROM auth.password_reset_tokens WHERE user_id = $1`, userID)
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hash, expiresAt)
	if err != nil {
		return apperrors.Internal("store password reset token", err)
	}
	return nil
}

func (r *UserRepository) ConsumePasswordResetToken(ctx context.Context, rawToken string) (*domain.PasswordResetToken, error) {
	hash := hashToken(rawToken)
	var t domain.PasswordResetToken
	err := r.db.QueryRow(ctx, `
		UPDATE auth.password_reset_tokens
		SET used_at = now()
		WHERE token_hash = $1 AND expires_at > now() AND used_at IS NULL
		RETURNING id, user_id, token_hash, expires_at, used_at, created_at
	`, hash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Unauthorized("invalid or expired reset token")
		}
		return nil, apperrors.Internal("consume password reset token", err)
	}
	return &t, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users SET password_hash = $1 WHERE id = $2
	`, passwordHash, userID)
	if err != nil {
		return apperrors.Internal("update password", err)
	}
	return nil
}

// ── Account lockout ───────────────────────────────────────────────────────────

// RecordFailedLogin increments the failed_login_attempts counter for a user.
// If the count reaches maxAttempts the account is locked until lockDuration from now.
func (r *UserRepository) RecordFailedLogin(ctx context.Context, userID string, maxAttempts int, lockDuration time.Duration) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
		        WHEN failed_login_attempts + 1 >= $2
		        THEN now() + $3::interval
		        ELSE locked_until
		    END
		WHERE id = $1
	`, userID, maxAttempts, lockDuration.String())
	if err != nil {
		return apperrors.Internal("record failed login", err)
	}
	return nil
}

// RecordSuccessfulLogin resets the failed attempts counter and records the login time.
func (r *UserRepository) RecordSuccessfulLogin(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users
		SET failed_login_attempts = 0, locked_until = NULL, last_login_at = now()
		WHERE id = $1
	`, userID)
	if err != nil {
		return apperrors.Internal("record successful login", err)
	}
	return nil
}

// ── API keys ──────────────────────────────────────────────────────────────────

func (r *UserRepository) CreateAPIKey(ctx context.Context, userID, name, keyHash, keyPrefix string, expiresAt *time.Time) (*domain.APIKey, error) {
	var k domain.APIKey
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth.api_keys (user_id, name, key_hash, key_prefix, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, key_prefix, key_hash, last_used, expires_at, created_at
	`, userID, name, keyHash, keyPrefix, expiresAt).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&k.LastUsed, &k.ExpiresAt, &k.CreatedAt,
	)
	if err != nil {
		return nil, apperrors.Internal("create api key", err)
	}
	return &k, nil
}

func (r *UserRepository) ListAPIKeys(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, key_prefix, key_hash, last_used, expires_at, created_at
		FROM auth.api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, apperrors.Internal("list api keys", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		var k domain.APIKey
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
			&k.LastUsed, &k.ExpiresAt, &k.CreatedAt,
		); err != nil {
			return nil, apperrors.Internal("scan api key", err)
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

func (r *UserRepository) DeleteAPIKey(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM auth.api_keys WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return apperrors.Internal("delete api key", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("api key not found")
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
