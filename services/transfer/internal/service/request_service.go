package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
)

const requestSlugBytes = 32 // 256-bit slug — astronomically hard to guess

// RequestService handles business logic for the file-request (upload dropbox) workflow.
type RequestService struct {
	repo       *repository.RequestRepository
	cfg        *config.Config
	js         *jetstream.Client
	log        *zap.Logger
	storageURL string
	httpClient *http.Client

	// Simple in-memory rate limiter for public upload endpoint.
	// Replace with Redis-backed solution if deploying multiple transfer instances.
	rateMu     sync.Mutex
	rateWindow map[string][]time.Time
}

// NewRequestService constructs a RequestService.
func NewRequestService(
	repo *repository.RequestRepository,
	cfg *config.Config,
	js *jetstream.Client,
	log *zap.Logger,
	storageURL string,
) *RequestService {
	return &RequestService{
		repo:       repo,
		cfg:        cfg,
		js:         js,
		log:        log,
		storageURL: storageURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		rateWindow: make(map[string][]time.Time),
	}
}

// --- Create / List / Get / Deactivate (authenticated) ---

// CreateRequestParams carries inputs for creating a file request.
type CreateRequestParams struct {
	OwnerID        string
	Name           string
	Description    string
	AllowedTypes   string // comma-separated MIME prefixes; empty = all
	MaxSizeMB      int    // 0 = unlimited
	MaxFiles       int    // 0 = unlimited
	ExpiresInHrs   int    // must be >= 1
	NotifyEmails   string // comma-separated; empty = no invite emails sent
	NotifyOnUpload bool   // true = email the owner when a guest submits a file
}

// Create creates a new file request and returns the persisted record.
// If NotifyEmails is set, an invite notification is published via NATS for each address.
func (s *RequestService) Create(ctx context.Context, p CreateRequestParams) (*domain.FileRequest, error) {
	slug, err := crypto.RandomToken(requestSlugBytes)
	if err != nil {
		return nil, apperrors.Internal("generate slug", err)
	}
	req := &domain.FileRequest{
		OwnerID:        p.OwnerID,
		Slug:           slug,
		Name:           p.Name,
		Description:    p.Description,
		AllowedTypes:   p.AllowedTypes,
		MaxSizeMB:      p.MaxSizeMB,
		MaxFiles:       p.MaxFiles,
		NotifyEmails:   p.NotifyEmails,
		NotifyOnUpload: p.NotifyOnUpload,
		ExpiresAt:      time.Now().Add(time.Duration(p.ExpiresInHrs) * time.Hour),
	}
	result, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	// Send upload-link invitations (best-effort — never fail the creation on NATS error).
	if s.js != nil && p.NotifyEmails != "" {
		go s.publishRequestInvite(ctx, result)
	}

	return result, nil
}

// publishRequestInvite sends a request_invite notification for each recipient.
func (s *RequestService) publishRequestInvite(ctx context.Context, req *domain.FileRequest) {
	recipients := splitEmails(req.NotifyEmails)
	if len(recipients) == 0 {
		return
	}
	uploadURL := s.cfg.App.BaseURL + "/r/" + req.Slug
	data, _ := json.Marshal(map[string]any{
		"RequestName": req.Name,
		"UploadURL":   uploadURL,
	})
	ev := map[string]any{
		"type": "request_invite",
		"to":   recipients,
		"data": json.RawMessage(data),
	}
	if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
		s.log.Warn("failed to publish request_invite notification", zap.Error(err))
	}
}

