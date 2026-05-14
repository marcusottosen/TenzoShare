package service

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
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
	GetLockoutConfig(ctx context.Context) (maxAttempts int, lockDuration time.Duration, err error)
	CreateAPIKey(ctx context.Context, userID, name, keyHash, keyPrefix string, expiresAt *time.Time) (*domain.APIKey, error)
	ListAPIKeys(ctx context.Context, userID string) ([]*domain.APIKey, error)
	DeleteAPIKey(ctx context.Context, id, userID string) error
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	ListContacts(ctx context.Context, userID string) ([]*domain.Contact, error)
	CreateContact(ctx context.Context, userID, email, name string) (*domain.Contact, error)
	UpdateContactName(ctx context.Context, id, userID, name string) error
	DeleteContact(ctx context.Context, id, userID string) error
	UpdateAutoSaveContacts(ctx context.Context, userID string, autoSave bool) error
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

// deriveMFAKey derives a 32-byte AES key from the password pepper using
// HKDF-SHA256 (RFC 5869). This is safe regardless of pepper length and avoids
// the zero-padding issue of a direct copy when pepper is shorter than 32 bytes.
func deriveMFAKey(pepper string) ([]byte, error) {
	if pepper == "" {
		return nil, fmt.Errorf("PASSWORD_PEPPER is not set")
	}
	return hkdfSHA256([]byte(pepper), []byte("tenzoshare_mfa_key_v1"), 32), nil
}

// hkdfSHA256 implements HKDF-SHA256 (RFC 5869) using only the standard library.
// ikm is the input key material, info is the context/label, length is output size.
func hkdfSHA256(ikm, info []byte, length int) []byte {
	// Extract: PRK = HMAC-SHA256(salt=zeros[32], IKM)
	salt := make([]byte, sha256.Size)
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	prk := mac.Sum(nil)

	// Expand: OKM = T(1) || T(2) || ... where T(i) = HMAC-SHA256(PRK, T(i-1) || info || i)
	result := make([]byte, 0, length)
	t := []byte{}
	counter := byte(1)
	for len(result) < length {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{counter})
		t = mac.Sum(nil)
		result = append(result, t...)
		counter++
	}
	return result[:length]
}

