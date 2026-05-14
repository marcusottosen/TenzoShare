package email

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const brandingCacheTTL = 5 * time.Minute

// BrandingData holds the subset of admin branding config used in email templates.
type BrandingData struct {
	AppName      string
	PrimaryColor string
	LogoURL      string // data-URL or absolute URL; empty = show text logo
	// Email white-label fields
	EmailSenderName    string // display name for From: header
	EmailSupportEmail  string // contact address shown in footer
	EmailFooterText    string // custom message at bottom of every email
	EmailSubjectPrefix string // prepended to every subject, e.g. "[Acme] "
	EmailHeaderLink    string // URL the header logo/name links to
	EmailReplyTo       string // Reply-To header address
	// Custom email colors (empty = fall back to built-in defaults)
	EmailButtonColor     string // CTA button background; fallback: PrimaryColor
	EmailButtonTextColor string // CTA button text; fallback: #ffffff
	EmailBodyBgColor     string // outer wrapper background; fallback: #f1f5f9
	EmailCardBgColor     string // info card background; fallback: #f8fafc
	EmailCardBorderColor string // info card border; fallback: #e2e8f0
	EmailHeadingColor    string // heading text; fallback: #1e293b
	EmailTextColor       string // body paragraph text; fallback: #475569
	// Per-type subject templates (empty = use built-in default)
	// Placeholders: {{AppName}}, {{Title}}, {{RequestName}}
	SubjectTransferReceived     string
	SubjectPasswordReset        string
	SubjectEmailVerification    string
	SubjectDownloadNotification string
	SubjectExpiryReminder       string
	SubjectTransferRevoked      string
	SubjectRequestSubmission    string
	// Per-type CTA button text (empty = use template default)
	CTATransferReceived     string
	CTADownloadNotification string
	CTAPasswordReset        string
	CTAEmailVerification    string
	CTAExpiryReminder       string
	CTARequestSubmission    string
	// Per-type fully custom HTML templates (empty = use standard branded template)
	CustomTransferReceived     string
	CustomPasswordReset        string
	CustomEmailVerification    string
	CustomDownloadNotification string
	CustomExpiryReminder       string
	CustomTransferRevoked      string
	CustomRequestSubmission    string
}

var defaultBranding = BrandingData{
	AppName:      "TenzoShare",
	PrimaryColor: "#1E293B",
}

// BrandingFetcher fetches branding from the admin service public endpoint
// and caches the result for brandingCacheTTL. On fetch failure it returns
// the last-known-good value (or defaults if never fetched).
type BrandingFetcher struct {
	adminURL  string
	client    *http.Client
	log       *zap.Logger
	mu        sync.RWMutex
	cached    BrandingData
	fetchedAt time.Time
	hasData   bool
}

// NewBrandingFetcher creates a BrandingFetcher that reads from adminURL.
// adminURL should be the base URL of the admin service, e.g. "http://tenzoshare-admin:8087".
func NewBrandingFetcher(adminURL string, log *zap.Logger) *BrandingFetcher {
	return &BrandingFetcher{
		adminURL: adminURL,
		client:   &http.Client{Timeout: 5 * time.Second},
		log:      log,
		cached:   defaultBranding,
	}
}

// Get returns the current branding, refreshing from the admin service if the TTL has expired.
func (f *BrandingFetcher) Get() BrandingData {
	f.mu.RLock()
	if f.hasData && time.Since(f.fetchedAt) < brandingCacheTTL {
		data := f.cached
		f.mu.RUnlock()
		return data
	}
	f.mu.RUnlock()

	fresh, err := f.fetch()
	if err != nil {
		f.log.Warn("failed to fetch branding; using cached/default", zap.Error(err))
		f.mu.RLock()
		data := f.cached
		f.mu.RUnlock()
		return data
	}

	f.mu.Lock()
	f.cached = fresh
	f.fetchedAt = time.Now()
	f.hasData = true
	f.mu.Unlock()
	return fresh
}

