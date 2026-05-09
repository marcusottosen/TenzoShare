package service

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

	registerRateLimit       = 10
	registerRateLimitWindow = 1 * time.Hour

	passwordResetRateLimit       = 5
	passwordResetRateLimitWindow = 1 * time.Hour

	// MFA endpoint rate limits (per user ID, not per IP — prevents account enumeration).
	mfaSetupRateLimit         = 3 // 3 setup attempts
	mfaSetupRateLimitWindow   = 10 * time.Minute
	mfaVerifyRateLimit        = 5 // 5 verify attempts
	mfaVerifyRateLimitWindow  = 5 * time.Minute
	mfaDisableRateLimit       = 3 // 3 disable attempts
	mfaDisableRateLimitWindow = 15 * time.Minute
	mfaLoginRateLimit         = 5 // 5 /login/mfa attempts per user_id
	mfaLoginRateLimitWindow   = 15 * time.Minute

	lockoutConfigTTL = 60 * time.Second
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

// userRepository is the data-access interface required by AuthService.
// It is satisfied by *repository.UserRepository.
type userRepository interface {
	Create(ctx context.Context, email, passwordHash string, role domain.Role) (*domain.User, error)
	UpsertAdminByEmail(ctx context.Context, email, passwordHash string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	StoreRefreshToken(ctx context.Context, userID, rawToken string, expiresAt time.Time) error
	ConsumeRefreshToken(ctx context.Context, rawToken string) (*domain.RefreshToken, error)
	RevokeAllRefreshTokens(ctx context.Context, userID string) error
	GetMFASecret(ctx context.Context, userID string) (*domain.MFASecret, error)
	UpsertMFASecret(ctx context.Context, userID, encryptedSecret string) error
	EnableMFA(ctx context.Context, userID string) error
	StorePasswordResetToken(ctx context.Context, userID, rawToken string, expiresAt time.Time) error
	ConsumePasswordResetToken(ctx context.Context, rawToken string) (*domain.PasswordResetToken, error)
	UpdatePassword(ctx context.Context, userID, newHash string) error
	RecordFailedLogin(ctx context.Context, userID string, maxAttempts int, lockDuration time.Duration) error
	RecordSuccessfulLogin(ctx context.Context, userID string) error
	GetLockoutConfig(ctx context.Context) (maxAttempts int, lockDuration time.Duration, requireMFA bool, err error)
	CreateAPIKey(ctx context.Context, userID, name, keyHash, keyPrefix string, expiresAt *time.Time) (*domain.APIKey, error)
	ListAPIKeys(ctx context.Context, userID string) ([]*domain.APIKey, error)
	DeleteAPIKey(ctx context.Context, id, userID string) error
	UpdatePreferences(ctx context.Context, userID string, dateFormat, timeFormat, timezone *string) error
	DisableMFA(ctx context.Context, userID string) error
}

type AuthService struct {
	repo       userRepository
	cfg        *config.Config
	cache      *cache.Client
	js         *jetstream.Client
	log        *zap.Logger
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	mfaKey     []byte // 32-byte AES key derived from PASSWORD_PEPPER

	lockoutMu          sync.Mutex
	cachedMaxAttempts  int
	cachedLockDuration time.Duration
	cachedRequireMFA   bool
	lockoutCachedAt    time.Time
}

func New(
	repo *repository.UserRepository,
	cfg *config.Config, cache *cache.Client,
	js *jetstream.Client,
	log *zap.Logger,
) (*AuthService, error) {
	privKey, err := parseRSAPrivateKey(cfg.JWT.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("auth service: parse JWT private key: %w", err)
	}

	mfaKey, err := deriveMFAKey(cfg.App.Pepper)
	if err != nil {
		return nil, fmt.Errorf("auth service: derive MFA key from pepper: %w", err)
	}

	return &AuthService{
		repo:       repo,
		cfg:        cfg,
		cache:      cache,
		js:         js,
		log:        log,
		privateKey: privKey,
		publicKey:  &privKey.PublicKey,
		mfaKey:     mfaKey,
	}, nil
}

// parseRSAPrivateKey parses a PKCS#8 or PKCS#1 RSA private key from PEM.
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	if pemStr == "" {
		return nil, fmt.Errorf("JWT_PRIVATE_KEY is not set")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	// Try PKCS#8 first (openssl genpkey output)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PEM key is not RSA")
		}
		return rsaKey, nil
	}
	// Fall back to PKCS#1
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// deriveMFAKey derives a 32-byte AES key from the password pepper.
// If pepper is a 64-hex string (32 bytes), decode it; otherwise pad/truncate to 32 bytes.
func deriveMFAKey(pepper string) ([]byte, error) {
	if pepper == "" {
		return nil, fmt.Errorf("PASSWORD_PEPPER is not set")
	}
	if len(pepper) == 64 {
		b, err := hex.DecodeString(pepper)
		if err == nil && len(b) == 32 {
			return b, nil
		}
	}
	key := make([]byte, 32)
	copy(key, []byte(pepper))
	return key, nil
}

