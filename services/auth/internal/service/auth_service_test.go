package service

// Auth service unit tests.
// Tests live in package service (not service_test) to access unexported struct fields.
// All tests use stub implementations of userRepository — no real DB required.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── stub repo ─────────────────────────────────────────────────────────────────

// stubUserRepo is a minimal in-memory implementation of userRepository for tests.
type stubUserRepo struct {
	users         map[string]*domain.User // keyed by email
	usersByID     map[string]*domain.User // keyed by id
	refreshTokens map[string]*domain.RefreshToken
	mfaSecrets    map[string]*domain.MFASecret          // keyed by userID
	resetTokens   map[string]*domain.PasswordResetToken // keyed by raw token
	storedAPIKeys map[string][]*domain.APIKey           // keyed by userID
	err           error                                 // if set, every method returns this error
}

func newStubRepo() *stubUserRepo {
	return &stubUserRepo{
		users:         make(map[string]*domain.User),
		usersByID:     make(map[string]*domain.User),
		refreshTokens: make(map[string]*domain.RefreshToken),
		mfaSecrets:    make(map[string]*domain.MFASecret),
		resetTokens:   make(map[string]*domain.PasswordResetToken),
		storedAPIKeys: make(map[string][]*domain.APIKey),
	}
}

func (r *stubUserRepo) addUser(u *domain.User) {
	r.users[u.Email] = u
	r.usersByID[u.ID] = u
}

func (r *stubUserRepo) Create(_ context.Context, email, passwordHash string, role domain.Role) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	if _, exists := r.users[email]; exists {
		return nil, apperrors.Conflict("email already registered")
	}
	u := &domain.User{
		ID:           "user-" + email,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		IsActive:     true,
	}
	r.addUser(u)
	return u, nil
}

func (r *stubUserRepo) UpsertAdminByEmail(_ context.Context, email, hash string) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	u := &domain.User{ID: "admin-1", Email: email, PasswordHash: hash, Role: domain.RoleAdmin, IsActive: true}
	r.addUser(u)
	return u, nil
}

func (r *stubUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	u, ok := r.users[email]
	if !ok {
		return nil, apperrors.NotFound("user not found")
	}
	return u, nil
}

func (r *stubUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	u, ok := r.usersByID[id]
	if !ok {
		return nil, apperrors.NotFound("user not found")
	}
	return u, nil
}

