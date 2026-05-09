// Package email provides SMTP email delivery for the notification service.
// It uses Go's standard net/smtp with optional STARTTLS.
package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// Sender delivers transactional emails via SMTP.
type Sender struct {
	mu       sync.RWMutex // protects cfg for live config updates
	cfg      config.SMTPConfig
	log      *zap.Logger
	branding *BrandingFetcher
}

// New creates a Sender. branding may be nil (falls back to defaults).
func New(cfg config.SMTPConfig, log *zap.Logger, branding *BrandingFetcher) *Sender {
	return &Sender{cfg: cfg, log: log, branding: branding}
}

// UpdateConfig atomically replaces the SMTP configuration. Safe to call from
// any goroutine; the next Send will pick up the new settings.
func (s *Sender) UpdateConfig(cfg config.SMTPConfig) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	s.log.Info("smtp configuration reloaded",
		zap.String("host", cfg.Host),
		zap.String("port", cfg.Port),
		zap.String("from", cfg.From),
	)
}

// Message is the data passed to Send.
type Message struct {
	To       []string
	FromName string // optional display name for the From header, e.g. "Acme Transfers"
	ReplyTo  string // optional Reply-To address
	Subject  string
	Body     string // plain-text body (always set)
	HTMLBody string // HTML body; if set, message is sent as multipart/alternative
}

// Branding returns the current BrandingData for use by callers that need
// email-specific settings (sender name, subject prefix, etc.).
func (s *Sender) Branding() BrandingData {
	if s.branding != nil {
		return s.branding.Get()
	}
	return defaultBranding
}

// RenderHTML renders the named email type as HTML with branding injected.
// unsubscribeURL is embedded in the footer; pass "" to omit the unsubscribe link.
// Returns an empty string on error so the plain-text fallback is always used.
func (s *Sender) RenderHTML(emailType string, data any, unsubscribeURL string) string {
	var b BrandingData
	if s.branding != nil {
		b = s.branding.Get()
	} else {
		b = defaultBranding
	}
	html, err := renderHTML(emailType, b, data, unsubscribeURL)
	if err != nil {
		s.log.Warn("html email render failed",
			zap.String("type", emailType), zap.Error(err))
		return ""
	}
	return html
}

// Send delivers msg via SMTP. If msg.HTMLBody is non-empty the message is sent
// as multipart/alternative (text/plain + text/html); otherwise plain text only.
func (s *Sender) Send(msg Message) error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	addr := cfg.Host + ":" + cfg.Port

	body := buildMIME(cfg.From, msg.FromName, msg.ReplyTo, msg.To, msg.Subject, msg.Body, msg.HTMLBody)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	if cfg.UseTLS {
		tlsCfg := &tls.Config{ServerName: cfg.Host} //nolint:gosec
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := client.Mail(cfg.From); err != nil {
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
	return smtp.SendMail(addr, auth, cfg.From, msg.To, body)
}

// buildMIME constructs the raw SMTP message bytes. When htmlBody is non-empty
// the message uses multipart/alternative so clients can choose the best part.
func buildMIME(from, fromName, replyTo string, to []string, subject, plainBody, htmlBody string) []byte {
	var buf bytes.Buffer
	if fromName != "" {
		// RFC 5322 display-name format: "Display Name" <addr@example.com>
		quoted := fmt.Sprintf("%q", fromName) // Go's %q adds surrounding double-quotes and escapes
		fmt.Fprintf(&buf, "From: %s <%s>\r\n", quoted, from)
	} else {
		fmt.Fprintf(&buf, "From: %s\r\n", from)
	}
	if replyTo != "" {
		fmt.Fprintf(&buf, "Reply-To: %s\r\n", replyTo)
	}
	fmt.Fprintf(&buf, "To: %s\r\n", joinAddrs(to))
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	if htmlBody == "" {
		// Plain-text only
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "\r\n%s", plainBody)
		return buf.Bytes()
	}

	// multipart/alternative — text/plain first (lowest fidelity), text/html last (preferred)
	boundary := fmt.Sprintf("--tenzoshare_%d", time.Now().UnixNano())
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
	fmt.Fprintf(&buf, "\r\n")

	// plain part
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n%s\r\n", plainBody)

	// HTML part
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n%s\r\n", htmlBody)

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)
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

var requestSubmissionTpl = template.Must(template.New("request_submission").Parse(`Hello,

Someone has uploaded a file to your file request "{{ .RequestName }}" on TenzoShare.

File: {{ .Filename }}{{ if .SubmitterName }}
Submitted by: {{ .SubmitterName }}{{ end }}

You can review the submission here:
{{ .ReviewURL }}

— The TenzoShare team
`))

var emailVerificationTpl = template.Must(template.New("email_verification").Parse(`Hello,

Please verify your email address to complete your TenzoShare registration.

Click the link below to verify your email (valid for 24 hours):
{{ .VerificationURL }}

If you didn’t create a TenzoShare account, you can safely ignore this email.

— The TenzoShare team
`))
var transferExpiryReminderTpl = template.Must(template.New("transfer_expiry_reminder").Parse(`Hello,

This is a reminder that the following transfer is expiring within the next 24 hours:

Transfer: {{ .Title }}
Download link: {{ .DownloadURL }}
Expires: {{ .ExpiresAt }}

Please download any files you need before it expires.

— The TenzoShare team
`))

var transferRevokedTpl = template.Must(template.New("transfer_revoked").Parse(`Hello,

The following transfer shared with you has been revoked by the sender and is no longer available:

Transfer: {{ .Title }}{{ if .SenderEmail }}
Sender: {{ .SenderEmail }}{{ end }}

If you have not yet downloaded the files, please contact the sender directly.

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

// RenderRequestSubmission returns a formatted email body notifying a request owner of a new submission.
func RenderRequestSubmission(data RequestSubmissionData) (string, error) {
	var buf bytes.Buffer
	if err := requestSubmissionTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderEmailVerification returns a formatted email body with the email verification link.
func RenderEmailVerification(data EmailVerificationData) (string, error) {
	var buf bytes.Buffer
	if err := emailVerificationTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderTransferExpiryReminder returns a formatted email body for an expiry reminder.
func RenderTransferExpiryReminder(data TransferExpiryReminderData) (string, error) {
	var buf bytes.Buffer
	if err := transferExpiryReminderTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderTransferRevoked returns a formatted email body notifying a recipient that a transfer was revoked.
func RenderTransferRevoked(data TransferRevokedData) (string, error) {
	var buf bytes.Buffer
	if err := transferRevokedTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type TransferReceivedData struct {
	SenderName  string
	Slug        string
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
	Slug           string
	RecipientEmail string
	DownloadedAt   string
	DownloadURL    string
}

type RequestSubmissionData struct {
	RequestName   string
	Filename      string
	SubmitterName string
	ReviewURL     string
}

type EmailVerificationData struct {
	VerificationURL string
}

type TransferExpiryReminderData struct {
	Title       string
	DownloadURL string
	ExpiresAt   string
}

type TransferRevokedData struct {
	Title       string
	SenderEmail string
}
