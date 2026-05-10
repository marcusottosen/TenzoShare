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
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── stub repo ─────────────────────────────────────────────────────────────────

type stubTransferRepo struct {
	transfers    map[string]*domain.Transfer // keyed by slug
	byID         map[string]*domain.Transfer // keyed by id
	fileIDs      map[string][]string         // transfer id → file ids
	fileDlCounts map[string]map[string]int   // transfer id → file id → count
	err          error
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
	// Populate FileCount and IsExhausted from stored state, mirroring the real repo.
	copy := *t
	copy.FileCount = len(r.fileIDs[t.ID])
	copy.IsExhausted = false
	if t.MaxDownloads > 0 && len(r.fileIDs[t.ID]) > 0 {
		allExhausted := true
		for _, fid := range r.fileIDs[t.ID] {
			cnt := 0
			if r.fileDlCounts != nil && r.fileDlCounts[t.ID] != nil {
				cnt = r.fileDlCounts[t.ID][fid]
			}
			if cnt < t.MaxDownloads {
				allExhausted = false
				break
			}
		}
		copy.IsExhausted = allExhausted
	}
	return &copy, nil
}

func (r *stubTransferRepo) GetByID(_ context.Context, id string) (*domain.Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	t, ok := r.byID[id]
	if !ok {
		return nil, apperrors.NotFound("transfer not found")
	}
	// Populate FileCount and IsExhausted from stored state, mirroring the real repo.
	copy := *t
	copy.FileCount = len(r.fileIDs[t.ID])
	copy.IsExhausted = false
	if t.MaxDownloads > 0 && len(r.fileIDs[t.ID]) > 0 {
		allExhausted := true
		for _, fid := range r.fileIDs[t.ID] {
			cnt := 0
			if r.fileDlCounts != nil && r.fileDlCounts[t.ID] != nil {
				cnt = r.fileDlCounts[t.ID][fid]
			}
			if cnt < t.MaxDownloads {
				allExhausted = false
				break
			}
		}
		copy.IsExhausted = allExhausted
	}
	return &copy, nil
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

func (r *stubTransferRepo) AttemptFileDownload(_ context.Context, transferID, fileID string, maxDownloads int) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	if r.fileDlCounts == nil {
		r.fileDlCounts = make(map[string]map[string]int)
	}
	if r.fileDlCounts[transferID] == nil {
		r.fileDlCounts[transferID] = make(map[string]int)
	}
	current := r.fileDlCounts[transferID][fileID]
	if current >= maxDownloads {
		return false, nil
	}
	r.fileDlCounts[transferID][fileID] = current + 1
	return true, nil
}

func (r *stubTransferRepo) GetFileInfos(_ context.Context, transferID string) ([]*repository.FileInfo, error) {
	if r.err != nil {
		return nil, r.err
	}
	var infos []*repository.FileInfo
	for _, fid := range r.fileIDs[transferID] {
		infos = append(infos, &repository.FileInfo{ID: fid})
	}
	return infos, nil
}

