package service

// Transfer service unit tests.
// Tests live in package service to access the transferRepository interface.

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── stub repo ─────────────────────────────────────────────────────────────────

type stubTransferRepo struct {
	transfers map[string]*domain.Transfer // keyed by slug
	byID      map[string]*domain.Transfer // keyed by id
	fileIDs   map[string][]string         // transfer id → file ids
	err       error
}

func newStubTransferRepo() *stubTransferRepo {
	return &stubTransferRepo{
		transfers: make(map[string]*domain.Transfer),
		byID:      make(map[string]*domain.Transfer),
		fileIDs:   make(map[string][]string),
	}
}

func (r *stubTransferRepo) Create(_ context.Context, t *domain.Transfer, fileIDs []string) (*domain.Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	t.ID = "transfer-" + t.Slug
	r.transfers[t.Slug] = t
	r.byID[t.ID] = t
	r.fileIDs[t.ID] = fileIDs
	return t, nil
}

func (r *stubTransferRepo) GetBySlug(_ context.Context, slug string) (*domain.Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	t, ok := r.transfers[slug]
	if !ok {
		return nil, apperrors.NotFound("transfer not found")
	}
	return t, nil
}

func (r *stubTransferRepo) GetByID(_ context.Context, id string) (*domain.Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	t, ok := r.byID[id]
	if !ok {
		return nil, apperrors.NotFound("transfer not found")
	}
	return t, nil
}

func (r *stubTransferRepo) ListByOwner(_ context.Context, ownerID string, _, _ int) ([]*domain.Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out []*domain.Transfer
	for _, t := range r.byID {
		if t.OwnerID == ownerID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *stubTransferRepo) GetFileIDs(_ context.Context, transferID string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.fileIDs[transferID], nil
}

func (r *stubTransferRepo) IncrementDownloads(_ context.Context, id string) error {
	if t, ok := r.byID[id]; ok {
		t.DownloadCount++
	}
	return r.err
}

func (r *stubTransferRepo) Revoke(_ context.Context, id, ownerID string) error {
	if r.err != nil {
		return r.err
	}
	t, ok := r.byID[id]
	if !ok || t.OwnerID != ownerID {
		return apperrors.NotFound("transfer not found")
	}
	t.IsRevoked = true
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestTransferService(repo transferRepository) *TransferService {
	return &TransferService{
		repo: repo,
		cfg: &config.Config{
			App: config.AppConfig{Pepper: "test-pepper-1234567890123"},
		},
		log: zap.NewNop(),
	}
}

func createTransferHelper(t *testing.T, svc *TransferService, password string, expiresIn time.Duration, fileIDs []string) *domain.Transfer {
	t.Helper()
	if expiresIn == 0 {
		expiresIn = 24 * time.Hour
	}
	if len(fileIDs) == 0 {
		fileIDs = []string{"file-uuid-0000-0000-0000-000000000001"}
	}
	result, err := svc.Create(context.Background(), CreateParams{
		OwnerID:   "owner-1",
		Name:      "Test Transfer",
		FileIDs:   fileIDs,
		Password:  password,
		ExpiresIn: expiresIn,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return result.Transfer
}

// ── Create validation ─────────────────────────────────────────────────────────

func TestCreate_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	if tr.Slug == "" {
		t.Fatal("expected non-empty slug")
	}
	if tr.OwnerID != "owner-1" {
		t.Errorf("OwnerID = %q, want %q", tr.OwnerID, "owner-1")
	}
}

func TestCreate_EmptyName_ReturnsValidation(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		OwnerID:   "owner-1",
		Name:      "   ",
		FileIDs:   []string{"file-id"},
		ExpiresIn: 24 * time.Hour,
	})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeValidation {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreate_NoFiles_ReturnsValidation(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		OwnerID:   "owner-1",
		Name:      "Test",
		FileIDs:   []string{},
		ExpiresIn: 24 * time.Hour,
	})
	if err == nil {
		t.Fatal("expected validation error for no files")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeValidation {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreate_ExpiresInZero_ReturnsValidation(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		OwnerID:   "owner-1",
		Name:      "Test",
		FileIDs:   []string{"file-id"},
		ExpiresIn: 0,
	})
	if err == nil {
		t.Fatal("expected validation error for zero expiry")
	}
}

func TestCreate_ExpiresInTooLong_ReturnsValidation(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		OwnerID:   "owner-1",
		Name:      "Test",
		FileIDs:   []string{"file-id"},
		ExpiresIn: 91 * 24 * time.Hour, // 91 days > 90-day max
	})
	if err == nil {
		t.Fatal("expected validation error for expiry exceeding 90 days")
	}
}