func (f *BrandingFetcher) fetch() (BrandingData, error) {
	resp, err := f.client.Get(f.adminURL + "/api/v1/branding") //nolint:noctx
	if err != nil {
		return BrandingData{}, fmt.Errorf("fetch branding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BrandingData{}, fmt.Errorf("fetch branding: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BrandingData{}, fmt.Errorf("read branding response: %w", err)
	}

	var raw struct {
		AppName                     string  `json:"app_name"`
		PrimaryColor                string  `json:"primary_color"`
		LogoDataURL                 *string `json:"logo_data_url"`
		EmailSenderName             string  `json:"email_sender_name"`
		EmailSupportEmail           string  `json:"email_support_email"`
		EmailFooterText             string  `json:"email_footer_text"`
		EmailSubjectPrefix          string  `json:"email_subject_prefix"`
		EmailHeaderLink             string  `json:"email_header_link"`
		EmailReplyTo                string  `json:"email_reply_to"`
		EmailButtonColor            string  `json:"email_button_color"`
		EmailButtonTextColor        string  `json:"email_button_text_color"`
		EmailBodyBgColor            string  `json:"email_body_bg_color"`
		EmailCardBgColor            string  `json:"email_card_bg_color"`
		EmailCardBorderColor        string  `json:"email_card_border_color"`
		EmailHeadingColor           string  `json:"email_heading_color"`
		EmailTextColor              string  `json:"email_text_color"`
		SubjectTransferReceived     string  `json:"subject_transfer_received"`
		SubjectPasswordReset        string  `json:"subject_password_reset"`
		SubjectEmailVerification    string  `json:"subject_email_verification"`
		SubjectDownloadNotification string  `json:"subject_download_notification"`
		SubjectExpiryReminder       string  `json:"subject_expiry_reminder"`
		SubjectTransferRevoked      string  `json:"subject_transfer_revoked"`
		SubjectRequestSubmission    string  `json:"subject_request_submission"`
		CTATransferReceived         string  `json:"cta_transfer_received"`
		CTADownloadNotification     string  `json:"cta_download_notification"`
		CTAPasswordReset            string  `json:"cta_password_reset"`
		CTAEmailVerification        string  `json:"cta_email_verification"`
		CTAExpiryReminder           string  `json:"cta_expiry_reminder"`
		CTARequestSubmission        string  `json:"cta_request_submission"`
		CustomTransferReceived      string  `json:"custom_transfer_received"`
		CustomPasswordReset         string  `json:"custom_password_reset"`
		CustomEmailVerification     string  `json:"custom_email_verification"`
		CustomDownloadNotification  string  `json:"custom_download_notification"`
		CustomExpiryReminder        string  `json:"custom_expiry_reminder"`
		CustomTransferRevoked       string  `json:"custom_transfer_revoked"`
		CustomRequestSubmission     string  `json:"custom_request_submission"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return BrandingData{}, fmt.Errorf("parse branding response: %w", err)
	}

	bd := BrandingData{
		AppName:                     raw.AppName,
		PrimaryColor:                raw.PrimaryColor,
		EmailSenderName:             raw.EmailSenderName,
		EmailSupportEmail:           raw.EmailSupportEmail,
		EmailFooterText:             raw.EmailFooterText,
		EmailSubjectPrefix:          raw.EmailSubjectPrefix,
		EmailHeaderLink:             raw.EmailHeaderLink,
		EmailReplyTo:                raw.EmailReplyTo,
		EmailButtonColor:            raw.EmailButtonColor,
		EmailButtonTextColor:        raw.EmailButtonTextColor,
		EmailBodyBgColor:            raw.EmailBodyBgColor,
		EmailCardBgColor:            raw.EmailCardBgColor,
		EmailCardBorderColor:        raw.EmailCardBorderColor,
		EmailHeadingColor:           raw.EmailHeadingColor,
		EmailTextColor:              raw.EmailTextColor,
		SubjectTransferReceived:     raw.SubjectTransferReceived,
		SubjectPasswordReset:        raw.SubjectPasswordReset,
		SubjectEmailVerification:    raw.SubjectEmailVerification,
		SubjectDownloadNotification: raw.SubjectDownloadNotification,
		SubjectExpiryReminder:       raw.SubjectExpiryReminder,
		SubjectTransferRevoked:      raw.SubjectTransferRevoked,
		SubjectRequestSubmission:    raw.SubjectRequestSubmission,
		CTATransferReceived:         raw.CTATransferReceived,
		CTADownloadNotification:     raw.CTADownloadNotification,
		CTAPasswordReset:            raw.CTAPasswordReset,
		CTAEmailVerification:        raw.CTAEmailVerification,
		CTAExpiryReminder:           raw.CTAExpiryReminder,
		CTARequestSubmission:        raw.CTARequestSubmission,
		CustomTransferReceived:      raw.CustomTransferReceived,
		CustomPasswordReset:         raw.CustomPasswordReset,
		CustomEmailVerification:     raw.CustomEmailVerification,
		CustomDownloadNotification:  raw.CustomDownloadNotification,
		CustomExpiryReminder:        raw.CustomExpiryReminder,
		CustomTransferRevoked:       raw.CustomTransferRevoked,
		CustomRequestSubmission:     raw.CustomRequestSubmission,
	}
	if bd.AppName == "" {
		bd.AppName = defaultBranding.AppName
	}
	if bd.PrimaryColor == "" {
		bd.PrimaryColor = defaultBranding.PrimaryColor
	}
	if raw.LogoDataURL != nil && *raw.LogoDataURL != "" {
		bd.LogoURL = *raw.LogoDataURL
	}
	return bd, nil
}
