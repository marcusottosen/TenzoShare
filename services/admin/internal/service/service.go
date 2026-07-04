package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// Service is the base service struct that holds shared dependencies.
type Service struct {
	repo *repository.Repository
	cfg  *config.Config
	log  *zap.Logger
}

// New creates a new service instance.
func New(repo *repository.Repository, cfg *config.Config, log *zap.Logger) *Service {
	return &Service{
		repo: repo,
		cfg:  cfg,
		log:  log,
	}
}

// Repository returns the underlying repository.
func (s *Service) Repository() *repository.Repository {
	return s.repo
}

// Config returns the configuration.
func (s *Service) Config() *config.Config {
	return s.cfg
}

// Logger returns the logger.
func (s *Service) Logger() *zap.Logger {
	return s.log
}

// PublishAuditEvent publishes an audit event to NATS (best-effort, never blocks).
func (s *Service) PublishAuditEvent(ctx context.Context, action, actorID, actorEmail, subjectID, ipAddr string, metadata map[string]any) {
	js := s.repo.JetStream()
	if js == nil {
		return
	}

	event := map[string]any{
		"action":      action,
		"actor_id":    actorID,
		"actor_email": actorEmail,
		"subject_id":  subjectID,
		"ip_address":  ipAddr,
		"metadata":    metadata,
	}

	data, err := json.Marshal(event)
	if err != nil {
		s.log.Error("failed to marshal audit event", zap.Error(err))
		return
	}

	subject := "AUDIT.admin"
	go func() {
		if err := js.Publish(ctx, subject, data); err != nil {
			s.log.Error("failed to publish audit event", zap.String("subject", subject), zap.Error(err))
		}
	}()
}

// SMTPEncKey derives a stable 32-byte AES key from the application pepper.
func (s *Service) SMTPEncKey() []byte {
	pepper := os.Getenv("PEPPER")
	if pepper == "" {
		pepper = "default-dev-pepper-change-in-production"
	}
	hash := sha256.Sum256([]byte(pepper + ":smtp_settings"))
	return hash[:]
}
