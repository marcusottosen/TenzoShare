// Package consumer subscribes to NATS JetStream subjects and dispatches
// email delivery based on the event type.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/notification/internal/email"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
)

// EmailEvent is the canonical payload published to NOTIFICATIONS.email by any
// TenzoShare service. The Type field selects which email template to render.
type EmailEvent struct {
	Type string          `json:"type"` // "transfer_received" | "password_reset" | "download_notification" | "request_submission" | "email_verification" | "transfer_expiry_reminder" | "transfer_revoked"
	To   []string        `json:"to"`
	Data json.RawMessage `json:"data"`
}

const platformURLCacheTTL = 5 * time.Minute

// Consumer subscribes to NOTIFICATIONS.email and delivers emails.
type Consumer struct {
	js          *jetstream.Client
	sender      *email.Sender
	log         *zap.Logger
	pepper      string // for HMAC unsubscribe token generation
	baseURL     string // fallback base URL (from env) used when admin platform URLs are not set
	authBaseURL string // internal auth service URL for preference checks
	adminURL    string // internal admin service URL for platform config

	// cached platform URLs from admin service
	platformMu          sync.RWMutex
	cachedPortalURL     string
	cachedDownloadURL   string
	platformURLsFetched time.Time
	platformURLsHasData bool
	platformURLsClient  *http.Client
}

func New(js *jetstream.Client, sender *email.Sender, log *zap.Logger, pepper, baseURL, authBaseURL, adminURL string) *Consumer {
	return &Consumer{
		js:                 js,
		sender:             sender,
		log:                log,
		pepper:             pepper,
		baseURL:            baseURL,
		authBaseURL:        authBaseURL,
		adminURL:           adminURL,
		platformURLsClient: &http.Client{Timeout: 3 * time.Second},
	}
}

// Start blocks until ctx is done. It subscribes to NOTIFICATIONS.email using
// a durable consumer named "notification-service".
func (c *Consumer) Start(ctx context.Context) error {
	c.log.Info("notification consumer starting")
	// Warm up platform URL cache in background.
	go c.refreshPlatformURLs()
	return c.js.Subscribe(ctx, "NOTIFICATIONS", "notification-service", "NOTIFICATIONS.email",
		func(subject string, data []byte) error {
			return c.handle(subject, data)
		},
	)
}