func (r *stubTransferRepo) GetFileDownloadCounts(_ context.Context, transferID string) (map[string]int, error) {
	if r.err != nil {
		return nil, r.err
	}
	result := make(map[string]int)
	if r.fileDlCounts != nil {
		for k, v := range r.fileDlCounts[transferID] {
			result[k] = v
		}
	}
	return result, nil
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

func (r *stubTransferRepo) GetTransfersNeedingReminder(_ context.Context) ([]*domain.Transfer, error) {
	return nil, r.err
}

func (r *stubTransferRepo) MarkReminderSent(_ context.Context, _ string) error {
	return r.err
}

func (r *stubTransferRepo) UpdateRecipientEmail(_ context.Context, id, ownerID, recipientEmail string) error {
	if r.err != nil {
		return r.err
	}
	t, ok := r.byID[id]
	if !ok || t.OwnerID != ownerID {
		return apperrors.NotFound("transfer not found")
	}
	t.RecipientEmail = recipientEmail
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

// ── AttemptFileDownload ───────────────────────────────────────────────────────

func TestAttemptFileDownload_NoPassword_Success(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	tr := createTransferHelper(t, svc, "", 0, []string{"f1"})

	result, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"})
	if err != nil {
		t.Fatalf("AttemptFileDownload: %v", err)
	}
	if result.Transfer.Slug != tr.Slug {
		t.Errorf("Slug = %q", result.Transfer.Slug)
	}
}

func TestAttemptFileDownload_CorrectPassword_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secretpass", 0, []string{"f1"})

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1", Password: "secretpass"})
	if err != nil {
		t.Fatalf("AttemptFileDownload with correct password: %v", err)
	}
}

func TestAttemptFileDownload_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secretpass", 0, []string{"f1"})

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1", Password: "wrongpass"})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttemptFileDownload_MissingPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secretpass", 0, []string{"f1"})

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1", Password: ""})
	if err == nil {
		t.Fatal("expected error when password is required but not provided")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeUnauthorized {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttemptFileDownload_RevokedTransfer_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	tr := createTransferHelper(t, svc, "", 0, []string{"f1"})

	svc.Revoke(context.Background(), tr.ID, "owner-1") //nolint:errcheck

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"})
	if err == nil {
		t.Fatal("expected error for revoked transfer")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttemptFileDownload_ExpiredTransfer_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)

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

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"})
	if err == nil {
		t.Fatal("expected error for expired transfer")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttemptFileDownload_FileNotInTransfer_ReturnsNotFound(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	tr := createTransferHelper(t, svc, "", 0, []string{"f1", "f2"})

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f-doesnotexist"})
	if err == nil {
		t.Fatal("expected not-found for file not in transfer")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeNotFound {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttemptFileDownload_PerFileLimit_BlocksAfterLimit(t *testing.T) {
	// With max_downloads=1, the second attempt on the same file is blocked.
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)

	future := time.Now().Add(time.Hour)
	tr := &domain.Transfer{
		ID:           "tid",
		Slug:         "limit-slug",
		OwnerID:      "owner-1",
		MaxDownloads: 1,
		ExpiresAt:    &future,
	}
	repo.transfers[tr.Slug] = tr
	repo.byID[tr.ID] = tr
	repo.fileIDs[tr.ID] = []string{"f1", "f2"}

	// First attempt succeeds.
	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"})
	if err != nil {
		t.Fatalf("first download: %v", err)
	}

	// Second attempt on same file is forbidden.
	_, err = svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"})
	if err == nil {
		t.Fatal("expected forbidden for second download of same file")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeForbidden {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttemptFileDownload_PerFileLimit_OtherFilesStillAllowed(t *testing.T) {
	// Exhausting file f1 must NOT block file f2.
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)

	future := time.Now().Add(time.Hour)
	tr := &domain.Transfer{
		ID:           "tid2",
		Slug:         "limit-slug2",
		OwnerID:      "owner-1",
		MaxDownloads: 1,
		ExpiresAt:    &future,
	}
	repo.transfers[tr.Slug] = tr
	repo.byID[tr.ID] = tr
	repo.fileIDs[tr.ID] = []string{"f1", "f2"}

	// Exhaust f1.
	svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"}) //nolint:errcheck

	// f2 must still be accessible.
	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f2"})
	if err != nil {
		t.Errorf("f2 should still be allowed after f1 is exhausted, got: %v", err)
	}
}

func TestAttemptFileDownload_UnlimitedTransfer_AlwaysAllowed(t *testing.T) {
	// MaxDownloads=0 means unlimited; many downloads of the same file should succeed.
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)

	future := time.Now().Add(time.Hour)
	tr := &domain.Transfer{
		ID:           "tid3",
		Slug:         "unlimited-slug",
		OwnerID:      "owner-1",
		MaxDownloads: 0, // unlimited
		ExpiresAt:    &future,
	}
	repo.transfers[tr.Slug] = tr
	repo.byID[tr.ID] = tr
	repo.fileIDs[tr.ID] = []string{"f1"}

	for i := 0; i < 5; i++ {
		_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: tr.Slug, FileID: "f1"})
		if err != nil {
			t.Fatalf("attempt %d on unlimited transfer failed: %v", i+1, err)
		}
	}
}

func TestAttemptFileDownload_NonExistentSlug_ReturnsNotFound(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{Slug: "doesnotexist", FileID: "f1"})
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

// ── List ──────────────────────────────────────────────────────────────────────

func TestList_ReturnsOwnerTransfers(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	ctx := context.Background()

	// Create two transfers for owner-1 and one for owner-2.
	createTransferHelper(t, svc, "", 0, nil)
	createTransferHelper(t, svc, "", 0, nil)
	// Create a second-owner transfer by directly manipulating the stub.
	repo := newStubTransferRepo()
	svc2 := newTestTransferService(repo)
	tr3 := createTransferHelper(t, svc2, "", 0, nil)
	_ = tr3

	// List for owner-1 from the first svc.
	transfers, err := svc.List(ctx, "owner-1", 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(transfers) != 2 {
		t.Errorf("expected 2 transfers for owner-1, got %d", len(transfers))
	}
}

func TestList_EmptyForUnknownOwner(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	createTransferHelper(t, svc, "", 0, nil)

	transfers, err := svc.List(context.Background(), "nobody", 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(transfers) != 0 {
		t.Errorf("expected 0 transfers, got %d", len(transfers))
	}
}

func TestList_RepoError_Propagated(t *testing.T) {
	repo := newStubTransferRepo()
	repo.err = errors.New("db failure")
	svc := newTestTransferService(repo)

	_, err := svc.List(context.Background(), "owner-1", 100, 0)
	if err == nil {
		t.Fatal("expected error propagated from repo")
	}
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestGetByID_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	got, err := svc.GetByID(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != tr.ID {
		t.Errorf("expected ID %q, got %q", tr.ID, got.ID)
	}
}

func TestGetByID_NotFound_ReturnsError(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())

	_, err := svc.GetByID(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent transfer")
	}
}

// ── Get (owner-enforced) ──────────────────────────────────────────────────────

func TestGet_CorrectOwnerWithFileIDs(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	fileIDs := []string{"file-uuid-1111", "file-uuid-2222"}
	tr := createTransferHelper(t, svc, "", 0, fileIDs)

	got, gotFileIDs, err := svc.Get(context.Background(), tr.ID, "owner-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != tr.ID {
		t.Errorf("expected ID %q, got %q", tr.ID, got.ID)
	}
	if len(gotFileIDs) != len(fileIDs) {
		t.Errorf("expected %d fileIDs, got %d", len(fileIDs), len(gotFileIDs))
	}
}

func TestGet_WrongOwner_ReturnsForbidden2(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	_, _, err := svc.Get(context.Background(), tr.ID, "intruder")
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

// ── Validate ──────────────────────────────────────────────────────────────────

func TestValidate_NoPassword_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)

	result, err := svc.Validate(context.Background(), AccessParams{Slug: tr.Slug})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Transfer.ID != tr.ID {
		t.Errorf("expected transfer ID %q, got %q", tr.ID, result.Transfer.ID)
	}
}

func TestValidate_CorrectPassword_Success(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secret123", 0, nil)

	result, err := svc.Validate(context.Background(), AccessParams{Slug: tr.Slug, Password: "secret123"})
	if err != nil {
		t.Fatalf("Validate with correct password: %v", err)
	}
	if result.Transfer.ID != tr.ID {
		t.Errorf("expected transfer ID %q, got %q", tr.ID, result.Transfer.ID)
	}
}

func TestValidate_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secret123", 0, nil)

	_, err := svc.Validate(context.Background(), AccessParams{Slug: tr.Slug, Password: "wrongpass"})
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestValidate_MissingPassword_ReturnsUnauthorized(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "secret123", 0, nil)

	_, err := svc.Validate(context.Background(), AccessParams{Slug: tr.Slug})
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestValidate_RevokedTransfer_ReturnsForbidden(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())
	tr := createTransferHelper(t, svc, "", 0, nil)
	_ = svc.Revoke(context.Background(), tr.ID, "owner-1")

	_, err := svc.Validate(context.Background(), AccessParams{Slug: tr.Slug})
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for revoked transfer, got %v", err)
	}
}

func TestValidate_ExpiredTransfer_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	tr := createTransferHelper(t, svc, "", 1*time.Millisecond, nil)
	time.Sleep(5 * time.Millisecond)

	_, err := svc.Validate(context.Background(), AccessParams{Slug: tr.Slug})
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for expired transfer, got %v", err)
	}
}

func TestValidate_NonExistentSlug_ReturnsError(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())

	_, err := svc.Validate(context.Background(), AccessParams{Slug: "doesnotexist"})
	if err == nil {
		t.Fatal("expected error for nonexistent slug")
	}
}