func (r *stubUserRepo) StoreRefreshToken(_ context.Context, userID, rawToken string, expiresAt time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.refreshTokens[rawToken] = &domain.RefreshToken{UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (r *stubUserRepo) ConsumeRefreshToken(_ context.Context, rawToken string) (*domain.RefreshToken, error) {
	if r.err != nil {
		return nil, r.err
	}
	rt, ok := r.refreshTokens[rawToken]
	if !ok {
		return nil, apperrors.Unauthorized("invalid or expired refresh token")
	}
	delete(r.refreshTokens, rawToken)
	return rt, nil
}

func (r *stubUserRepo) RevokeAllRefreshTokens(_ context.Context, _ string) error { return r.err }

func (r *stubUserRepo) GetMFASecret(_ context.Context, userID string) (*domain.MFASecret, error) {
	if s, ok := r.mfaSecrets[userID]; ok {
		return s, nil
	}
	return nil, apperrors.NotFound("mfa not configured")
}

func (r *stubUserRepo) UpsertMFASecret(_ context.Context, userID, encryptedSecret string) error {
	if r.err != nil {
		return r.err
	}
	r.mfaSecrets[userID] = &domain.MFASecret{UserID: userID, Secret: encryptedSecret}
	return nil
}
func (r *stubUserRepo) EnableMFA(_ context.Context, _ string) error             { return r.err }
func (r *stubUserRepo) RecordSuccessfulLogin(_ context.Context, _ string) error { return r.err }
func (r *stubUserRepo) UpdatePassword(_ context.Context, userID, newHash string) error {
	if r.err != nil {
		return r.err
	}
	if u, ok := r.usersByID[userID]; ok {
		u.PasswordHash = newHash
	}
	return nil
}

func (r *stubUserRepo) RecordFailedLogin(_ context.Context, id string, _ int, _ time.Duration) error {
	if u, ok := r.usersByID[id]; ok {
		u.FailedLoginAttempts++
	}
	return r.err
}

func (r *stubUserRepo) StorePasswordResetToken(_ context.Context, _, _ string, _ time.Time) error {
	return r.err
}

func (r *stubUserRepo) ConsumePasswordResetToken(_ context.Context, rawToken string) (*domain.PasswordResetToken, error) {
	if r.err != nil {
		return nil, r.err
	}
	if rt, ok := r.resetTokens[rawToken]; ok {
		delete(r.resetTokens, rawToken)
		return rt, nil
	}
	return nil, apperrors.Unauthorized("invalid or expired reset token")
}

func (r *stubUserRepo) CreateAPIKey(_ context.Context, userID, name, keyHash, prefix string, expiresAt *time.Time) (*domain.APIKey, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &domain.APIKey{ID: "key-1", UserID: userID, Name: name, KeyPrefix: prefix, KeyHash: keyHash, ExpiresAt: expiresAt}, nil
}

func (r *stubUserRepo) ListAPIKeys(_ context.Context, userID string) ([]*domain.APIKey, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.storedAPIKeys[userID], nil
}

func (r *stubUserRepo) DeleteAPIKey(_ context.Context, _, _ string) error { return r.err }

func (r *stubUserRepo) UpdatePreferences(_ context.Context, _ string, _, _, _ *string) error {
	return r.err
}

func (r *stubUserRepo) GetLockoutConfig(_ context.Context) (int, time.Duration, error) {
	return 10, 15 * time.Minute, nil
}

// ── test helpers ──────────────────────────────────────────────────────────────

func generateRSATestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func newTestAuthService(t *testing.T, repo userRepository) *AuthService {
	t.Helper()
	k := generateRSATestKey(t)
	pepper := "test-pepper-12345678901234567890" // >= 8 chars
	mfaKey, err := deriveMFAKey(pepper)
	if err != nil {
		t.Fatal(err)
	}
	return &AuthService{
		repo:       repo,
		cfg:        testConfig(pepper),
		log:        zap.NewNop(),
		privateKey: k,
		publicKey:  &k.PublicKey,
		mfaKey:     mfaKey,
	}
}

func testConfig(pepper string) *config.Config {
	return &config.Config{
		App: config.AppConfig{Pepper: pepper},
		JWT: config.JWTConfig{
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 168 * time.Hour,
		},
	}
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	u, err := svc.Register(context.Background(), "alice@example.com", "password123", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", u.Email, "alice@example.com")
	}
	if u.Role != domain.RoleUser {
		t.Errorf("Role = %q, want %q", u.Role, domain.RoleUser)
	}
}

func TestRegister_DuplicateEmail_ReturnsConflict(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	svc.Register(context.Background(), "alice@example.com", "password123", "") //nolint:errcheck
	_, err := svc.Register(context.Background(), "alice@example.com", "other", "")
	if err == nil {
		t.Fatal("expected conflict error for duplicate email")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != apperrors.CodeConflict {
		t.Errorf("code = %q, want %q", ae.Code, apperrors.CodeConflict)
	}
}

func TestRegister_RepoError_Propagated(t *testing.T) {
	repo := newStubRepo()
	repo.err = errors.New("db down")
	svc := newTestAuthService(t, repo)
	_, err := svc.Register(context.Background(), "a@b.com", "pass", "")
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func registerUser(t *testing.T, svc *AuthService, email, password string) *domain.User {
	t.Helper()
	u, err := svc.Register(context.Background(), email, password, "")
	if err != nil {
		t.Fatalf("register helper: %v", err)
	}
	return u
}

func TestLogin_Success(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	registerUser(t, svc, "bob@example.com", "correcthorse")

	pair, user, err := svc.Login(context.Background(), "bob@example.com", "correcthorse", "")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair == nil || pair.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if user.Email != "bob@example.com" {
		t.Errorf("Email = %q", user.Email)
	}
}

func TestLogin_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	registerUser(t, svc, "carol@example.com", "rightpass")

	_, _, err := svc.Login(context.Background(), "carol@example.com", "wrongpass", "")
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("code = %q, want %q", ae.Code, apperrors.CodeUnauthorized)
	}
}

func TestLogin_UnknownEmail_ReturnsUnauthorized(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())

	_, _, err := svc.Login(context.Background(), "ghost@example.com", "anypass", "")
	if err == nil {
		t.Fatal("expected unauthorized error for unknown email")
	}
	var ae *apperrors.AppError
	if errors.As(err, &ae) && ae.Code == apperrors.CodeUnauthorized {
		return // expected
	}
	t.Errorf("unexpected error: %v", err)
}

func TestLogin_DisabledAccount_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)

	u := registerUser(t, svc, "dave@example.com", "pass")
	// Disable the account directly in the stub
	repo.users[u.Email].IsActive = false
	repo.usersByID[u.ID].IsActive = false

	_, _, err := svc.Login(context.Background(), "dave@example.com", "pass", "")
	if err == nil {
		t.Fatal("expected error for disabled account")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_LockedAccount_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)

	u := registerUser(t, svc, "eve@example.com", "pass")
	// Lock the account
	future := time.Now().Add(15 * time.Minute)
	repo.users[u.Email].LockedUntil = &future
	repo.usersByID[u.ID].LockedUntil = &future

	_, _, err := svc.Login(context.Background(), "eve@example.com", "pass", "")
	if err == nil {
		t.Fatal("expected error for locked account")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── ValidateAccessToken ───────────────────────────────────────────────────────

func TestValidateAccessToken_ValidToken_ReturnsClaims(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())

	registerUser(t, svc, "frank@example.com", "pass12345")
	pair, _, err := svc.Login(context.Background(), "frank@example.com", "pass12345", "")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.Email != "frank@example.com" {
		t.Errorf("claims.Email = %q", claims.Email)
	}
}

