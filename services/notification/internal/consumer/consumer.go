// Package consumer subscribes to NATS JetStream subjects and dispatches
// email delivery based on the event type.
package consumer

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/notification/internal/email"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
)

// EmailEvent is the canonical payload published to NOTIFICATIONS.email by any
// TenzoShare service. The Type field selects which email template to render.
type EmailEvent struct {
	Type string          `json:"type"` // "transfer_received" | "password_reset" | "download_notification"
	To   []string        `json:"to"`
	Data json.RawMessage `json:"data"`
}

// Consumer subscribes to NOTIFICATIONS.email and delivers emails.
type Consumer struct {
	js     *jetstream.Client
	sender *email.Sender
	log    *zap.Logger
}

func New(js *jetstream.Client, sender *email.Sender, log *zap.Logger) *Consumer {
	return &Consumer{js: js, sender: sender, log: log}
}

// Start blocks until ctx is done. It subscribes to NOTIFICATIONS.email using
// a durable consumer named "notification-service".
func (c *Consumer) Start(ctx context.Context) error {
	c.log.Info("notification consumer starting")
	return c.js.Subscribe(ctx, "NOTIFICATIONS", "notification-service", "NOTIFICATIONS.email",
		func(subject string, data []byte) error {
			return c.handle(subject, data)
		},
	)
}

func (c *Consumer) handle(subject string, data []byte) error {
	var ev EmailEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		c.log.Error("failed to unmarshal email event", zap.Error(err), zap.String("subject", subject))
		// do not NAK — bad messages should not be redelivered infinitely
		return nil
	}

	if len(ev.To) == 0 {
		c.log.Warn("email event has no recipients", zap.String("type", ev.Type))
		return nil
	}

	var (
		subject2 string
		body     string
		err      error
	)

	switch ev.Type {
	case "transfer_received":
		var d email.TransferReceivedData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "You've received files via TenzoShare: " + d.Title
		body, err = email.RenderTransferReceived(d)

	case "password_reset":
		var d email.PasswordResetData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "Reset your TenzoShare password"
		body, err = email.RenderPasswordReset(d)

	case "download_notification":
		var d email.DownloadNotificationData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "Your transfer was downloaded"
		body, err = email.RenderDownloadNotification(d)

	default:
		c.log.Warn("unknown email event type", zap.String("type", ev.Type))
		return nil
	}

	if err != nil {
		c.log.Error("failed to render email template",
			zap.String("type", ev.Type), zap.Error(err))
		return nil
	}

	sendErr := c.sender.Send(email.Message{
		To:      ev.To,
		Subject: subject2,
		Body:    body,
	})
	if sendErr != nil {
		c.log.Error("failed to send email",
			zap.String("type", ev.Type),
			zap.Strings("to", ev.To),
			zap.Error(sendErr))
		// return error so the message gets NAKed and retried
		return sendErr
	}

	c.log.Info("email delivered",
		zap.String("type", ev.Type),
		zap.Strings("to", ev.To),
		zap.Time("at", time.Now()),
	)
	return nil
}