// splitEmails splits a comma-separated email string into a trimmed slice.
func splitEmails(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// GetPublic returns the public-facing view of a file request (by slug).
func (s *RequestService) GetPublic(ctx context.Context, slug string) (*domain.FileRequest, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// Get returns a file request with its submissions, enforcing owner access.
func (s *RequestService) Get(ctx context.Context, id, ownerID string) (*domain.FileRequest, []*domain.RequestSubmission, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if req.OwnerID != ownerID {
		return nil, nil, apperrors.Forbidden("access denied")
	}
	subs, err := s.repo.ListSubmissions(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return req, subs, nil
}

// List returns all file requests owned by ownerID.
func (s *RequestService) List(ctx context.Context, ownerID string, limit, offset int) ([]*domain.FileRequest, error) {
	return s.repo.ListByOwner(ctx, ownerID, limit, offset)
}

// Deactivate closes a file request so guests can no longer upload to it.
func (s *RequestService) Deactivate(ctx context.Context, id, ownerID string) error {
	return s.repo.Deactivate(ctx, id, ownerID)
}

// UpdateNotifyEmails replaces the recipient (invite) email list for a file request.
// emails may be empty to remove all recipients.
func (s *RequestService) UpdateNotifyEmails(ctx context.Context, id, ownerID string, emails []string) (*domain.FileRequest, error) {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.OwnerID != ownerID {
		return nil, apperrors.Forbidden("access denied")
	}

	// Deduplicate and normalise.
	seen := map[string]struct{}{}
	var unique []string
	for _, e := range emails {
		if e = strings.TrimSpace(e); e != "" {
			if _, dup := seen[e]; !dup {
				seen[e] = struct{}{}
				unique = append(unique, e)
			}
		}
	}
	notifyEmails := strings.Join(unique, ",")

	if err := s.repo.UpdateNotifyEmails(ctx, id, ownerID, notifyEmails); err != nil {
		return nil, err
	}
	req.NotifyEmails = notifyEmails
	return req, nil
}

// ResendInvite re-publishes request_invite emails to all current recipients.
// Only the owner may call this; the request must be active and non-expired.
func (s *RequestService) ResendInvite(ctx context.Context, id, ownerID string) error {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if req.OwnerID != ownerID {
		return apperrors.Forbidden("access denied")
	}
	if req.IsExpired() {
		return apperrors.BadRequest("cannot resend: request is closed or expired")
	}
	if req.NotifyEmails == "" {
		return apperrors.BadRequest("no recipients to notify")
	}
	go s.publishRequestInvite(ctx, req)
	return nil
}

// --- Submit (public upload endpoint) ---

// SubmitParams carries the inputs for a guest file submission.
type SubmitParams struct {
	Slug          string
	File          multipart.File
	Header        *multipart.FileHeader
	SubmitterName string
	Message       string
	IP            string
	ServiceToken  string // short-lived RS256 JWT issued by the handler
}

// Submit validates the request, uploads the file to the Storage service, and
// records the submission. Rate-limited to 10 uploads / hour / IP.
func (s *RequestService) Submit(ctx context.Context, p SubmitParams) (*domain.RequestSubmission, error) {
	// Rate limiting: 10 uploads per hour per IP address.
	if !s.allowIP(p.IP, 10, time.Hour) {
		return nil, apperrors.RateLimit("upload rate limit exceeded — please try again later")
	}

	req, err := s.repo.GetBySlug(ctx, p.Slug)
	if err != nil {
		return nil, err
	}
	if req.IsExpired() {
		return nil, apperrors.BadRequest("this file request has expired or been closed")
	}
	if req.MaxSizeMB > 0 && p.Header.Size > int64(req.MaxSizeMB)*1024*1024 {
		return nil, apperrors.BadRequest(fmt.Sprintf("file exceeds the maximum size of %d MB", req.MaxSizeMB))
	}

	// Forward the file to the Storage service.
	fileID, err := s.uploadToStorage(ctx, p.File, p.Header, p.ServiceToken)
	if err != nil {
		return nil, err
	}

	sub := &domain.RequestSubmission{
		RequestID:     req.ID,
		FileID:        fileID,
		Filename:      p.Header.Filename,
		SizeBytes:     p.Header.Size,
		SubmitterName: p.SubmitterName,
		Message:       p.Message,
		SubmitterIP:   p.IP,
	}
	result, err := s.repo.AddSubmission(ctx, sub)
	if err != nil {
		return nil, err
	}

	// Notify the request owner via NATS (best-effort — never fail the upload on NATS error).
	if s.js != nil && req.NotifyOnUpload && req.OwnerEmail != "" {
		data, _ := json.Marshal(map[string]any{
			"RequestName":   req.Name,
			"Filename":      p.Header.Filename,
			"SubmitterName": p.SubmitterName,
			"ReviewURL":     s.cfg.App.BaseURL + "/requests",
		})
		ev := map[string]any{
			"type": "request_submission",
			"to":   []string{req.OwnerEmail},
			"data": json.RawMessage(data),
		}
		go func() {
			if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
				s.log.Warn("failed to publish request_submission notification", zap.Error(err))
			}
		}()
	}

	return result, nil
}

// uploadToStorage posts a multipart file to the Storage service and returns
// the storage file ID.
func (s *RequestService) uploadToStorage(ctx context.Context, file multipart.File, header *multipart.FileHeader, token string) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	fw, err := mw.CreateFormFile("file", header.Filename)
	if err != nil {
		return "", apperrors.Internal("create form file", err)
	}
	if _, err := io.Copy(fw, file); err != nil {
		return "", apperrors.Internal("copy file data", err)
	}
	mw.Close() //nolint:errcheck

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.storageURL+"/api/v1/files", &body)
	if err != nil {
		return "", apperrors.Internal("build storage request", err)
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", apperrors.Internal("upload to storage service", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		s.log.Warn("storage upload failed", zap.Int("status", resp.StatusCode), zap.String("body", string(raw)))
		return "", apperrors.Internal("storage upload failed", nil)
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", apperrors.Internal("decode storage response", err)
	}
	return res.ID, nil
}

// allowIP checks whether ip has not exceeded limit uploads within window.
// Uses a simple sliding-window counter backed by a goroutine-safe in-memory map.
func (s *RequestService) allowIP(ip string, limit int, window time.Duration) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	times := s.rateWindow[ip]
	fresh := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}

	if len(fresh) >= limit {
		s.rateWindow[ip] = fresh
		return false
	}
	s.rateWindow[ip] = append(fresh, now)
	return true
}
