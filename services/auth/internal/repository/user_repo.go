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

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.role, u.is_active, u.email_verified,
		       u.created_at, u.updated_at,
		       COALESCE(m.is_enabled, false) AS mfa_enabled
		FROM auth.users u
		LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id
		WHERE u.email = $1
	`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
		&u.MFAEnabled,
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
		       COALESCE(m.is_enabled, false) AS mfa_enabled
		FROM auth.users u
		LEFT JOIN auth.mfa_secrets m ON m.user_id = u.id
		WHERE u.id = $1
	`, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
		&u.MFAEnabled,
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

// ── helpers ───────────────────────────────────────────────────────────────────

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
