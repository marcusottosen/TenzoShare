package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/models"
	"github.com/tenzoshare/tenzoshare/services/admin/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
)

// ── User service ─────────────────────────────────────────────────────────────

// pepper returns the password pepper from environment or a default for dev.
func (s *Service) pepper() string {
	pepper := os.Getenv("PEPPER")
	if pepper == "" {
		pepper = "default-dev-pepper-change-in-production"
	}
	return pepper
}

// ListUsers returns a paginated list of users.
func (s *Service) ListUsers(ctx context.Context, filters repository.UserFilters) ([]models.UserRow, int, error) {
	return s.repo.ListUsers(ctx, filters)
}

// GetUser returns a single user by ID.
func (s *Service) GetUser(ctx context.Context, userID string) (*models.UserRow, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// CreateUser creates a new user with hashed password.
func (s *Service) CreateUser(ctx context.Context, email, password, role, adminID, adminEmail, ipAddr string) (string, error) {
	// Hash password
	passwordHash, err := crypto.HashPassword(password, s.pepper())
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	// Create user
	userID, err := s.repo.CreateUser(ctx, email, passwordHash, role)
	if err != nil {
		return "", err
	}

	// Audit
	s.PublishAuditEvent(ctx, "admin.user.create", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": email,
		"role":         role,
	})

	return userID, nil
}

// UpdateUser updates a user's role and/or active status.
func (s *Service) UpdateUser(ctx context.Context, userID string, role *string, isActive *bool, adminID, adminEmail, ipAddr string) error {
	// Verify user exists
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	// Update
	if err := s.repo.UpdateUser(ctx, userID, role, isActive); err != nil {
		return err
	}

	// Audit
	changes := map[string]any{"target_email": user.Email}
	if role != nil {
		changes["new_role"] = *role
	}
	if isActive != nil {
		changes["new_active"] = *isActive
	}

	s.PublishAuditEvent(ctx, "admin.user.update", adminID, adminEmail, userID, ipAddr, changes)

	return nil
}

// DeleteUser deletes a user and all related data.
func (s *Service) DeleteUser(ctx context.Context, userID, adminID, adminEmail, ipAddr string) error {
	// Get user before deleting
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	// Delete
	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return err
	}

	// Audit
	s.PublishAuditEvent(ctx, "admin.user.delete", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": user.Email,
	})

	return nil
}

// UnlockUser clears failed login attempts and unlocks the account.
func (s *Service) UnlockUser(ctx context.Context, userID, adminID, adminEmail, ipAddr string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	if err := s.repo.UnlockUser(ctx, userID); err != nil {
		return err
	}

	s.PublishAuditEvent(ctx, "admin.user.unlock", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": user.Email,
	})

	return nil
}

// ResetUserMFA removes a user's MFA secret.
func (s *Service) ResetUserMFA(ctx context.Context, userID, adminID, adminEmail, ipAddr string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	if err := s.repo.ResetUserMFA(ctx, userID); err != nil {
		return err
	}

	s.PublishAuditEvent(ctx, "admin.user.mfa_reset", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": user.Email,
	})

	return nil
}

// VerifyUserEmail marks a user's email as verified.
func (s *Service) VerifyUserEmail(ctx context.Context, userID, adminID, adminEmail, ipAddr string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	if err := s.repo.VerifyUserEmail(ctx, userID); err != nil {
		return err
	}

	s.PublishAuditEvent(ctx, "admin.user.verify_email", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": user.Email,
	})

	return nil
}

// ResetUserPassword generates a random temporary password and returns it.
func (s *Service) ResetUserPassword(ctx context.Context, userID, adminID, adminEmail, ipAddr string) (string, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("user not found")
		}
		return "", err
	}

	// Generate 12-byte random password
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	tempPassword := base64.RawURLEncoding.EncodeToString(b)

	// Hash and save
	passwordHash, err := crypto.HashPassword(tempPassword, s.pepper())
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.SetUserPassword(ctx, userID, passwordHash); err != nil {
		return "", err
	}

	s.PublishAuditEvent(ctx, "admin.user.reset_password", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": user.Email,
	})

	return tempPassword, nil
}

// SetUserPassword sets a user's password to a specific value.
func (s *Service) SetUserPassword(ctx context.Context, userID, password, adminID, adminEmail, ipAddr string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	passwordHash, err := crypto.HashPassword(password, s.pepper())
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.SetUserPassword(ctx, userID, passwordHash); err != nil {
		return err
	}

	s.PublishAuditEvent(ctx, "admin.user.set_password", adminID, adminEmail, userID, ipAddr, map[string]any{
		"target_email": user.Email,
	})

	return nil
}

// ── User quota service ───────────────────────────────────────────────────────

// ListUserQuotas returns all per-user quota overrides.
func (s *Service) ListUserQuotas(ctx context.Context) ([]models.UserQuota, error) {
	return s.repo.ListUserQuotas(ctx)
}

// GetUserQuota returns a user's quota override (or nil if none).
func (s *Service) GetUserQuota(ctx context.Context, userID string) (*models.UserQuota, error) {
	quota, err := s.repo.GetUserQuota(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No override = nil, not an error
		}
		return nil, err
	}
	return quota, nil
}

// SetUserQuota creates or updates a user's quota override.
func (s *Service) SetUserQuota(ctx context.Context, userID string, quotaBytes *int64, adminID, adminEmail, ipAddr string) error {
	// Verify user exists
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	// Set or remove quota
	if quotaBytes == nil {
		// Remove override
		if err := s.repo.DeleteUserQuota(ctx, userID); err != nil {
			return err
		}
		s.PublishAuditEvent(ctx, "admin.quota.remove", adminID, adminEmail, userID, ipAddr, map[string]any{
			"target_email": user.Email,
		})
	} else {
		// Set override
		if err := s.repo.SetUserQuota(ctx, userID, quotaBytes, adminID); err != nil {
			return err
		}
		s.PublishAuditEvent(ctx, "admin.quota.set", adminID, adminEmail, userID, ipAddr, map[string]any{
			"target_email": user.Email,
			"quota_bytes":  *quotaBytes,
		})
	}

	return nil
}