func (c *Consumer) handle(subject string, data []byte) error {
	// Refresh platform URLs if the cache is stale (non-blocking best-effort).
	go c.refreshPlatformURLs()

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

	// F3: Check notification preferences for the primary recipient.
	// Non-critical: on error we send the email (fail-open).
	recipient := ev.To[0]
	if !c.shouldSend(ev.Type, recipient) {
		c.log.Info("email suppressed by user preference",
			zap.String("type", ev.Type), zap.String("to", recipient))
		return nil
	}

	// F1: Generate a signed unsubscribe URL for the primary recipient.
	// Password-reset and email-verification emails are security-critical and
	// must not include an unsubscribe link that could suppress them.
	var unsubURL string
	if ev.Type != "password_reset" && ev.Type != "email_verification" {
		token := crypto.UnsubscribeToken(recipient, c.pepper)
		// Use the base URL (falls back to env) for the API endpoint.
		apiBase := c.resolveBase(c.baseURL)
		unsubURL = fmt.Sprintf("%s/api/v1/auth/unsubscribe?token=%s", apiBase, token)
	}

	var (
		subject2 string
		body     string
		htmlData any
		err      error
	)

	switch ev.Type {
	case "transfer_received":
		var d email.TransferReceivedData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		// Override download URL if a platform download_url is configured.
		if d.Slug != "" {
			if base := c.platformDownloadURL(); base != "" {
				d.DownloadURL = base + "/t/" + d.Slug
			}
		}
		subject2 = "You've received files via TenzoShare: " + d.Title
		body, err = email.RenderTransferReceived(d)
		htmlData = d

	case "password_reset":
		var d email.PasswordResetData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		// Override ResetURL base with portal_url if configured.
		if portalBase := c.platformPortalURL(); portalBase != "" {
			d.ResetURL = rebaseURL(d.ResetURL, portalBase)
		}
		subject2 = "Reset your TenzoShare password"
		body, err = email.RenderPasswordReset(d)
		htmlData = d

	case "download_notification":
		var d email.DownloadNotificationData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		// Override download URL if a platform download_url is configured.
		if d.Slug != "" {
			if base := c.platformDownloadURL(); base != "" {
				d.DownloadURL = base + "/t/" + d.Slug
			}
		}
		subject2 = "Your transfer was downloaded"
		body, err = email.RenderDownloadNotification(d)
		htmlData = d

	case "request_submission":
		var d email.RequestSubmissionData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "New file submitted to your request: " + d.RequestName
		body, err = email.RenderRequestSubmission(d)
		htmlData = d

	case "email_verification":
		var d email.EmailVerificationData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		// Override VerificationURL base with portal_url if configured.
		if portalBase := c.platformPortalURL(); portalBase != "" {
			d.VerificationURL = rebaseURL(d.VerificationURL, portalBase)
		}
		subject2 = "Verify your TenzoShare account"
		body, err = email.RenderEmailVerification(d)
		htmlData = d

	case "transfer_expiry_reminder":
		var d email.TransferExpiryReminderData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "Your TenzoShare transfer expires soon: " + d.Title
		body, err = email.RenderTransferExpiryReminder(d)
		htmlData = d

	case "transfer_revoked":
		var d email.TransferRevokedData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "A TenzoShare transfer has been revoked: " + d.Title
		body, err = email.RenderTransferRevoked(d)
		htmlData = d

	case "request_invite":
		var d email.RequestInviteData
		if err = json.Unmarshal(ev.Data, &d); err != nil {
			break
		}
		subject2 = "You've been invited to upload files: " + d.RequestName
		body, err = email.RenderRequestInvite(d)
		htmlData = d

	default:
		c.log.Warn("unknown email event type", zap.String("type", ev.Type))
		return nil
	}

	if err != nil {
		c.log.Error("failed to render email template",
			zap.String("type", ev.Type), zap.Error(err))
		return nil
	}

	// Resolve final subject using branding config.
	branding := c.sender.Branding()
	subject2 = c.resolveSubject(ev.Type, subject2, branding)

	// Render HTML version; RenderHTML logs internally and returns "" on failure
	// so the plain-text fallback is always used in that case.
	htmlBody := c.sender.RenderHTML(ev.Type, htmlData, unsubURL)

	sendErr := c.sender.Send(email.Message{
		To:                 ev.To,
		FromName:           branding.EmailSenderName,
		ReplyTo:            branding.EmailReplyTo,
		Subject:            subject2,
		Body:               body,
		HTMLBody:           htmlBody,
		ListUnsubscribeURL: unsubURL,
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

// resolveSubject returns the final email subject for the given event type.
// It prefers per-type subject templates from branding config (with {{AppName}},
// {{Title}}, {{RequestName}} placeholder substitution), then falls back to the
// defaultSubject generated by the switch block in handle(). The global subject
// prefix is applied last.
func (c *Consumer) resolveSubject(emailType, defaultSubject string, b email.BrandingData) string {
	// Per-type template overrides, keyed by event type.
	tplOverrides := map[string]string{
		"transfer_received":        b.SubjectTransferReceived,
		"password_reset":           b.SubjectPasswordReset,
		"email_verification":       b.SubjectEmailVerification,
		"download_notification":    b.SubjectDownloadNotification,
		"transfer_expiry_reminder": b.SubjectExpiryReminder,
		"transfer_revoked":         b.SubjectTransferRevoked,
		"request_submission":       b.SubjectRequestSubmission,
	}

	subject := tplOverrides[emailType]
	if subject == "" {
		// Fall back to the default built in this session, but still swap
		// "TenzoShare" for the configured app name.
		subject = defaultSubject
		if b.AppName != "" && b.AppName != "TenzoShare" {
			subject = strings.ReplaceAll(subject, "TenzoShare", b.AppName)
		}
	} else {
		// Expand placeholders in the admin-configured template.
		subject = strings.ReplaceAll(subject, "{{AppName}}", b.AppName)
		// {{Title}} and {{RequestName}} are already in defaultSubject; for the
		// override we re-extract them from defaultSubject (they appear after the
		// last ": " separator).
		if idx := strings.LastIndex(defaultSubject, ": "); idx >= 0 {
			dynamic := defaultSubject[idx+2:]
			subject = strings.ReplaceAll(subject, "{{Title}}", dynamic)
			subject = strings.ReplaceAll(subject, "{{RequestName}}", dynamic)
		}
	}

	if b.EmailSubjectPrefix != "" {
		subject = b.EmailSubjectPrefix + subject
	}
	return subject
}

// shouldSend queries the auth service's internal notification-prefs endpoint to
// determine whether the email type should be sent to the recipient.
// Returns true (send) on any error so email is never silently dropped due to
// infrastructure issues (fail-open).
//
// Types that bypass preference gating (security/transactional critical):
//
//	password_reset, email_verification
func (c *Consumer) shouldSend(emailType, recipientEmail string) bool {
	// Security-critical emails always bypass opt-out.
	if emailType == "password_reset" || emailType == "email_verification" {
		return true
	}
	if c.authBaseURL == "" {
		return true // not configured — fail open
	}

	url := fmt.Sprintf("%s/api/v1/auth/internal/notification-prefs?email=%s", c.authBaseURL, recipientEmail)
	resp, err := http.Get(url) //nolint:noctx,gosec
	if err != nil || resp.StatusCode != http.StatusOK {
		c.log.Warn("failed to fetch notification prefs; sending email",
			zap.String("type", emailType), zap.String("to", recipientEmail), zap.Error(err))
		return true
	}
	defer resp.Body.Close()

	var prefs struct {
		OptOut               bool `json:"opt_out"`
		TransferReceived     bool `json:"transfer_received"`
		DownloadNotification bool `json:"download_notification"`
		ExpiryReminders      bool `json:"expiry_reminders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prefs); err != nil {
		return true // fail-open
	}
	if prefs.OptOut {
		return false
	}
	switch emailType {
	case "transfer_received":
		return prefs.TransferReceived
	case "download_notification":
		return prefs.DownloadNotification
	case "transfer_expiry_reminder":
		return prefs.ExpiryReminders
	default:
		// request_submission, transfer_revoked — always send
		return true
	}
}

// ── Platform URL helpers ──────────────────────────────────────────────────────

// platformPortalURL returns the portal URL from admin config, or "" if unset.
// Safe to call from any goroutine.
func (c *Consumer) platformPortalURL() string {
	c.platformMu.RLock()
	defer c.platformMu.RUnlock()
	return c.cachedPortalURL
}

// platformDownloadURL returns the download URL from admin config, or "" if unset.
func (c *Consumer) platformDownloadURL() string {
	c.platformMu.RLock()
	defer c.platformMu.RUnlock()
	return c.cachedDownloadURL
}

// resolveBase returns adminURL if it is non-empty, otherwise falls back to fallback.
func (c *Consumer) resolveBase(fallback string) string {
	if b := c.platformPortalURL(); b != "" {
		return b
	}
	return fallback
}

// refreshPlatformURLs fetches portal_url and download_url from the admin service
// and caches them. Should be called periodically or on startup.
func (c *Consumer) refreshPlatformURLs() {
	c.platformMu.RLock()
	needRefresh := !c.platformURLsHasData || time.Since(c.platformURLsFetched) > platformURLCacheTTL
	c.platformMu.RUnlock()
	if !needRefresh {
		return
	}
	if c.adminURL == "" {
		return
	}

	resp, err := c.platformURLsClient.Get(c.adminURL + "/api/v1/platform/config") //nolint:noctx
	if err != nil {
		c.log.Warn("failed to fetch platform config for URL rewriting; using fallback", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	var cfg struct {
		PortalURL   string `json:"portal_url"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		c.log.Warn("failed to parse platform config response", zap.Error(err))
		return
	}

	c.platformMu.Lock()
	c.cachedPortalURL = cfg.PortalURL
	c.cachedDownloadURL = cfg.DownloadURL
	c.platformURLsFetched = time.Now()
	c.platformURLsHasData = true
	c.platformMu.Unlock()

	c.log.Debug("platform URLs refreshed",
		zap.String("portal_url", cfg.PortalURL),
		zap.String("download_url", cfg.DownloadURL))
}

// rebaseURL replaces the scheme+host of rawURL with newBase.
// If parsing fails the original rawURL is returned unchanged.
func rebaseURL(rawURL, newBase string) string {
	if rawURL == "" || newBase == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	base, err := url.Parse(newBase)
	if err != nil {
		return rawURL
	}
	parsed.Scheme = base.Scheme
	parsed.Host = base.Host
	return parsed.String()
}
