package email

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	htmltpl "html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

// htmlTemplates holds one parsed template set per email event type.
// Each set contains the layout template plus the per-type content partial.
var htmlTemplates map[string]*htmltpl.Template

func init() {
	types := []string{
		"email_verification",
		"password_reset",
		"transfer_received",
		"download_notification",
		"request_submission",
		"transfer_expiry_reminder",
		"transfer_revoked",
	}
	htmlTemplates = make(map[string]*htmltpl.Template, len(types))
	for _, t := range types {
		tpl := htmltpl.Must(
			htmltpl.New("").ParseFS(
				templatesFS,
				"templates/layout.html",
				"templates/"+t+".html",
			),
		)
		htmlTemplates[t] = tpl
	}
}

// renderHTML renders the named email type as HTML, injecting branding into the
// template data. The data argument must be a struct whose fields are accessible
// in the template; URL fields are marked safe so html/template does not re-encode them.
// unsubscribeURL is injected as the "UnsubscribeURL" template variable.
func renderHTML(emailType string, branding BrandingData, data any, unsubscribeURL string) (string, error) {
	tpl, ok := htmlTemplates[emailType]
	if !ok {
		return "", fmt.Errorf("unknown email type for HTML rendering: %s", emailType)
	}

	// Flatten the typed data struct into map[string]any so all fields are
	// accessible at the top level of the template context alongside branding.
	raw, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal email data: %w", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", fmt.Errorf("unmarshal email data to map: %w", err)
	}

	// Inject branding — typed so html/template treats them correctly.
	m["AppName"] = branding.AppName
	m["PrimaryColor"] = htmltpl.CSS(branding.PrimaryColor) // safe CSS value
	if branding.LogoURL != "" {
		m["LogoURL"] = htmltpl.URL(branding.LogoURL)
	} else {
		m["LogoURL"] = ""
	}
	m["EmailSenderName"] = branding.EmailSenderName
	m["EmailSupportEmail"] = branding.EmailSupportEmail
	m["EmailFooterText"] = branding.EmailFooterText
	if branding.EmailHeaderLink != "" {
		m["EmailHeaderLink"] = htmltpl.URL(branding.EmailHeaderLink)
	} else {
		m["EmailHeaderLink"] = ""
	}
	// Show "Powered by TenzoShare" when the app has been rebranded.
	m["ShowPoweredBy"] = branding.AppName != "TenzoShare" && branding.AppName != ""

	// Resolved email colors — fall back to sensible defaults when not configured.
	resolveCSS := func(configured, fallback string) htmltpl.CSS {
		if configured != "" {
			return htmltpl.CSS(configured)
		}
		return htmltpl.CSS(fallback)
	}
	m["ButtonColor"] = resolveCSS(branding.EmailButtonColor, branding.PrimaryColor)
	m["ButtonTextColor"] = resolveCSS(branding.EmailButtonTextColor, "#ffffff")
	m["BodyBgColor"] = resolveCSS(branding.EmailBodyBgColor, "#f1f5f9")
	m["CardBgColor"] = resolveCSS(branding.EmailCardBgColor, "#f8fafc")
	m["CardBorderColor"] = resolveCSS(branding.EmailCardBorderColor, "#e2e8f0")
	m["HeadingColor"] = resolveCSS(branding.EmailHeadingColor, "#1e293b")
	m["TextColor"] = resolveCSS(branding.EmailTextColor, "#475569")

	// Per-type CTA button text — admin-configured or template defaults.
	ctaDefaults := map[string]string{
		"transfer_received":        "View &amp; Download Files",
		"download_notification":    "View Transfer",
		"password_reset":           "Reset Password",
		"email_verification":       "Verify Email Address",
		"transfer_expiry_reminder": "Download Files Now",
		"request_submission":       "Review Submission",
	}
	ctaOverrides := map[string]string{
		"transfer_received":        branding.CTATransferReceived,
		"download_notification":    branding.CTADownloadNotification,
		"password_reset":           branding.CTAPasswordReset,
		"email_verification":       branding.CTAEmailVerification,
		"transfer_expiry_reminder": branding.CTAExpiryReminder,
		"request_submission":       branding.CTARequestSubmission,
	}
	cta := ctaOverrides[emailType]
	if cta == "" {
		cta = ctaDefaults[emailType]
	}
	m["CTAButtonText"] = cta

	// Mark known URL fields as safe so html/template does not re-encode them.
	for _, field := range []string{
		"DownloadURL", "VerificationURL", "ResetURL", "ReviewURL",
	} {
		if v, ok := m[field].(string); ok && v != "" {
			m[field] = htmltpl.URL(v)
		}
	}

	// Inject unsubscribe URL.
	if unsubscribeURL != "" {
		m["UnsubscribeURL"] = htmltpl.URL(unsubscribeURL)
	} else {
		m["UnsubscribeURL"] = ""
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout", m); err != nil {
		return "", fmt.Errorf("execute HTML template %s: %w", emailType, err)
	}
	return buf.String(), nil
}
