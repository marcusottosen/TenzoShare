// Package email provides SMTP email delivery for the notification service.
// It uses Go's standard net/smtp with optional STARTTLS.
package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"text/template"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// Sender delivers transactional emails via SMTP.
type Sender struct {
	cfg config.SMTPConfig
	log *zap.Logger
}

// New creates a Sender. Call Send to deliver emails.
func New(cfg config.SMTPConfig, log *zap.Logger) *Sender {
	return &Sender{cfg: cfg, log: log}
}

// Message is the data passed to Send.
type Message struct {
	To      []string
	Subject string
	Body    string // plain-text body (HTML body optional in future)
}

// Send delivers msg via SMTP. Non-fatal: errors are logged.
func (s *Sender) Send(msg Message) error {
	addr := s.cfg.Host + ":" + s.cfg.Port

	body := buildRFC2822(s.cfg.From, msg.To, msg.Subject, msg.Body)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	if s.cfg.UseTLS {
		tlsCfg := &tls.Config{ServerName: s.cfg.Host} //nolint:gosec
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, s.cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := client.Mail(s.cfg.From); err != nil {
			return fmt.Errorf("smtp MAIL FROM: %w", err)
		}
		for _, to := range msg.To {
			if err := client.Rcpt(to); err != nil {
				return fmt.Errorf("smtp RCPT TO %s: %w", to, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp DATA: %w", err)
		}
		if _, err = w.Write(body); err != nil {
			return fmt.Errorf("smtp write body: %w", err)
		}
		return w.Close()
	}

	// plain SMTP (dev / MailHog)
	return smtp.SendMail(addr, auth, s.cfg.From, msg.To, body)
}

func buildRFC2822(from string, to []string, subject, body string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", joinAddrs(to))
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n%s", body)
	return buf.Bytes()
}

func joinAddrs(addrs []string) string {
	buf := &bytes.Buffer{}
	for i, a := range addrs {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(a)
	}
	return buf.String()
}

// ── Templates ─────────────────────────────────────────────────────────────────

var transferReceivedTpl = template.Must(template.New("transfer_received").Parse(`Hello,

{{ .SenderName }} has shared files with you via TenzoShare.

Transfer: {{ .Title }}
{{ if .Message }}
Message: {{ .Message }}
{{ end }}
Download link: {{ .DownloadURL }}
{{ if .ExpiresAt }}Expires: {{ .ExpiresAt }}{{ end }}

This link is {{ if .HasPassword }}password-protected{{ else }}public{{ end }}.

— The TenzoShare team
`))

var passwordResetTpl = template.Must(template.New("password_reset").Parse(`Hello,

We received a request to reset your TenzoShare password.

Click the link below to reset your password (valid for 1 hour):
{{ .ResetURL }}

If you didn't request a password reset, you can safely ignore this email.

— The TenzoShare team
`))

var downloadNotificationTpl = template.Must(template.New("download_notification").Parse(`Hello,

Your transfer "{{ .Title }}" was downloaded by {{ .RecipientEmail }}.

Downloaded at: {{ .DownloadedAt }}
Download link: {{ .DownloadURL }}

— The TenzoShare team
`))

// RenderTransferReceived returns a formatted email body for a new transfer notification.
func RenderTransferReceived(data TransferReceivedData) (string, error) {
	var buf bytes.Buffer
	if err := transferReceivedTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderPasswordReset returns a formatted email body for a password reset.
func RenderPasswordReset(data PasswordResetData) (string, error) {
	var buf bytes.Buffer
	if err := passwordResetTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderDownloadNotification returns a formatted email body for a download notification.
func RenderDownloadNotification(data DownloadNotificationData) (string, error) {
	var buf bytes.Buffer
	if err := downloadNotificationTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type TransferReceivedData struct {
	SenderName  string
	Title       string
	Message     string
	DownloadURL string
	ExpiresAt   string
	HasPassword bool
}

type PasswordResetData struct {
	ResetURL string
}

type DownloadNotificationData struct {
	Title          string
	RecipientEmail string
	DownloadedAt   string
	DownloadURL    string
}
