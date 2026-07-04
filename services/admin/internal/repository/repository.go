package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
)

// Repository is the base repository struct that holds shared dependencies.
type Repository struct {
	db *pgxpool.Pool
	js *jetstream.Client
}

// New creates a new repository instance.
func New(db *pgxpool.Pool, js *jetstream.Client) *Repository {
	return &Repository{db: db, js: js}
}

// DB returns the underlying database pool.
func (r *Repository) DB() *pgxpool.Pool {
	return r.db
}

// JetStream returns the NATS JetStream client (may be nil).
func (r *Repository) JetStream() *jetstream.Client {
	return r.js
}

// Exec wraps db.Exec with context.
func (r *Repository) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.db.Exec(ctx, sql, args...)
	return err
}