// lockoutConfig returns the current lockout policy, reading from DB if the
// cached values are older than lockoutConfigTTL (60 s).
func (s *AuthService) lockoutConfig(ctx context.Context) (maxAttempts int, lockDuration time.Duration) {
	s.lockoutMu.Lock()
	defer s.lockoutMu.Unlock()
	if time.Since(s.lockoutCachedAt) < lockoutConfigTTL && s.cachedMaxAttempts > 0 {
		return s.cachedMaxAttempts, s.cachedLockDuration
	}
	ma, ld, err := s.repo.GetLockoutConfig(ctx)
	if err != nil {
		s.log.Warn("lockoutConfig: could not read auth_settings, using fallback", zap.Error(err))
		return 10, 15 * time.Minute
	}
	s.cachedMaxAttempts = ma
	s.cachedLockDuration = ld
	s.lockoutCachedAt = time.Now()
	return ma, ld
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
		}
		if limited {
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

func (s *AuthService) Login(ctx context.Context, email, password, clientIP string) (*TokenPair, *domain.User, error) {
	if clientIP != "" {
		limited, err := s.checkRateLimit(ctx, clientIP)
		if err != nil {
			s.log.Warn("rate limit check failed", zap.Error(err))
		}
		if limited {
			return nil, nil, apperrors.RateLimit("too many login attempts; try again in 15 minutes")
		}
	}

	// Per-email rate limit — prevents distributed credential-stuffing where the
	// attacker rotates source IPs but targets the same account.
	if email != "" && s.cache != nil {
		limited, err := s.checkRateLimitGeneric(ctx, "ratelimit:login:email:"+email, loginRateLimit, loginRateLimitWindow)
		if err != nil {
			s.log.Warn("email rate limit check failed", zap.Error(err))
		}
		if limited {
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

	ok, err := crypto.VerifyPassword(password, user.PasswordHash, s.cfg.App.Pepper)
	if err != nil || !ok {
		maxAttempts, lockDur := s.lockoutConfig(ctx)
		_ = s.repo.RecordFailedLogin(ctx, user.ID, maxAttempts, lockDur)
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
		s.log.Warn("mfa secret not found during mfa login", zap.String("user_id", userID))
		return nil, apperrors.Unauthorized("invalid OTP code")
	}

	encKey := s.mfaEncryptionKey()
	rawSecret, err := crypto.Decrypt(mfaSecret.Secret, encKey)
	if err != nil {
		return nil, apperrors.Internal("decrypt mfa secret", err)
	}

	valid, err := totp.ValidateCustom(otpCode, string(rawSecret), time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // allow 1 step tolerance for clock skew
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
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

	valid, err := totp.ValidateCustom(otpCode, string(rawSecret), time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // allow 1 step tolerance for clock skew
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
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

func (s *AuthService) RequestPasswordReset(ctx context.Context, email, clientIP string) (userID, resetToken string, err error) {
	if clientIP != "" {
		var limited bool
		limited, err = s.checkRateLimitGeneric(ctx, "ratelimit:pwreset:"+clientIP, passwordResetRateLimit, passwordResetRateLimitWindow)
		if err != nil {
			s.log.Warn("password-reset rate-limit check failed", zap.Error(err))
			err = nil
		}
		if limited {
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
	}, jwt.WithIssuer("tenzoshare-auth"), jwt.WithAudience("tenzoshare-api"))
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
			Issuer:    "tenzoshare-auth",
			Audience:  jwt.ClaimStrings{"tenzoshare-api"},
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

func (s *AuthService) checkRateLimit(ctx context.Context, clientIP string) (bool, error) {
	if s.cache == nil {
		s.log.Warn("rate limiter unavailable (cache is nil); denying request as fail-safe", zap.String("key", clientIP))
		return true, fmt.Errorf("rate limiter unavailable")
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
		s.log.Warn("rate limiter unavailable (cache is nil); denying request as fail-safe", zap.String("key", key))
		return true, fmt.Errorf("rate limiter unavailable")
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

// ValidateAPIKey authenticates a raw API key by hashing it and looking it up.
// Expiry is enforced at the database level via GetAPIKeyByHash.
// Returns the matching APIKey record, or Unauthorized if invalid/expired.
func (s *AuthService) ValidateAPIKey(ctx context.Context, rawKey string) (*domain.APIKey, error) {
	if rawKey == "" {
		return nil, apperrors.Unauthorized("api key required")
	}
	keyHash := hashAPIKey(rawKey)
	k, err := s.repo.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		// Map not-found/expired to Unauthorized so callers don't leak key existence.
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, apperrors.Unauthorized("invalid or expired api key")
		}
		return nil, err
	}
	return k, nil
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

// ── Contacts ─────────────────────────────────────────────────────────────────

func (s *AuthService) ListContacts(ctx context.Context, userID string) ([]*domain.Contact, error) {
	return s.repo.ListContacts(ctx, userID)
}

func (s *AuthService) CreateContact(ctx context.Context, userID, email, name string) (*domain.Contact, error) {
	return s.repo.CreateContact(ctx, userID, email, name)
}

func (s *AuthService) UpdateContactName(ctx context.Context, id, userID, name string) error {
	return s.repo.UpdateContactName(ctx, id, userID, name)
}

func (s *AuthService) DeleteContact(ctx context.Context, id, userID string) error {
	return s.repo.DeleteContact(ctx, id, userID)
}

func (s *AuthService) UpdateAutoSaveContacts(ctx context.Context, userID string, autoSave bool) error {
	return s.repo.UpdateAutoSaveContacts(ctx, userID, autoSave)
}
