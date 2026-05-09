// Package consumer subscribes to the NATS AUDIT.* stream and persists events to DB.
package consumer

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/audit/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
)

// Consumer subscribes to AUDIT.* and writes events to the audit_logs table.
type Consumer struct {
	js   *jetstream.Client
	repo *repository.Repository
	log  *zap.Logger
}

func New(js *jetstream.Client, repo *repository.Repository, log *zap.Logger) *Consumer {
	return &Consumer{js: js, repo: repo, log: log}
}

// Start blocks until ctx is done.
func (c *Consumer) Start(ctx context.Context) error {
	c.log.Info("audit consumer starting")
	return c.js.Subscribe(ctx, "AUDIT", "audit-service", "AUDIT.*",
		func(subject string, data []byte) error {
			return c.handle(subject, data)
		},
	)
}

func (c *Consumer) handle(subject string, data []byte) error {
	// Extract source from subject: "AUDIT.auth" → "auth"
	parts := strings.SplitN(subject, ".", 2)
	source := ""
	if len(parts) == 2 {
		source = parts[1]
	}

	// Parse common fields from payload (best effort)
	var partial struct {
		Action   string  `json:"action"`
		UserID   *string `json:"user_id"`
		ClientIP *string `json:"client_ip"`
		Success  *bool   `json:"success"`
	}
	_ = json.Unmarshal(data, &partial)

	success := true
	if partial.Success != nil {
		success = *partial.Success
	}
	action := partial.Action
	if action == "" {
		action = subject
	}

	entry := repository.AuditLog{
		Source:   source,
		Action:   action,
		UserID:   partial.UserID,
		ClientIP: partial.ClientIP,
		Subject:  subject,
		Payload:  json.RawMessage(data),
		Success:  success,
		Severity: deriveSeverity(action, success),
	}

	if err := c.repo.Insert(context.Background(), entry); err != nil {
		c.log.Error("failed to persist audit event",
			zap.String("subject", subject),
			zap.Error(err),
		)
		return err
	}

	c.log.Debug("audit event persisted",
		zap.String("source", source),
		zap.String("action", action),
	)
	return nil
}

// deriveSeverity maps an action + success flag to a severity level.
// Levels: "info" | "warning" | "error"
func deriveSeverity(action string, success bool) string {
	if !success {
		if strings.Contains(action, "_failed") || strings.Contains(action, "_error") {
			return "error"
		}
		return "warning"
	}
	// Destructive/sensitive actions are "warning" even when they succeed.
	for _, kw := range []string{"_deleted", "_purged", "_purge", "_revoked", "_terminated", "_banned", "_locked", "_mfa_reset", "_password_reset", "_password_set"} {
		if strings.Contains(action, kw) {
			return "warning"
		}
	}
	return "info"
}