// lockoutConfig returns the current lockout policy, reading from DB if the
// cached values are older than lockoutConfigTTL (60 s).
func (s *AuthService) lockoutConfig(ctx context.Context) (maxAttempts int, lockDuration time.Duration, requireMFA bool) {
	s.lockoutMu.Lock()
	defer s.lockoutMu.Unlock()
	if time.Since(s.lockoutCachedAt) < lockoutConfigTTL && s.cachedMaxAttempts > 0 {
		return s.cachedMaxAttempts, s.cachedLockDuration, s.cachedRequireMFA
	}
	ma, ld, rmfa, err := s.repo.GetLockoutConfig(ctx)
	if err != nil {
		s.log.Warn("lockoutConfig: could not read auth_settings, using fallback", zap.Error(err))
		return 10, 15 * time.Minute, false
	}
	s.cachedMaxAttempts = ma
	s.cachedLockDuration = ld
	s.cachedRequireMFA = rmfa
	s.lockoutCachedAt = time.Now()
	return ma, ld, rmfa
}

func (s *AuthService) EnsureBootstrapAdmin(ctx context.Context, email, password string) error {
	hash, err := crypto.HashPassword(password, s.cfg.App.Pepper)
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

func (s *AuthService) Register(ctx context.Context, email, password, clientIP string) (*domain.User, error) {
	if clientIP != "" {
		limited, err := s.checkRateLimitGeneric(ctx, "ratelimit:register:"+clientIP, registerRateLimit, registerRateLimitWindow)
		if err != nil {
			s.log.Warn("register rate-limit check failed", zap.Error(err))
		} else if limited {
			return nil, apperrors.RateLimit("too many registrations from this IP; try again later")
		}
	}

	hash, err := crypto.HashPassword(password, s.cfg.App.Pepper)
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

func (s *AuthService) Login(ctx context.Context, email, password, clientIP string) (*TokenPair, *domain.User, bool, error) {
	if clientIP != "" {
		limited, err := s.checkRateLimit(ctx, clientIP)
		if err != nil {
			s.log.Warn("rate limit check failed", zap.Error(err))
		} else if limited {
			return nil, nil, false, apperrors.RateLimit("too many login attempts; try again in 15 minutes")
		}
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		s.recordIPAttempt(ctx, clientIP)
		return nil, nil, false, apperrors.Unauthorized("invalid credentials")
	}

	if !user.IsActive {
		return nil, nil, false, apperrors.Unauthorized("account is disabled")
	}

	if user.IsLocked() {
		return nil, nil, false, apperrors.Unauthorized("account is temporarily locked; try again later")
	}

	ok, err := crypto.VerifyPassword(password, user.PasswordHash, s.cfg.App.Pepper)
	if err != nil || !ok {
		maxAttempts, lockDur, _ := s.lockoutConfig(ctx)
		_ = s.repo.RecordFailedLogin(ctx, user.ID, maxAttempts, lockDur)
		s.recordIPAttempt(ctx, clientIP)
		s.publishAudit(ctx, AuditEvent{
			Action: "login", UserID: user.ID, Email: email, ClientIP: clientIP,
			Success: false, Reason: "invalid credentials", Timestamp: time.Now(),
		})
		return nil, nil, false, apperrors.Unauthorized("invalid credentials")
	}

	if user.MFAEnabled {
		return nil, user, false, apperrors.Unauthorized("mfa_required")
	}

	// If admin requires MFA but user hasn't set it up, issue a setup-only token
	// (short-lived, MFASetupRequired=true in claims) rather than full access tokens.
	// The BlockIfMFASetupPending middleware enforces that this token can only reach
	// /mfa/setup and /mfa/verify.
	_, _, requireMFA := s.lockoutConfig(ctx)
	mfaSetupRequired := requireMFA && !user.MFAEnabled

	_ = s.repo.RecordSuccessfulLogin(ctx, user.ID)

	if mfaSetupRequired {
		setupToken, err := s.signSetupOnlyToken(user)
		if err != nil {
			return nil, nil, false, err
		}
		setupPair := &TokenPair{
			AccessToken: setupToken,
			ExpiresIn:   int64((10 * time.Minute).Seconds()),
		}
		s.publishAudit(ctx, AuditEvent{
			Action: "login", UserID: user.ID, Email: email, ClientIP: clientIP,
			Success: true, Timestamp: time.Now(),
		})
		return setupPair, user, true, nil
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, false, err
	}
	s.publishAudit(ctx, AuditEvent{
		Action: "login", UserID: user.ID, Email: email, ClientIP: clientIP,
		Success: true, Timestamp: time.Now(),
	})
	return pair, user, false, nil
}

func (s *AuthService) LoginWithMFA(ctx context.Context, userID, otpCode string) (*TokenPair, error) {
	// Rate limit per user_id to prevent brute force of 6-digit TOTP codes.
	if limited, _ := s.checkRateLimitGeneric(ctx, "ratelimit:mfa_login:"+userID, mfaLoginRateLimit, mfaLoginRateLimitWindow); limited {
		return nil, apperrors.RateLimit("too many MFA attempts; try again in 15 minutes")
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	if !user.IsActive {
		return nil, apperrors.Unauthorized("account is disabled")
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
	// Rate limit: prevent rapid secret-generation (amplifies replacement attacks).
	if limited, _ := s.checkRateLimitGeneric(ctx, "ratelimit:mfa_setup:"+userID, mfaSetupRateLimit, mfaSetupRateLimitWindow); limited {
		return nil, apperrors.RateLimit("too many MFA setup requests; try again later")
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Prevent secret-replacement: if MFA is already enabled, the user must
	// disable it first (which requires a current OTP). This stops an attacker
	// who has the session from silently replacing the TOTP secret.
	if user.MFAEnabled {
		return nil, apperrors.Conflict("MFA is already enabled; disable it first")
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

func (s *AuthService) VerifyMFA(ctx context.Context, userID, otpCode string) (*TokenPair, error) {
	// Rate limit to prevent brute-force of 6-digit codes during setup.
	if limited, _ := s.checkRateLimitGeneric(ctx, "ratelimit:mfa_verify:"+userID, mfaVerifyRateLimit, mfaVerifyRateLimitWindow); limited {
		return nil, apperrors.RateLimit("too many MFA verification attempts; try again in 5 minutes")
	}

	mfaSecret, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return nil, err
	}

	encKey := s.mfaEncryptionKey()
	rawSecret, err := crypto.Decrypt(mfaSecret.Secret, encKey)
	if err != nil {
		return nil, apperrors.Internal("decrypt mfa secret", err)
	}

	if !totp.Validate(otpCode, string(rawSecret)) {
		return nil, apperrors.Unauthorized("invalid OTP code")
	}

	if err := s.repo.EnableMFA(ctx, userID); err != nil {
		return nil, err
	}
	s.publishAudit(ctx, AuditEvent{
		Action: "mfa_enabled", UserID: userID, Success: true, Timestamp: time.Now(),
	})

	// After enabling MFA, fetch the user and issue full tokens so the client
	// can immediately use the session (especially when coming from the
	// mfa_setup_required flow where they only had a setup-only token).
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("fetch user after mfa enable", err)
	}
	return s.issueTokenPair(ctx, user)
}

// DisableMFA verifies the current OTP code then removes the user's MFA secret,
// fully disabling two-factor authentication.
func (s *AuthService) DisableMFA(ctx context.Context, userID, otpCode string) error {
	// Rate limit to prevent brute-force of the disable endpoint.
	if limited, _ := s.checkRateLimitGeneric(ctx, "ratelimit:mfa_disable:"+userID, mfaDisableRateLimit, mfaDisableRateLimitWindow); limited {
		return apperrors.RateLimit("too many MFA disable attempts; try again in 15 minutes")
	}

	mfaSecret, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return apperrors.BadRequest("MFA is not enabled for this account")
	}

	encKey := s.mfaEncryptionKey()
	rawSecret, err := crypto.Decrypt(mfaSecret.Secret, encKey)
	if err != nil {
		return apperrors.Internal("decrypt mfa secret", err)
	}

	if !totp.Validate(otpCode, string(rawSecret)) {
		return apperrors.Unauthorized("invalid OTP code")
	}

	if err := s.repo.DisableMFA(ctx, userID); err != nil {
		return err
	}
	s.publishAudit(ctx, AuditEvent{
		Action: "mfa_disabled", UserID: userID, Success: true, Timestamp: time.Now(),
	})
	return nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email, clientIP string) (userID, resetToken string, err error) {
	if clientIP != "" {
		var limited bool
		limited, err = s.checkRateLimitGeneric(ctx, "ratelimit:pwreset:"+clientIP, passwordResetRateLimit, passwordResetRateLimitWindow)
		if err != nil {
			s.log.Warn("password-reset rate-limit check failed", zap.Error(err))
			err = nil // non-fatal
		} else if limited {
			return "", "", apperrors.RateLimit("too many password-reset requests from this IP; try again later")
		}
	}

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

	hash, err := crypto.HashPassword(newPassword, s.cfg.App.Pepper)
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
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, apperrors.Unauthorized("unexpected signing method")
		}
		return s.publicKey, nil
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
		JTI:    uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWT.AccessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", apperrors.Internal("sign access token", err)
	}
	return signed, nil
}

func (s *AuthService) mfaEncryptionKey() []byte {
	return s.mfaKey
}

// signSetupOnlyToken issues a short-lived (10 min) access token with
// MFASetupRequired=true. Tokens with this flag are blocked on all protected
// routes except /mfa/setup and /mfa/verify by the BlockIfMFASetupPending
// middleware.
func (s *AuthService) signSetupOnlyToken(user *domain.User) (string, error) {
	now := time.Now()
	const setupTTL = 10 * time.Minute
	claims := middleware.Claims{
		UserID:           user.ID,
		Email:            user.Email,
		Role:             string(user.Role),
		JTI:              uuid.New().String(),
		MFASetupRequired: true,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(setupTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", apperrors.Internal("sign setup-only access token", err)
	}
	return signed, nil
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

func (s *AuthService) checkRateLimitGeneric(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if s.cache == nil {
		return false, nil
	}
	count, err := s.cache.Incr(ctx, key)
	if err != nil {
		return false, err
	}
	if count == 1 {
		_, _ = s.cache.Expire(ctx, key, window)
	}
	return count > limit, nil
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

// UpdatePreferences stores per-user date/time formatting preferences.
// Pass nil for a field to reset it to the system default.
func (s *AuthService) UpdatePreferences(ctx context.Context, userID string, dateFormat, timeFormat, timezone *string) error {
	validDateFormats := map[string]bool{"ISO": true, "EU": true, "US": true, "DE": true, "LONG": true}
	if dateFormat != nil && !validDateFormats[*dateFormat] {
		return apperrors.Validation("date_format must be one of: ISO, EU, US, DE, LONG")
	}
	if timeFormat != nil && *timeFormat != "12h" && *timeFormat != "24h" {
		return apperrors.Validation("time_format must be '12h' or '24h'")
	}
	return s.repo.UpdatePreferences(ctx, userID, dateFormat, timeFormat, timezone)
}

// RevokeAccessToken places a token's JTI on the Redis blacklist so it cannot
// be reused even before it naturally expires (e.g. after logout or password change).
// No-op if Redis is unavailable.
func (s *AuthService) RevokeAccessToken(ctx context.Context, jti string) error {
	if s.cache == nil || jti == "" {
		return nil
	}
	return s.cache.RevokeToken(ctx, jti, s.cfg.JWT.AccessTTL)
}

// IsTokenRevoked reports whether a JTI is currently blacklisted.
// Used by the gateway-level revocation check middleware.
func (s *AuthService) IsTokenRevoked(ctx context.Context, jti string) bool {
	if s.cache == nil {
		return false
	}
	return s.cache.IsTokenRevoked(ctx, jti)
}

// ── API key management ────────────────────────────────────────────────────────

// APIKeyResult is returned once on creation — RawKey is never stored and cannot be retrieved again.
type APIKeyResult struct {
	*domain.APIKey
	RawKey string
}

// CreateAPIKey generates a new personal access token for the user.
// The raw key is returned only once; thereafter only the prefix is visible.
func (s *AuthService) CreateAPIKey(ctx context.Context, userID, name string, expiresAt *time.Time) (*APIKeyResult, error) {
	// Generate: "ts_" + 32 random bytes as lowercase hex (67 chars total)
	raw, err := crypto.RandomBytes(32)
	if err != nil {
		return nil, apperrors.Internal("generate api key", err)
	}
	rawKey := "ts_" + fmt.Sprintf("%x", raw)
	keyPrefix := rawKey[:12] // "ts_" + 9 hex chars

	keyHash := hashAPIKey(rawKey)

	k, err := s.repo.CreateAPIKey(ctx, userID, name, keyHash, keyPrefix, expiresAt)
	if err != nil {
		return nil, err
	}

	s.publishAudit(ctx, AuditEvent{
		Action: "apikey.created", UserID: userID, Success: true, Timestamp: time.Now(),
	})
	return &APIKeyResult{APIKey: k, RawKey: rawKey}, nil
}

func (s *AuthService) ListAPIKeys(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	return s.repo.ListAPIKeys(ctx, userID)
}

func (s *AuthService) DeleteAPIKey(ctx context.Context, id, userID string) error {
	err := s.repo.DeleteAPIKey(ctx, id, userID)
	if err == nil {
		s.publishAudit(ctx, AuditEvent{
			Action: "apikey.deleted", UserID: userID, Success: true, Timestamp: time.Now(),
		})
	}
	return err
}

// hashAPIKey returns SHA-256 hex of the raw key string (no token re-use).
func hashAPIKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", h)
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

	ok, err := crypto.VerifyPassword(p.CurrentPassword, user.PasswordHash, s.cfg.App.Pepper)
	if err != nil {
		return apperrors.Internal("verify current password", err)
	}
	if !ok {
		return apperrors.Unauthorized("current password is incorrect")
	}

	if len(p.NewPassword) < 8 {
		return apperrors.Validation("new password must be at least 8 characters")
	}

	newHash, err := crypto.HashPassword(p.NewPassword, s.cfg.App.Pepper)
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