// ── AtomicFileDownload additional scenarios ───────────────────────────────────

func TestAttemptFileDownload_TransferIsExhausted_ReturnsForbidden(t *testing.T) {
	repo := newStubTransferRepo()
	svc := newTestTransferService(repo)
	fileIDs := []string{"file-a"}
	tr := createTransferHelper(t, svc, "", 0, fileIDs)
	// Set MaxDownloads=1 on stored transfer.
	repo.transfers[tr.Slug].MaxDownloads = 1
	repo.byID[tr.ID].MaxDownloads = 1
	// Mark the file as already fully downloaded.
	repo.fileDlCounts = map[string]map[string]int{
		tr.ID: {"file-a": 1},
	}

	_, err := svc.AttemptFileDownload(context.Background(), AttemptFileDownloadParams{
		Slug:   tr.Slug,
		FileID: "file-a",
	})
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden when transfer exhausted, got %v", err)
	}
}

// ── Create edge cases ──────────────────────────────────────────────────────────

func TestCreate_MaxDownloadsZero_IsUnlimited(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())

	result, err := svc.Create(context.Background(), CreateParams{
		OwnerID:      "owner-1",
		Name:         "Unlimited DL",
		FileIDs:      []string{"file-001"},
		ExpiresIn:    24 * time.Hour,
		MaxDownloads: 0,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Transfer.MaxDownloads != 0 {
		t.Errorf("expected MaxDownloads=0, got %d", result.Transfer.MaxDownloads)
	}
}

func TestCreate_WithPassword_HashStored(t *testing.T) {
	svc := newTestTransferService(newStubTransferRepo())

	result, err := svc.Create(context.Background(), CreateParams{
		OwnerID:   "owner-1",
		Name:      "Protected",
		FileIDs:   []string{"file-001"},
		ExpiresIn: 24 * time.Hour,
		Password:  "s3cret!",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Transfer.PasswordHash == "" {
		t.Error("expected non-empty PasswordHash for password-protected transfer")
	}
	// The raw password must not be stored directly.
	if result.Transfer.PasswordHash == "s3cret!" {
		t.Error("PasswordHash should be a hash, not the raw password")
	}
}
