// Package email provides SMTP email delivery for the notification service.
// It supports three TLS modes via SMTP_USE_TLS:
//   - false (default): plain TCP with automatic STARTTLS upgrade when offered (port 587)
//   - true: implicit TLS from the start of the connection (SMTPS, port 465)
package email

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

const (
	smtpDialTimeout = 15 * time.Second
	smtpOpTimeout   = 60 * time.Second
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
	To                 []string
	FromName           string // optional display name for the From header, e.g. "Acme Transfers"
	ReplyTo            string // optional Reply-To address
	Subject            string
	Body               string // plain-text body (always set)
	HTMLBody           string // HTML body; if set, message is sent as multipart/alternative
	ListUnsubscribeURL string // if set, adds List-Unsubscribe + List-Unsubscribe-Post headers
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
// If a fully custom template is configured for this email type it is used
// (with {{Tag}} substitution) instead of the standard branded layout.
// unsubscribeURL is embedded in the footer of standard templates; it is
// available as {{UnsubscribeURL}} in custom templates.
// Returns an empty string on error so the plain-text fallback is always used.
func (s *Sender) RenderHTML(emailType string, data any, unsubscribeURL string) string {
	var b BrandingData
	if s.branding != nil {
		b = s.branding.Get()
	} else {
		b = defaultBranding
	}

	// Per-type custom template overrides.
	customTemplates := map[string]string{
		"transfer_received":        b.CustomTransferReceived,
		"password_reset":           b.CustomPasswordReset,
		"email_verification":       b.CustomEmailVerification,
		"download_notification":    b.CustomDownloadNotification,
		"transfer_expiry_reminder": b.CustomExpiryReminder,
		"transfer_revoked":         b.CustomTransferRevoked,
		"request_submission":       b.CustomRequestSubmission,
		"request_invite":           "",
	}
	if customTpl := customTemplates[emailType]; customTpl != "" {
		// Inject unsubscribe URL as a substitutable tag too.
		result := renderCustomHTML(customTpl, emailType, b, data)
		result = strings.ReplaceAll(result, "{{UnsubscribeURL}}", unsubscribeURL)
		return result
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
	body := buildMIME(cfg.From, msg.FromName, msg.ReplyTo, msg.To, msg.Subject, msg.Body, msg.HTMLBody, msg.ListUnsubscribeURL)

	if cfg.UseTLS {
		// Implicit TLS (SMTPS): TLS wraps the TCP connection from the very start.
		// tls.DialWithDialer is used so we get a dial timeout; an op deadline is
		// set on the resulting connection to bound the entire SMTP transaction.
		netDialer := &net.Dialer{Timeout: smtpDialTimeout}
		conn, err := tls.DialWithDialer(netDialer, "tcp", addr, &tls.Config{ServerName: cfg.Host}) //nolint:gosec
		if err != nil {
			return fmt.Errorf("smtp implicit-tls dial: %w", err)
		}
		defer conn.Close()                              //nolint:errcheck
		conn.SetDeadline(time.Now().Add(smtpOpTimeout)) //nolint:errcheck

		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer client.Close() //nolint:errcheck

		// smtp.PlainAuth refuses to send credentials when smtp.Client.tls==false,
		// even though the underlying connection IS TLS (the client just doesn't know
		// because TLS was not started via StartTLS). Use our own Auth impl that
		// skips the check — safe because we know the transport is already encrypted.
		if cfg.Username != "" {
			if err := client.Auth(implicitTLSPlainAuth{cfg.Username, cfg.Password}); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		return smtpSendData(client, cfg.From, msg.To, body)
	}

	// Plain TCP path — STARTTLS upgrade is attempted if the server advertises it.
	// We dial manually (instead of using smtp.SendMail) so we can apply timeouts.
	tcpConn, err := (&net.Dialer{Timeout: smtpDialTimeout}).Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer tcpConn.Close()                              //nolint:errcheck
	tcpConn.SetDeadline(time.Now().Add(smtpOpTimeout)) //nolint:errcheck

	client, err := smtp.NewClient(tcpConn, cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close() //nolint:errcheck

	// Upgrade to TLS if the server advertises STARTTLS.
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil { //nolint:gosec
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if cfg.Username != "" {
		// PlainAuth correctly checks c.tls==true after StartTLS.
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	return smtpSendData(client, cfg.From, msg.To, body)
}

// smtpSendData issues MAIL FROM, RCPT TO, and DATA commands to deliver body.
func smtpSendData(client *smtp.Client, from string, to []string, body []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("smtp RCPT TO %s: %w", addr, err)
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

// implicitTLSPlainAuth implements smtp.Auth for connections already wrapped in
// TLS at the TCP level (implicit TLS / SMTPS, port 465). It behaves identically
// to smtp.PlainAuth but omits the TLS check that would otherwise fail because
// smtp.Client is unaware TLS is active when the connection was pre-dialed.
type implicitTLSPlainAuth struct{ username, password string }

func (a implicitTLSPlainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	resp := "\x00" + a.username + "\x00" + a.password
	return "PLAIN", []byte(resp), nil
}

func (a implicitTLSPlainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected server challenge during PLAIN auth")
	}
	return nil, nil
}

// buildMIME constructs the raw SMTP message bytes. When htmlBody is non-empty
// the message uses multipart/alternative so clients can choose the best part.
// All bodies are base64-encoded (Content-Transfer-Encoding: base64) so that
// non-ASCII content in user-supplied strings is transmitted safely through
// strict 7-bit MTAs.
func buildMIME(from, fromName, replyTo string, to []string, subject, plainBody, htmlBody, listUnsubURL string) []byte {
	var buf bytes.Buffer

	// From header — RFC 5322 display name, RFC 2047 encoded if non-ASCII.
	if fromName != "" {
		fmt.Fprintf(&buf, "From: %s <%s>\r\n", encodeDisplayName(fromName), from)
	} else {
		fmt.Fprintf(&buf, "From: %s\r\n", from)
	}
	if replyTo != "" {
		fmt.Fprintf(&buf, "Reply-To: %s\r\n", replyTo)
	}
	fmt.Fprintf(&buf, "To: %s\r\n", joinAddrs(to))
	// Subject — RFC 2047 B-encoded when non-ASCII characters are present.
	fmt.Fprintf(&buf, "Subject: %s\r\n", encodeHeader(subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))

	// Message-ID — required by RFC 5322; used by spam filters and threading.
	domain := "mail.local"
	if at := strings.LastIndex(from, "@"); at >= 0 {
		domain = from[at+1:]
	}
	fmt.Fprintf(&buf, "Message-ID: <%d@%s>\r\n", time.Now().UnixNano(), domain)

	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	// List-Unsubscribe — displayed as an unsubscribe button by Gmail / Outlook.
	if listUnsubURL != "" {
		fmt.Fprintf(&buf, "List-Unsubscribe: <%s>\r\n", listUnsubURL)
		fmt.Fprintf(&buf, "List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
	}

	if htmlBody == "" {
		// Plain-text only — base64-encoded for safe 8-bit transport.
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&buf, "\r\n")
		buf.WriteString(mimeBase64([]byte(plainBody)))
		return buf.Bytes()
	}

	// multipart/alternative — text/plain first (lowest fidelity), text/html last (preferred).
	// Boundary does NOT start with '--'; the delimiter adds those automatically.
	boundary := fmt.Sprintf("tenzoshare_%d", time.Now().UnixNano())
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
	fmt.Fprintf(&buf, "\r\n")

	// plain part
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&buf, "\r\n")
	buf.WriteString(mimeBase64([]byte(plainBody)))
	fmt.Fprintf(&buf, "\r\n")

	// HTML part
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&buf, "\r\n")
	buf.WriteString(mimeBase64([]byte(htmlBody)))
	fmt.Fprintf(&buf, "\r\n")

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	return buf.Bytes()
}

// mimeBase64 returns data as base64 with 76-character line wrapping per RFC 2045.
func mimeBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var sb strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		sb.WriteString(encoded[i:end])
		sb.WriteString("\r\n")
	}
	return sb.String()
}

// encodeHeader RFC 2047 B-encodes s when it contains non-ASCII characters,
// otherwise returns s unchanged. Used for Subject and other unstructured headers.
func encodeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return mime.BEncoding.Encode("UTF-8", s)
		}
	}
	return s
}

// encodeDisplayName returns a safe RFC 5322 display-name token. ASCII names are
// quoted with double-quotes; non-ASCII names are RFC 2047 Q-encoded.
func encodeDisplayName(name string) string {
	for _, r := range name {
		if r > 127 {
			return mime.QEncoding.Encode("UTF-8", name)
		}
	}
	// ASCII: use a simple double-quoted string. Escape embedded " and \.
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name)
	return `"` + escaped + `"`
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

var requestInviteTpl = template.Must(template.New("request_invite").Parse(`Hello,

You've been asked to upload files to the following request on TenzoShare.

Request: {{ .RequestName }}

Click the link below to submit your files — no account required:
{{ .UploadURL }}

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

// RenderRequestInvite returns a formatted email body inviting a recipient to upload files.
func RenderRequestInvite(data RequestInviteData) (string, error) {
	var buf bytes.Buffer
	if err := requestInviteTpl.Execute(&buf, data); err != nil {
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

type RequestInviteData struct {
	RequestName string
	UploadURL   string
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
