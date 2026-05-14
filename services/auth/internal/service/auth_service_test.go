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
	"testing"
	"time"

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
	err           error // if set, every method returns this error
}

func newStubRepo() *stubUserRepo {
	return &stubUserRepo{
		users:         make(map[string]*domain.User),
		usersByID:     make(map[string]*domain.User),
		refreshTokens: make(map[string]*domain.RefreshToken),
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

func (r *stubUserRepo) GetMFASecret(_ context.Context, _ string) (*domain.MFASecret, error) {
	return nil, apperrors.NotFound("mfa not configured")
}

func (r *stubUserRepo) UpsertMFASecret(_ context.Context, _, _ string) error    { return r.err }
func (r *stubUserRepo) EnableMFA(_ context.Context, _ string) error             { return r.err }
func (r *stubUserRepo) RecordSuccessfulLogin(_ context.Context, _ string) error { return r.err }
func (r *stubUserRepo) UpdatePassword(_ context.Context, _, _ string) error     { return r.err }

func (r *stubUserRepo) RecordFailedLogin(_ context.Context, id string, _ int, _ time.Duration) error {
	if u, ok := r.usersByID[id]; ok {
		u.FailedLoginAttempts++
	}
	return r.err
}

func (r *stubUserRepo) StorePasswordResetToken(_ context.Context, _, _ string, _ time.Time) error {
	return r.err
}

func (r *stubUserRepo) ConsumePasswordResetToken(_ context.Context, _ string) (*domain.PasswordResetToken, error) {
	return nil, apperrors.Unauthorized("invalid or expired reset token")
}

func (r *stubUserRepo) CreateAPIKey(_ context.Context, userID, name, keyHash, prefix string, expiresAt *time.Time) (*domain.APIKey, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &domain.APIKey{ID: "key-1", UserID: userID, Name: name, KeyPrefix: prefix, KeyHash: keyHash, ExpiresAt: expiresAt}, nil
}

func (r *stubUserRepo) ListAPIKeys(_ context.Context, _ string) ([]*domain.APIKey, error) {
	return nil, r.err
}

func (r *stubUserRepo) DeleteAPIKey(_ context.Context, _, _ string) error { return r.err }

func (r *stubUserRepo) GetAPIKeyByHash(_ context.Context, _ string) (*domain.APIKey, error) {
	return nil, r.err
}

func (r *stubUserRepo) ListContacts(_ context.Context, _ string) ([]*domain.Contact, error) {
	return nil, r.err
}
func (r *stubUserRepo) CreateContact(_ context.Context, _, _, _ string) (*domain.Contact, error) {
	return nil, r.err
}
func (r *stubUserRepo) UpdateContactName(_ context.Context, _, _, _ string) error { return r.err }
func (r *stubUserRepo) DeleteContact(_ context.Context, _, _ string) error        { return r.err }
func (r *stubUserRepo) UpdateAutoSaveContacts(_ context.Context, _ string, _ bool) error {
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