// ── Access ─────────────────────────────────────────────────────────────────────

func TestAccess_NoPassword_Success(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	tr := createTransferHelper(t, svc, "", 0, nil)

	result, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug})
	if err != nil {
		t.Fatalf("Access: %v", err)
	}
	if result.Transfer.Slug != tr.Slug {
		t.Errorf("Slug = %q", result.Transfer.Slug)
	}
}

func TestAccess_CorrectPassword_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secretpass", 0, nil)

	_, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug, Password: "secretpass"})
	if err != nil {
		t.Fatalf("Access with correct password: %v", err)
	}
}

func TestAccess_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secretpass", 0, nil)

	_, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug, Password: "wrongpass"})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAccess_MissingPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secretpass", 0, nil)

	_, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug, Password: ""})
	if err == nil {
		t.Fatal("expected error when password is required but not provided")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAccess_RevokedTransfer_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	tr := createTransferHelper(t, svc, "", 0, nil)

	// Revoke it
	svc.Revoke(context.Background(), tr.ID, "owner-1") //nolint:errcheck

	_, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug})
	if err == nil {
		t.Fatal("expected error for revoked transfer")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAccess_ExpiredTransfer_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)

	// Create with a past expiry directly in the repo
	past := time.Now().Add(-1 * time.Hour)
	tr := &domain.Transfer{
		ID:        "transfer-expired",
		Slug:      "expired-slug",
		OwnerID:   "owner-1",
		ExpiresAt: &past,
	}
	repo.transfers[tr.Slug] = tr
	repo.byID[tr.ID] = tr
	repo.fileIDs[tr.ID] = []string{"f1"}

	_, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug})
	if err == nil {
		t.Fatal("expected error for expired transfer")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAccess_DownloadLimitReached_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)

	future := time.Now().Add(time.Hour)
	tr := &domain.Transfer{
		ID:            "transfer-limited",
		Slug:          "limited-slug",
		OwnerID:       "owner-1",
		MaxDownloads:  2,
		DownloadCount: 2, // already at limit
		ExpiresAt:     &future,
	}
	repo.transfers[tr.Slug] = tr
	repo.byID[tr.ID] = tr
	repo.fileIDs[tr.ID] = []string{"f1"}

	_, err := svc.Access(context.Background(), AccessParams{Slug: tr.Slug})
	if err == nil {
		t.Fatal("expected error for download limit reached")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAccess_NonExistentSlug_ReturnsNotFound(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	_, err := svc.Access(context.Background(), AccessParams{Slug: "doesnotexist"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeNotFound {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── Get / ownership enforcement ───────────────────────────────────────────────

func TestGet_WrongOwner_ReturnsForbidden(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	_, _, err := svc.Get(context.Background(), tr.ID, "wrong-owner")
	if err == nil {
		t.Fatal("expected forbidden error for wrong owner")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGet_CorrectOwner_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	got, _, err := svc.Get(context.Background(), tr.ID, "owner-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != tr.ID {
		t.Errorf("ID = %q, want %q", got.ID, tr.ID)
	}
}

// ── Revoke ────────────────────────────────────────────────────────────────────

func TestRevoke_OwnerCanRevoke(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	if err := svc.Revoke(context.Background(), tr.ID, "owner-1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
}

func TestRevoke_WrongOwner_ReturnsNotFound(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	err := svc.Revoke(context.Background(), tr.ID, "not-the-owner")
	if err == nil {
		t.Fatal("expected error revoking with wrong owner")
	}
}