func TestValidateAccessToken_Garbage_ReturnsError(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	_, err := svc.ValidateAccessToken("not.a.jwt")
	if err == nil {
		t.Fatal("expected error for garbage token")
	}
}

func TestValidateAccessToken_WrongKey_ReturnsError(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	otherSvc := newTestAuthService(t, newStubRepo())

	// Register + login on otherSvc, validate on svc (different key)
	registerUser(t, otherSvc, "g@g.com", "pass12345")
	pair, _, err := otherSvc.Login(context.Background(), "g@g.com", "pass12345", "")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	_, err = svc.ValidateAccessToken(pair.AccessToken)
	if err == nil {
		t.Fatal("expected error: token signed by a different key")
	}
}

// ── CreateAPIKey ──────────────────────────────────────────────────────────────

func TestCreateAPIKey_ReturnsRawKeyOnce(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	result, err := svc.CreateAPIKey(context.Background(), "user-1", "my-key", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if result.RawKey == "" {
		t.Fatal("expected non-empty raw key")
	}
	if !startsWith(result.RawKey, "ts_") {
		t.Errorf("raw key %q should start with 'ts_'", result.RawKey)
	}
	if result.APIKey.KeyPrefix != result.RawKey[:12] {
		t.Errorf("prefix mismatch: stored %q, key starts with %q", result.APIKey.KeyPrefix, result.RawKey[:12])
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ── ChangePassword ────────────────────────────────────────────────────────────

func TestChangePassword_Success(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	u := registerUser(t, svc, "hank@example.com", "oldpass123")

	err := svc.ChangePassword(context.Background(), ChangePasswordParams{
		UserID:          u.ID,
		CurrentPassword: "oldpass123",
		NewPassword:     "newpass456",
	})
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
}

func TestChangePassword_WrongCurrentPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	u := registerUser(t, svc, "ivan@example.com", "realpass123")

	err := svc.ChangePassword(context.Background(), ChangePasswordParams{
		UserID:          u.ID,
		CurrentPassword: "wrongpass",
		NewPassword:     "newpass456",
	})
	if err == nil {
		t.Fatal("expected error for wrong current password")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestChangePassword_TooShortNewPassword_ReturnsValidation(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	u := registerUser(t, svc, "jane@example.com", "goodpassword")

	err := svc.ChangePassword(context.Background(), ChangePasswordParams{
		UserID:          u.ID,
		CurrentPassword: "goodpassword",
		NewPassword:     "short",
	})
	if err == nil {
		t.Fatal("expected validation error for short new password")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeValidation {
		t.Errorf("unexpected error: %v (code: %v)", err, ae)
	}
}

// ── deriveMFAKey ──────────────────────────────────────────────────────────────

func TestDeriveMFAKey_EmptyPepper_ReturnsError(t *testing.T) {
	_, err := deriveMFAKey("")
	if err == nil {
		t.Fatal("expected error for empty pepper")
	}
}

func TestDeriveMFAKey_ShortPepper_Pads(t *testing.T) {
	key, err := deriveMFAKey("shortpepper")
	if err != nil {
		t.Fatalf("deriveMFAKey: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}
}

func TestDeriveMFAKey_64HexPepper_DecodesAsBytes(t *testing.T) {
	// 64 hex chars = 32 bytes exactly
	hexPepper := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	key, err := deriveMFAKey(hexPepper)
	if err != nil {
		t.Fatalf("deriveMFAKey: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}
}

// ── parseRSAPrivateKey (used by New) ──────────────────────────────────────────

func TestParseRSAPrivateKey_EmptyString_ReturnsError(t *testing.T) {
	_, err := parseRSAPrivateKey("")
	if err == nil {
		t.Fatal("expected error for empty PEM")
	}
}

func TestParseRSAPrivateKey_PKCS8_Valid(t *testing.T) {
	k := generateRSATestKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	parsed, err := parseRSAPrivateKey(string(pemBytes))
	if err != nil {
		t.Fatalf("parseRSAPrivateKey: %v", err)
	}
	if parsed.N.Cmp(k.N) != 0 {
		t.Fatal("parsed key modulus mismatch")
	}
}

// ── EnsureBootstrapAdmin ──────────────────────────────────────────────────────

func TestEnsureBootstrapAdmin_CreatesAdmin(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)

	err := svc.EnsureBootstrapAdmin(context.Background(), "admin@example.com", "Str0ng!Pass")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	u, ok := repo.users["admin@example.com"]
	if !ok {
		t.Fatal("admin user was not stored in repo")
	}
	if u.Role != domain.RoleAdmin {
		t.Errorf("expected role admin, got %s", u.Role)
	}
}

func TestEnsureBootstrapAdmin_RepoError(t *testing.T) {
	repo := newStubRepo()
	repo.err = errors.New("db error")
	svc := newTestAuthService(t, repo)

	err := svc.EnsureBootstrapAdmin(context.Background(), "admin@example.com", "Str0ng!Pass")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── Login MFA-required path ───────────────────────────────────────────────────

func TestLogin_MFAEnabled_ReturnsMFARequired(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	_, err := svc.Register(ctx, "mfa@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	// Enable MFA on the user.
	repo.users["mfa@example.com"].MFAEnabled = true

	pair, user, err := svc.Login(ctx, "mfa@example.com", "password123", "")
	if pair != nil {
		t.Fatal("expected nil TokenPair when MFA is required")
	}
	if user == nil || user.Email != "mfa@example.com" {
		t.Fatal("expected user returned for MFA step")
	}
	if !apperrors.IsUnauthorized(err) || err.Error() != "mfa_required" {
		t.Errorf("expected 'mfa_required' unauthorized, got %v", err)
	}
}

// ── RefreshTokens ─────────────────────────────────────────────────────────────

func TestRefreshTokens_Success(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	_, err := svc.Register(ctx, "user@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	pair, _, err := svc.Login(ctx, "user@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}

	newPair, err := svc.RefreshTokens(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshTokens: %v", err)
	}
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatal("expected new token pair")
	}
	// Old refresh token should be consumed — second use should fail.
	_, err = svc.RefreshTokens(ctx, pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error on second use of same refresh token")
	}
}

func TestRefreshTokens_InvalidToken_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)

	_, err := svc.RefreshTokens(context.Background(), "totally-invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid refresh token")
	}
}

func TestRefreshTokens_DisabledUser_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	_, err := svc.Register(ctx, "disabled@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	pair, _, err := svc.Login(ctx, "disabled@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}

	// Disable the account after login.
	repo.users["disabled@example.com"].IsActive = false

	_, err = svc.RefreshTokens(ctx, pair.RefreshToken)
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error for disabled user, got %v", err)
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	_, err := svc.Register(ctx, "logout@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Logout(ctx, "user-logout@example.com")
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestLogout_RepoError(t *testing.T) {
	repo := newStubRepo()
	repo.err = errors.New("db error")
	svc := newTestAuthService(t, repo)

	err := svc.Logout(context.Background(), "some-user-id")
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

// ── GetMe ─────────────────────────────────────────────────────────────────────

func TestGetMe_Success(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "me@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetMe(ctx, registered.ID)
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if got.Email != "me@example.com" {
		t.Errorf("expected email me@example.com, got %s", got.Email)
	}
}

func TestGetMe_UserNotFound_ReturnsError(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)

	_, err := svc.GetMe(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

// ── RevokeAccessToken / IsTokenRevoked (nil cache path) ──────────────────────

func TestRevokeAccessToken_NilCache_ReturnsNil(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo()) // cache is nil by default
	err := svc.RevokeAccessToken(context.Background(), "some-jti")
	if err != nil {
		t.Fatalf("expected no error with nil cache, got %v", err)
	}
}

func TestRevokeAccessToken_EmptyJTI_ReturnsNil(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	err := svc.RevokeAccessToken(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error for empty JTI, got %v", err)
	}
}

func TestIsTokenRevoked_NilCache_ReturnsFalse(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	revoked := svc.IsTokenRevoked(context.Background(), "some-jti")
	if revoked {
		t.Fatal("expected false with nil cache")
	}
}

// ── ListAPIKeys / DeleteAPIKey ────────────────────────────────────────────────

func TestListAPIKeys_ReturnsStoredKeys(t *testing.T) {
	repo := newStubRepo()
	repo.storedAPIKeys["user-1"] = []*domain.APIKey{
		{ID: "key-1", Name: "CI key", UserID: "user-1"},
		{ID: "key-2", Name: "Deploy key", UserID: "user-1"},
	}
	svc := newTestAuthService(t, repo)

	keys, err := svc.ListAPIKeys(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestListAPIKeys_EmptyForUnknownUser(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())

	keys, err := svc.ListAPIKeys(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestDeleteAPIKey_Success(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())
	err := svc.DeleteAPIKey(context.Background(), "key-1", "user-1")
	if err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
}

func TestDeleteAPIKey_RepoError(t *testing.T) {
	repo := newStubRepo()
	repo.err = errors.New("not found")
	svc := newTestAuthService(t, repo)

	err := svc.DeleteAPIKey(context.Background(), "key-1", "user-1")
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

// ── RequestPasswordReset ──────────────────────────────────────────────────────

func TestRequestPasswordReset_UnknownEmail_ReturnsNilSilently(t *testing.T) {
	// Security: unknown email must NOT return an error (avoids user enumeration).
	svc := newTestAuthService(t, newStubRepo())

	uid, tok, err := svc.RequestPasswordReset(context.Background(), "nobody@example.com", "1.2.3.4")
	if err != nil {
		t.Fatalf("expected silent nil for unknown email, got %v", err)
	}
	if uid != "" || tok != "" {
		t.Errorf("expected empty uid/token for unknown email, got uid=%q tok=%q", uid, tok)
	}
}

func TestRequestPasswordReset_KnownUser_ReturnsToken(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	_, err := svc.Register(ctx, "reset@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}

	uid, tok, err := svc.RequestPasswordReset(ctx, "reset@example.com", "")
	if err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if uid == "" || tok == "" {
		t.Error("expected non-empty uid and token")
	}
}

// ── ConfirmPasswordReset ──────────────────────────────────────────────────────

func TestConfirmPasswordReset_InvalidToken_ReturnsUnauthorized(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())

	err := svc.ConfirmPasswordReset(context.Background(), "bad-token", "newpassword1")
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestConfirmPasswordReset_ValidToken_UpdatesPassword(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	// Register user and request a reset.
	_, err := svc.Register(ctx, "confirm@example.com", "oldpassword", "")
	if err != nil {
		t.Fatal(err)
	}
	uid, tok, err := svc.RequestPasswordReset(ctx, "confirm@example.com", "")
	if err != nil || tok == "" {
		t.Fatalf("RequestPasswordReset failed: err=%v tok=%q", err, tok)
	}

	// Seed the stub's resetTokens so ConsumePasswordResetToken succeeds.
	repo.resetTokens[tok] = &domain.PasswordResetToken{UserID: uid}

	err = svc.ConfirmPasswordReset(ctx, tok, "Newpassword123!")
	if err != nil {
		t.Fatalf("ConfirmPasswordReset: %v", err)
	}
	// Confirm the old password no longer works.
	_, _, loginErr := svc.Login(ctx, "confirm@example.com", "oldpassword", "")
	if loginErr == nil {
		t.Fatal("old password should have been invalidated")
	}
}

// ── SetupMFA ──────────────────────────────────────────────────────────────────

func TestSetupMFA_ValidUser_ReturnsProvisioningURL(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "mfasetup@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := svc.SetupMFA(ctx, registered.ID)
	if err != nil {
		t.Fatalf("SetupMFA: %v", err)
	}
	if result.Secret == "" {
		t.Error("expected non-empty TOTP secret")
	}
	if !strings.HasPrefix(result.ProvisioningURI, "otpauth://totp/") {
		t.Errorf("expected otpauth URI, got %q", result.ProvisioningURI)
	}
	// Verify secret was persisted in stub.
	if _, ok := repo.mfaSecrets[registered.ID]; !ok {
		t.Error("MFA secret was not stored in repo")
	}
}

func TestSetupMFA_UnknownUser_ReturnsError(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())

	_, err := svc.SetupMFA(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

// ── VerifyMFA ─────────────────────────────────────────────────────────────────

func TestVerifyMFA_NoMFASecret_ReturnsError(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "nomfa@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	// No SetupMFA called — GetMFASecret will return NotFound.
	err = svc.VerifyMFA(ctx, registered.ID, "123456")
	if err == nil {
		t.Fatal("expected error when MFA not configured")
	}
}

func TestVerifyMFA_WrongOTP_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "verify@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	// Set up MFA first so a secret is stored.
	_, err = svc.SetupMFA(ctx, registered.ID)
	if err != nil {
		t.Fatalf("SetupMFA: %v", err)
	}

	err = svc.VerifyMFA(ctx, registered.ID, "000000") // wrong code
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error for wrong OTP, got %v", err)
	}
}

func TestVerifyMFA_ValidOTP_EnablesMFA(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "validotp@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.SetupMFA(ctx, registered.ID)
	if err != nil {
		t.Fatalf("SetupMFA: %v", err)
	}

	// Generate a valid code for the current time window.
	code, err := totp.GenerateCode(result.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	err = svc.VerifyMFA(ctx, registered.ID, code)
	if err != nil {
		t.Fatalf("VerifyMFA with valid code: %v", err)
	}
}

// ── LoginWithMFA ──────────────────────────────────────────────────────────────

func TestLoginWithMFA_UnknownUser_ReturnsUnauthorized(t *testing.T) {
	svc := newTestAuthService(t, newStubRepo())

	_, err := svc.LoginWithMFA(context.Background(), "nonexistent-id", "123456")
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized for unknown user, got %v", err)
	}
}

func TestLoginWithMFA_NoMFASecret_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "mfalogin@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	// No MFA setup — secret won't be in repo.
	_, err = svc.LoginWithMFA(ctx, registered.ID, "123456")
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized when MFA not configured, got %v", err)
	}
}

func TestLoginWithMFA_ValidOTP_ReturnsTokenPair(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "mfaok@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	setupResult, err := svc.SetupMFA(ctx, registered.ID)
	if err != nil {
		t.Fatalf("SetupMFA: %v", err)
	}

	code, err := totp.GenerateCode(setupResult.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	pair, err := svc.LoginWithMFA(ctx, registered.ID, code)
	if err != nil {
		t.Fatalf("LoginWithMFA: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty token pair")
	}
}

func TestLoginWithMFA_WrongOTP_ReturnsUnauthorized(t *testing.T) {
	repo := newStubRepo()
	svc := newTestAuthService(t, repo)
	ctx := context.Background()

	registered, err := svc.Register(ctx, "mfabadotp@example.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetupMFA(ctx, registered.ID)
	if err != nil {
		t.Fatalf("SetupMFA: %v", err)
	}

	_, err = svc.LoginWithMFA(ctx, registered.ID, "000000") // wrong code
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized for wrong OTP, got %v", err)
	}
}
