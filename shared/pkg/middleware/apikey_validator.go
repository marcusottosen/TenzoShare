package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// NewAPIKeyValidator returns an APIKeyValidatorFunc that validates TenzoShare API keys
// by querying the auth.api_keys table directly. All backend services share the same
// PostgreSQL instance, so this helper works from any service that has a DB pool.
//
// On success it asynchronously updates the key's last_used timestamp.
// Expiry (expires_at) and user activation (is_active) are enforced at query time.
func NewAPIKeyValidator(pool *pgxpool.Pool) APIKeyValidatorFunc {
	return func(ctx context.Context, rawKey string) (userID, role string, err error) {
		h := sha256.Sum256([]byte(rawKey))
		hash := fmt.Sprintf("%x", h[:])

		var keyID string
		err = pool.QueryRow(ctx, `
			SELECT k.id, u.id, u.role
			FROM auth.api_keys k
			JOIN auth.users u ON u.id = k.user_id
			WHERE k.key_hash = $1
			  AND u.is_active = true
			  AND (k.expires_at IS NULL OR k.expires_at > NOW())
		`, hash).Scan(&keyID, &userID, &role)
		if err != nil {
			// Do not distinguish "not found" from "expired" — prevents timing oracles.
			return "", "", apperrors.Unauthorized("invalid or expired api key")
		}

		// Update last_used asynchronously — failure is tolerable (monitoring only).
		go func(kid string) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = pool.Exec(bgCtx,
				"UPDATE auth.api_keys SET last_used = NOW() WHERE id = $1", kid)
		}(keyID)

		return userID, role, nil
	}
}
