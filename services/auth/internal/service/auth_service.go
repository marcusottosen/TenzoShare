package service

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/auth/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

// TokenPair is the response to a successful login or token refresh.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // access token TTL in seconds
}

// MFASetupResult is returned from SetupMFA.
type MFASetupResult struct {
	Secret          string // raw TOTP secret (show once, never stored raw)
	ProvisioningURI string // otpauth:// URI for QR code
}

type AuthService struct {
	repo   *repository.UserRepository
	cfg    *config.Config
	log    *zap.Logger
}

func New(repo *repository.UserRepository, cfg *config.Config, log *zap.Logger) *AuthService {
	return &AuthService{repo: repo, cfg: cfg, log: log}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, password string) (*domain.User, error) {
	hash, err := crypto.HashPassword(password, s.cfg.App.BaseURL) // BaseURL acts as the pepper seed here; use a dedicated pepper var in production
	if err != nil {
		return nil, apperrors.Internal("hash password", err)
	}
	user, err := s.repo.Create(ctx, email, hash, domain.RoleUser)
	if err != nil {
		return nil, err
	}
	s.log.Info("user registered", zap.String("userID", user.ID), zap.String("email", user.Email))
	return user, nil
}

// Login verifies credentials and returns a token pair.
// If MFA is enabled, returns ErrMFARequired — the caller should prompt for the OTP.
func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, *domain.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		// don't leak whether the email exists
		return nil, nil, apperrors.Unauthorized("invalid credentials")
	}
	if !user.IsActive {
		return nil, nil, apperrors.Unauthorized("account is disabled")
	}

	ok, err := crypto.VerifyPassword(password, user.PasswordHash, s.cfg.App.BaseURL)
	if err != nil || !ok {
		return nil, nil, apperrors.Unauthorized("invalid credentials")
	}

	if user.MFAEnabled {
		// caller needs to call LoginWithMFA after collecting OTP from the user
		return nil, user, apperrors.Unauthorized("mfa_required")
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return pair, user, nil
}

// LoginWithMFA verifies OTP then issues tokens.
func (s *AuthService) LoginWithMFA(ctx context.Context, userID, otpCode string) (*TokenPair, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	mfaSecret, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return nil, apperrors.Unauthorized("mfa not configured")
	}

	// decrypt stored secret
	encKey := s.mfaEncryptionKey()
	rawSecret, err := crypto.Decrypt(mfaSecret.Secret, encKey)
	if err != nil {
		return nil, apperrors.Internal("decrypt mfa secret", err)
	}

	if !totp.Validate(otpCode, string(rawSecret)) {
		return nil, apperrors.Unauthorized("invalid OTP code")
	}

	return s.issueTokenPair(ctx, user)
}

// RefreshTokens exchanges a valid refresh token for a new token pair.
func (s *AuthService) RefreshTokens(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	rt, err := s.repo.ConsumeRefreshToken(ctx, rawRefreshToken)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, apperrors.Unauthorized("user not found")
	}
	if !user.IsActive {
		return nil, apperrors.Unauthorized("account is disabled")
	}
	return s.issueTokenPair(ctx, user)
}

// Logout revokes all refresh tokens for the user.
func (s *AuthService) Logout(ctx context.Context, userID string) error {
	return s.repo.RevokeAllRefreshTokens(ctx, userID)
}

// SetupMFA generates a TOTP secret and stores it (disabled) until VerifyMFA is called.
func (s *AuthService) SetupMFA(ctx context.Context, userID string) (*MFASetupResult, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "TenzoShare",
		AccountName: user.Email,
	})
	if err != nil {
		return nil, apperrors.Internal("generate totp key", err)
	}

	encKey := s.mfaEncryptionKey()
	encrypted, err := crypto.Encrypt([]byte(key.Secret()), encKey)
	if err != nil {
		return nil, apperrors.Internal("encrypt mfa secret", err)
	}

	if err := s.repo.UpsertMFASecret(ctx, userID, encrypted); err != nil {
		return nil, err
	}

	return &MFASetupResult{
		Secret:          key.Secret(),
		ProvisioningURI: key.URL(),
	}, nil
}

// VerifyMFA confirms the TOTP code and enables MFA for the user.
func (s *AuthService) VerifyMFA(ctx context.Context, userID, otpCode string) error {
	mfaSecret, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return err
	}

	encKey := s.mfaEncryptionKey()
	rawSecret, err := crypto.Decrypt(mfaSecret.Secret, encKey)
	if err != nil {
		return apperrors.Internal("decrypt mfa secret", err)
	}

	if !totp.Validate(otpCode, string(rawSecret)) {
		return apperrors.Unauthorized("invalid OTP code")
	}

	return s.repo.EnableMFA(ctx, userID)
}

// RequestPasswordReset generates a reset token. In production the caller sends
// this token via email; here we return it so the handler can pass it to
// the notification service.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) (userID, resetToken string, err error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		// don't leak whether the email exists — return success regardless
		return "", "", nil
	}

	token, err := crypto.RandomToken(32)
	if err != nil {
		return "", "", apperrors.Internal("generate reset token", err)
	}

	expiresAt := time.Now().Add(1 * time.Hour)
	if err := s.repo.StorePasswordResetToken(ctx, user.ID, token, expiresAt); err != nil {
		return "", "", err
	}

	return user.ID, token, nil
}

// ConfirmPasswordReset validates the token and sets a new password.
func (s *AuthService) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	rt, err := s.repo.ConsumePasswordResetToken(ctx, rawToken)
	if err != nil {
		return err
	}

	hash, err := crypto.HashPassword(newPassword, s.cfg.App.BaseURL)
	if err != nil {
		return apperrors.Internal("hash password", err)
	}

	if err := s.repo.UpdatePassword(ctx, rt.UserID, hash); err != nil {
		return err
	}

	// invalidate all sessions after a password change
	return s.repo.RevokeAllRefreshTokens(ctx, rt.UserID)
}

// ValidateAccessToken parses and validates a JWT access token.
// Used by other services (via gRPC) to validate tokens without calling auth HTTP API.
func (s *AuthService) ValidateAccessToken(tokenString string) (*middleware.Claims, error) {
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrors.Unauthorized("unexpected signing method")
		}
		return []byte(s.cfg.JWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, apperrors.Unauthorized("invalid or expired token")
	}
	return claims, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (s *AuthService) issueTokenPair(ctx context.Context, user *domain.User) (*TokenPair, error) {
	accessToken, err := s.signAccessToken(user)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := crypto.RandomToken(48)
	if err != nil {
		return nil, apperrors.Internal("generate refresh token", err)
	}

	expiresAt := time.Now().Add(s.cfg.JWT.RefreshTTL)
	if err := s.repo.StoreRefreshToken(ctx, user.ID, rawRefresh, expiresAt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.cfg.JWT.AccessTTL.Seconds()),
	}, nil
}

func (s *AuthService) signAccessToken(user *domain.User) (string, error) {
	now := time.Now()
	claims := middleware.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWT.AccessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return "", apperrors.Internal("sign access token", err)
	}
	return signed, nil
}

// mfaEncryptionKey derives a 32-byte key from JWT_SECRET for encrypting TOTP secrets.
// In production you'd use a dedicated MFA_ENCRYPTION_KEY env var.
func (s *AuthService) mfaEncryptionKey() []byte {
	key := []byte(s.cfg.JWT.Secret)
	if len(key) >= 32 {
		return key[:32]
	}
	// pad if shorter than 32 (shouldn't happen given config validation)
	padded := make([]byte, 32)
	copy(padded, key)
	return padded
}
