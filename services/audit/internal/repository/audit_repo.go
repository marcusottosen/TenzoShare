// Package repository provides database access for the audit service.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// AuditLog is a single audit event row.
type AuditLog struct {
	ID        string
	Source    string
	Action    string
	UserID    *string
	ClientIP  *string
	Subject   string
	Payload   json.RawMessage
	Success   bool
	CreatedAt time.Time
}

// ListFilter holds optional query filters for ListEvents.
type ListFilter struct {
	UserID    string
	Source    string
	Action    string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// Repository handles audit_logs persistence.
type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Insert writes a single audit event to the DB.
func (r *Repository) Insert(ctx context.Context, log AuditLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit.audit_logs
		    (source, action, user_id, client_ip, subject, payload, success)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, log.Source, log.Action, log.UserID, log.ClientIP, log.Subject, log.Payload, log.Success)
	if err != nil {
		return apperrors.Internal("insert audit log", err)
	}
	return nil
}

// List returns paginated audit events, optionally filtered.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]AuditLog, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	// Build query dynamically to support optional filters
	args := []any{}
	where := ""
	argIdx := 1

	addCondition := func(cond string, val any) {
		if where == "" {
			where = "WHERE "
		} else {
			where += " AND "
		}
		where += cond
		args = append(args, val)
		argIdx++
	}

	if f.UserID != "" {
		addCondition("user_id = $"+itoa(argIdx), f.UserID)
	}
	if f.Source != "" {
		addCondition("source = $"+itoa(argIdx), f.Source)
	}
	if f.Action != "" {
		addCondition("action LIKE $"+itoa(argIdx), f.Action+"%")
	}
	if f.StartTime != nil {
		addCondition("created_at >= $"+itoa(argIdx), *f.StartTime)
	}
	if f.EndTime != nil {
		addCondition("created_at < $"+itoa(argIdx), *f.EndTime)
	}

	// Count query
	var total int
	countSQL := "SELECT count(*) FROM audit.audit_logs " + where
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, apperrors.Internal("count audit logs", err)
	}

	// Data query
	dataSQL := "SELECT id, source, action, user_id, client_ip, subject, payload, success, created_at " +
		"FROM audit.audit_logs " + where +
		" ORDER BY created_at DESC" +
		" LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, total, nil
		}
		return nil, 0, apperrors.Internal("list audit logs", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(
			&l.ID, &l.Source, &l.Action, &l.UserID, &l.ClientIP,
			&l.Subject, &l.Payload, &l.Success, &l.CreatedAt,
		); err != nil {
			return nil, 0, apperrors.Internal("scan audit log row", err)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
