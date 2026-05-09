package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// RequestRepository handles persistence for file_requests and request_submissions.
type RequestRepository struct {
	db *pgxpool.Pool
}

// NewRequestRepository creates a new RequestRepository.
func NewRequestRepository(db *pgxpool.Pool) *RequestRepository {
	return &RequestRepository{db: db}
}

// Create inserts a new file request row and returns the persisted record.
func (r *RequestRepository) Create(ctx context.Context, req *domain.FileRequest) (*domain.FileRequest, error) {
	var out domain.FileRequest
	err := r.db.QueryRow(ctx, `
		INSERT INTO transfer.file_requests
			(owner_id, slug, name, description, allowed_types, max_size_mb, max_files, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, owner_id, slug, name, description, allowed_types, max_size_mb, max_files, expires_at, is_active, created_at
	`, req.OwnerID, req.Slug, req.Name, req.Description,
		req.AllowedTypes, req.MaxSizeMB, req.MaxFiles, req.ExpiresAt,
	).Scan(
		&out.ID, &out.OwnerID, &out.Slug, &out.Name, &out.Description,
		&out.AllowedTypes, &out.MaxSizeMB, &out.MaxFiles, &out.ExpiresAt, &out.IsActive, &out.CreatedAt,
	)
	if err != nil {
		return nil, apperrors.Internal("create file request", err)
	}
	return &out, nil
}

// GetBySlug looks up a file request by its public slug.
func (r *RequestRepository) GetBySlug(ctx context.Context, slug string) (*domain.FileRequest, error) {
	var out domain.FileRequest
	err := r.db.QueryRow(ctx, `
		SELECT fr.id, fr.owner_id, COALESCE(u.email, '') AS owner_email,
		       fr.slug, fr.name, fr.description, fr.allowed_types,
		       fr.max_size_mb, fr.max_files, fr.expires_at, fr.is_active, fr.created_at
		FROM transfer.file_requests fr
		LEFT JOIN auth.users u ON u.id = fr.owner_id
		WHERE fr.slug = $1
	`, slug).Scan(
		&out.ID, &out.OwnerID, &out.OwnerEmail, &out.Slug, &out.Name, &out.Description,
		&out.AllowedTypes, &out.MaxSizeMB, &out.MaxFiles, &out.ExpiresAt, &out.IsActive, &out.CreatedAt,
	)
	if err != nil {
		return nil, apperrors.NotFound("file request not found")
	}
	return &out, nil
}

// GetByID looks up a file request by its primary key.
func (r *RequestRepository) GetByID(ctx context.Context, id string) (*domain.FileRequest, error) {
	var out domain.FileRequest
	err := r.db.QueryRow(ctx, `
		SELECT fr.id, fr.owner_id, COALESCE(u.email, '') AS owner_email,
		       fr.slug, fr.name, fr.description, fr.allowed_types,
		       fr.max_size_mb, fr.max_files, fr.expires_at, fr.is_active, fr.created_at
		FROM transfer.file_requests fr
		LEFT JOIN auth.users u ON u.id = fr.owner_id
		WHERE fr.id = $1
	`, id).Scan(
		&out.ID, &out.OwnerID, &out.OwnerEmail, &out.Slug, &out.Name, &out.Description,
		&out.AllowedTypes, &out.MaxSizeMB, &out.MaxFiles, &out.ExpiresAt, &out.IsActive, &out.CreatedAt,
	)
	if err != nil {
		return nil, apperrors.NotFound("file request not found")
	}
	return &out, nil
}

// ListByOwner returns all file requests for a given owner, newest first, with submission counts.
func (r *RequestRepository) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*domain.FileRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT fr.id, fr.owner_id, fr.slug, fr.name, fr.description, fr.allowed_types,
		       fr.max_size_mb, fr.max_files, fr.expires_at, fr.is_active, fr.created_at,
		       COUNT(rs.id) AS submission_count
		FROM transfer.file_requests fr
		LEFT JOIN transfer.request_submissions rs ON rs.request_id = fr.id
		WHERE fr.owner_id = $1
		GROUP BY fr.id
		ORDER BY fr.created_at DESC LIMIT $2 OFFSET $3
	`, ownerID, limit, offset)
	if err != nil {
		return nil, apperrors.Internal("list file requests", err)
	}
	defer rows.Close()

	var list []*domain.FileRequest
	for rows.Next() {
		var out domain.FileRequest
		if err := rows.Scan(
			&out.ID, &out.OwnerID, &out.Slug, &out.Name, &out.Description,
			&out.AllowedTypes, &out.MaxSizeMB, &out.MaxFiles, &out.ExpiresAt, &out.IsActive, &out.CreatedAt,
			&out.SubmissionCount,
		); err != nil {
			return nil, apperrors.Internal("scan file request", err)
		}
		list = append(list, &out)
	}
	return list, nil
}

// Deactivate sets is_active=false on a request owned by ownerID.
func (r *RequestRepository) Deactivate(ctx context.Context, id, ownerID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE transfer.file_requests SET is_active = FALSE
		WHERE id = $1 AND owner_id = $2
	`, id, ownerID)
	if err != nil {
		return apperrors.Internal("deactivate file request", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("file request not found")
	}
	return nil
}

// AddSubmission records a file uploaded by a guest to a file request.
func (r *RequestRepository) AddSubmission(ctx context.Context, s *domain.RequestSubmission) (*domain.RequestSubmission, error) {
	var out domain.RequestSubmission
	err := r.db.QueryRow(ctx, `
		INSERT INTO transfer.request_submissions
			(request_id, file_id, filename, size_bytes, submitter_name, message, submitter_ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, request_id, file_id, filename, size_bytes, submitter_name, message, submitter_ip, submitted_at
	`, s.RequestID, s.FileID, s.Filename, s.SizeBytes,
		s.SubmitterName, s.Message, s.SubmitterIP,
	).Scan(
		&out.ID, &out.RequestID, &out.FileID, &out.Filename, &out.SizeBytes,
		&out.SubmitterName, &out.Message, &out.SubmitterIP, &out.SubmittedAt,
	)
	if err != nil {
		return nil, apperrors.Internal("add submission", err)
	}
	return &out, nil
}

// ListSubmissions returns all submissions for a request, newest first.
func (r *RequestRepository) ListSubmissions(ctx context.Context, requestID string) ([]*domain.RequestSubmission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, request_id, file_id, filename, size_bytes, submitter_name, message, submitter_ip, submitted_at
		FROM transfer.request_submissions WHERE request_id = $1
		ORDER BY submitted_at DESC
	`, requestID)
	if err != nil {
		return nil, apperrors.Internal("list submissions", err)
	}
	defer rows.Close()

	var list []*domain.RequestSubmission
	for rows.Next() {
		var out domain.RequestSubmission
		if err := rows.Scan(
			&out.ID, &out.RequestID, &out.FileID, &out.Filename, &out.SizeBytes,
			&out.SubmitterName, &out.Message, &out.SubmitterIP, &out.SubmittedAt,
		); err != nil {
			return nil, apperrors.Internal("scan submission", err)
		}
		list = append(list, &out)
	}
	return list, nil
}
