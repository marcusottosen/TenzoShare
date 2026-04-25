package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/auth/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/cache"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

const (
	loginRateLimit       = 5
	loginRateLimitWindow = 15 * time.Minute
	maxFailedAttempts    = 10
	lockoutDuration      = 15 * time.Minute
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

type MFASetupResult struct {
	Secret          string
	ProvisioningURI string
}

type AuditEvent struct {
	Action    string    `json:"action"`
	UserID    string    `json:"user_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type AuthService struct {
	repo  *repository.UserRepository
	cfg   *config.Config
	cache *cache.Client
	js    *jetstream.Client
	log   *zap.Logger
}

func New(
	repo *repository.UserRepository,
	cfg *config.Config,
	cache *cache.Client,
	js *jetstream.Client,
	log *zap.Logger,
) *AuthService {
	return &AuthService{repo: repo, cfg: cfg, cache: cache, js: js, log: log}
}

func (s *AuthService) EnsureBootstrapAdmin(ctx context.Context, email, password string) error {
	hash, err := crypto.HashPassword(password, s.cfg.App.BaseURL)
	if err != nil {
		return apperrors.Internal("hash bootstrap admin password", err)
	}

	user, err := s.repo.UpsertAdminByEmail(ctx, email, hash)
	if err != nil {
		return err
	}

	s.log.Info("bootstrap admin ensured",
		zap.String("userID", user.ID),
		zap.String("email", user.Email),
	)
	return nil
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*domain.User, error) {
	hash, err := crypto.HashPassword(password, s.cfg.App.BaseURL)
	if err != nil {
		return nil, apperrors.Internal("hash password", err)
	}
	user, err := s.repo.Create(ctx, email, hash, domain.RoleUser)
	if err != nil {
		return nil, err
	}
	s.log.Info("user registered", zap.String("userID", user.ID), zap.String("email", user.Email))
	s.publishAudit(ctx, AuditEvent{
		Action: "register", UserID: user.ID, Email: user.Email, Success: true, Timestamp: time.Now(),
	})
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password, clientIP string) (*TokenPair, *domain.User, error) {
	if clientIP != "" {
		limited, err := s.checkRateLimit(ctx, clientIP)
		if err != nil {
			s.log.Warn("rate limit check failed", zap.Error(err))
		} else if limited {
			return nil, nil, apperrors.RateLimit("too many login attempts; try again in 15 minutes")
		}
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		s.recordIPAttempt(ctx, clientIP)
		return nil, nil, apperrors.Unauthorized("invalid credentials")
	}

	if !user.IsActive {
		return nil, nil, apperrors.Unauthorized("account is disabled")
	}

	if user.IsLocked() {
		return nil, nil, apperrors.Unauthorized("account is temporarily locked; try again later")
	}

	ok, err := crypto.VerifyPassword(password, user.PasswordHash, s.cfg.App.BaseURL)
	if err != nil || !ok {
		_ = s.repo.RecordFailedLogin(ctx, user.ID, maxFailedAttempts, lockoutDuration)
		s.recordIPAttempt(ctx, clientIP)
		s.publishAudit(ctx, AuditEvent{
			Action: "login", UserID: user.ID, Email: email, ClientIP: clientIP,
			Success: false, Reason: "invalid credentials", Timestamp: time.Now(),
		})
		return nil, nil, apperrors.Unauthorized("invalid credentials")
	}

	if user.MFAEnabled {
		return nil, user, apperrors.Unauthorized("mfa_required")
	}

	_ = s.repo.RecordSuccessfulLogin(ctx, user.ID)
	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	s.publishAudit(ctx, AuditEvent{
		Action: "login", UserID: user.ID, Email: email, ClientIP: clientIP,
		Success: true, Timestamp: time.Now(),
	})
	return pair, user, nil
}

func (s *AuthService) LoginWithMFA(ctx context.Context, userID, otpCode string) (*TokenPair, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	mfaSecret, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return nil, apperrors.Unauthorized("mfa not configured")
	}

	encKey := s.mfaEncryptionKey()
	rawSecret, err := crypto.Decrypt(mfaSecret.Secret, encKey)
	if err != nil {
		return nil, apperrors.Internal("decrypt mfa secret", err)
	}

	if !totp.Validate(otpCode, string(rawSecret)) {
		s.publishAudit(ctx, AuditEvent{
			Action: "login_mfa", UserID: userID, Success: false,
			Reason: "invalid OTP", Timestamp: time.Now(),
		})
		return nil, apperrors.Unauthorized("invalid OTP code")
	}

	_ = s.repo.RecordSuccessfulLogin(ctx, userID)
	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}
	s.publishAudit(ctx, AuditEvent{
		Action: "login_mfa", UserID: userID, Email: user.Email, Success: true, Timestamp: time.Now(),
	})
	return pair, nil
}

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

func (s *AuthService) Logout(ctx context.Context, userID string) error {
	err := s.repo.RevokeAllRefreshTokens(ctx, userID)
	if err == nil {
		s.publishAudit(ctx, AuditEvent{
			Action: "logout", UserID: userID, Success: true, Timestamp: time.Now(),
		})
	}
	return err
}

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

	err = s.repo.EnableMFA(ctx, userID)
	if err == nil {
		s.publishAudit(ctx, AuditEvent{
			Action: "mfa_enabled", UserID: userID, Success: true, Timestamp: time.Now(),
		})
	}
	return err
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) (userID, resetToken string, err error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
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

	s.publishAudit(ctx, AuditEvent{
		Action: "password_reset_request", UserID: user.ID, Email: email,
		Success: true, Timestamp: time.Now(),
	})
	return user.ID, token, nil
}

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

	_ = s.repo.RevokeAllRefreshTokens(ctx, rt.UserID)
	s.publishAudit(ctx, AuditEvent{
		Action: "password_reset_confirm", UserID: rt.UserID, Success: true, Timestamp: time.Now(),
	})
	return nil
}

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

func (s *AuthService) mfaEncryptionKey() []byte {
	key := []byte(s.cfg.JWT.Secret)
	if len(key) >= 32 {
		return key[:32]
	}
	padded := make([]byte, 32)
	copy(padded, key)
	return padded
}

func (s *AuthService) checkRateLimit(ctx context.Context, clientIP string) (bool, error) {
	if s.cache == nil {
		return false, nil
	}
	key := fmt.Sprintf("ratelimit:login:%s", clientIP)
	count, err := s.cache.Incr(ctx, key)
	if err != nil {
		return false, err
	}
	if count == 1 {
		_, _ = s.cache.Expire(ctx, key, loginRateLimitWindow)
	}
	return count > loginRateLimit, nil
}

func (s *AuthService) recordIPAttempt(ctx context.Context, clientIP string) {
	if s.cache == nil || clientIP == "" {
		return
	}
	key := fmt.Sprintf("ratelimit:login:%s", clientIP)
	count, err := s.cache.Incr(ctx, key)
	if err != nil {
		return
	}
	if count == 1 {
		_, _ = s.cache.Expire(ctx, key, loginRateLimitWindow)
	}
}

func (s *AuthService) publishAudit(ctx context.Context, ev AuditEvent) {
	if s.js == nil {
		return
	}
	go func() {
		if err := s.js.Publish(ctx, "AUDIT.auth", ev); err != nil {
			s.log.Warn("failed to publish audit event", zap.Error(err), zap.String("action", ev.Action))
		}
	}()
}

// GetMe returns the full profile for the authenticated user.
func (s *AuthService) GetMe(ctx context.Context, userID string) (*domain.User, error) {
	return s.repo.GetByID(ctx, userID)
}

// ChangePasswordParams holds inputs for a self-service password change.
type ChangePasswordParams struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

// ChangePassword verifies the current password then replaces it with the new one.
func (s *AuthService) ChangePassword(ctx context.Context, p ChangePasswordParams) error {
	user, err := s.repo.GetByID(ctx, p.UserID)
	if err != nil {
		return err
	}

	ok, err := crypto.VerifyPassword(p.CurrentPassword, user.PasswordHash, s.cfg.App.BaseURL)
	if err != nil {
		return apperrors.Internal("verify current password", err)
	}
	if !ok {
		return apperrors.Unauthorized("current password is incorrect")
	}

	if len(p.NewPassword) < 8 {
		return apperrors.Validation("new password must be at least 8 characters")
	}

	newHash, err := crypto.HashPassword(p.NewPassword, s.cfg.App.BaseURL)
	if err != nil {
		return apperrors.Internal("hash new password", err)
	}
	if err := s.repo.UpdatePassword(ctx, p.UserID, newHash); err != nil {
		return err
	}

	s.publishAudit(ctx, AuditEvent{
		Action:  "auth.password_changed",
		UserID:  p.UserID,
		Success: true,
	})
	return nil
}
