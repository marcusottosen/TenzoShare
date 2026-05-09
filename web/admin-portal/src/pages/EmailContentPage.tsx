import React, { useEffect, useState } from 'react';
import { getBranding, updateBranding, type BrandingConfig } from '../api/admin';

// ── ColorPicker (local, same pattern as BrandingPage) ────────────────────────

function ColorPicker({
  label,
  description,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  description: string;
  value: string;
  placeholder?: string;
  onChange: (v: string) => void;
}) {
  const display = value || placeholder || '#000000';
  return (
    <div className="form-group">
      <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>{label}</label>
      <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 8 }}>
        {description}
        {!value && placeholder && (
          <span style={{ color: 'var(--color-text-muted)', fontStyle: 'italic' }}>
            {' '}— default: <code>{placeholder}</code>
          </span>
        )}
      </p>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input
          type="color"
          value={display}
          onChange={(e) => onChange(e.target.value)}
          style={{ width: 44, height: 34, padding: 2, cursor: 'pointer', borderRadius: 6, border: '1px solid var(--color-border)' }}
        />
        <input
          type="text"
          maxLength={7}
          value={value}
          placeholder={placeholder}
          onChange={(e) => {
            const v = e.target.value;
            if (v === '' || /^#[0-9A-Fa-f]{0,6}$/.test(v)) onChange(v);
          }}
          style={{ width: 110, fontFamily: 'monospace' }}
        />
        {value && (
          <button
            type="button"
            onClick={() => onChange('')}
            title="Clear (use default)"
            style={{ fontSize: 11, color: 'var(--color-text-muted)', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline', padding: 0 }}
          >
            reset
          </button>
        )}
        <div style={{ width: 28, height: 28, borderRadius: 6, background: display, border: '1px solid var(--color-border)', flexShrink: 0 }} />
      </div>
    </div>
  );
}

// ── Email type metadata ───────────────────────────────────────────────────────

interface EmailTypeMeta {
  key: string;
  label: string;
  defaultSubject: string;
  defaultCTA?: string; // undefined = no CTA button
  subjectField: keyof BrandingConfig;
  ctaField?: keyof BrandingConfig;
}

const EMAIL_TYPES: EmailTypeMeta[] = [
  {
    key: 'transfer_received',
    label: 'Files Received',
    defaultSubject: "You've received files via {{AppName}}: {{Title}}",
    defaultCTA: 'View & Download Files',
    subjectField: 'subject_transfer_received',
    ctaField: 'cta_transfer_received',
  },
  {
    key: 'password_reset',
    label: 'Password Reset',
    defaultSubject: 'Reset your {{AppName}} password',
    defaultCTA: 'Reset Password',
    subjectField: 'subject_password_reset',
    ctaField: 'cta_password_reset',
  },
  {
    key: 'email_verification',
    label: 'Email Verification',
    defaultSubject: 'Verify your {{AppName}} account',
    defaultCTA: 'Verify Email Address',
    subjectField: 'subject_email_verification',
    ctaField: 'cta_email_verification',
  },
  {
    key: 'download_notification',
    label: 'Download Notification',
    defaultSubject: 'Your transfer was downloaded',
    defaultCTA: 'View Transfer',
    subjectField: 'subject_download_notification',
    ctaField: 'cta_download_notification',
  },
  {
    key: 'transfer_expiry_reminder',
    label: 'Expiry Reminder',
    defaultSubject: 'Your {{AppName}} transfer expires soon: {{Title}}',
    defaultCTA: 'Download Files Now',
    subjectField: 'subject_expiry_reminder',
    ctaField: 'cta_expiry_reminder',
  },
  {
    key: 'transfer_revoked',
    label: 'Transfer Revoked',
    defaultSubject: 'A {{AppName}} transfer has been revoked: {{Title}}',
    subjectField: 'subject_transfer_revoked',
  },
  {
    key: 'request_submission',
    label: 'File Request Submission',
    defaultSubject: 'New file submitted to your request: {{RequestName}}',
    defaultCTA: 'Review Submission',
    subjectField: 'subject_request_submission',
    ctaField: 'cta_request_submission',
  },
];

// ── Tab type ──────────────────────────────────────────────────────────────────

type Tab = 'delivery' | 'colors' | 'per-type';

// ── Page ─────────────────────────────────────────────────────────────────────

export default function EmailContentPage() {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('delivery');

  // --- Delivery fields ---
  const [senderName, setSenderName] = useState('');
  const [replyTo, setReplyTo] = useState('');
  const [supportEmail, setSupportEmail] = useState('');
  const [subjectPrefix, setSubjectPrefix] = useState('');
  const [headerLink, setHeaderLink] = useState('');
  const [footerText, setFooterText] = useState('');

  // --- Color fields ---
  const [buttonColor, setButtonColor] = useState('');
  const [buttonTextColor, setButtonTextColor] = useState('');
  const [bodyBgColor, setBodyBgColor] = useState('');
  const [cardBgColor, setCardBgColor] = useState('');
  const [cardBorderColor, setCardBorderColor] = useState('');
  const [headingColor, setHeadingColor] = useState('');
  const [emailTextColor, setEmailTextColor] = useState('');

  // --- Per-type subjects & CTAs ---
  const [perTypeSubjects, setPerTypeSubjects] = useState<Record<string, string>>({});
  const [perTypeCTAs, setPerTypeCTAs] = useState<Record<string, string>>({});

  // primary color for button fallback hint
  const [primaryColor, setPrimaryColor] = useState('#1E293B');

  useEffect(() => {
    setLoading(true);
    getBranding()
      .then((c) => {
        setPrimaryColor(c.primary_color ?? '#1E293B');
        // Delivery
        setSenderName(c.email_sender_name ?? '');
        setReplyTo(c.email_reply_to ?? '');
        setSupportEmail(c.email_support_email ?? '');
        setSubjectPrefix(c.email_subject_prefix ?? '');
        setHeaderLink(c.email_header_link ?? '');
        setFooterText(c.email_footer_text ?? '');
        // Colors
        setButtonColor(c.email_button_color ?? '');
        setButtonTextColor(c.email_button_text_color ?? '');
        setBodyBgColor(c.email_body_bg_color ?? '');
        setCardBgColor(c.email_card_bg_color ?? '');
        setCardBorderColor(c.email_card_border_color ?? '');
        setHeadingColor(c.email_heading_color ?? '');
        setEmailTextColor(c.email_text_color ?? '');
        // Per-type
        const subjects: Record<string, string> = {};
        const ctas: Record<string, string> = {};
        for (const t of EMAIL_TYPES) {
          subjects[t.key] = (c[t.subjectField] as string) ?? '';
          if (t.ctaField) ctas[t.key] = (c[t.ctaField] as string) ?? '';
        }
        setPerTypeSubjects(subjects);
        setPerTypeCTAs(ctas);
      })
      .catch(() => setError('Failed to load branding settings.'))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      await updateBranding({
        // Delivery
        email_sender_name: senderName,
        email_reply_to: replyTo,
        email_support_email: supportEmail,
        email_subject_prefix: subjectPrefix,
        email_header_link: headerLink,
        email_footer_text: footerText,
        // Colors
        email_button_color: buttonColor,
        email_button_text_color: buttonTextColor,
        email_body_bg_color: bodyBgColor,
        email_card_bg_color: cardBgColor,
        email_card_border_color: cardBorderColor,
        email_heading_color: headingColor,
        email_text_color: emailTextColor,
        // Per-type subjects
        subject_transfer_received: perTypeSubjects['transfer_received'] ?? '',
        subject_password_reset: perTypeSubjects['password_reset'] ?? '',
        subject_email_verification: perTypeSubjects['email_verification'] ?? '',
        subject_download_notification: perTypeSubjects['download_notification'] ?? '',
        subject_expiry_reminder: perTypeSubjects['transfer_expiry_reminder'] ?? '',
        subject_transfer_revoked: perTypeSubjects['transfer_revoked'] ?? '',
        subject_request_submission: perTypeSubjects['request_submission'] ?? '',
        // Per-type CTAs
        cta_transfer_received: perTypeCTAs['transfer_received'] ?? '',
        cta_download_notification: perTypeCTAs['download_notification'] ?? '',
        cta_password_reset: perTypeCTAs['password_reset'] ?? '',
        cta_email_verification: perTypeCTAs['email_verification'] ?? '',
        cta_expiry_reminder: perTypeCTAs['transfer_expiry_reminder'] ?? '',
        cta_request_submission: perTypeCTAs['request_submission'] ?? '',
      });
      setSuccess('Email settings saved successfully.');
    } catch {
      setError('Failed to save settings. Please try again.');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <div style={{ padding: 32, color: 'var(--color-text-muted)' }}>Loading…</div>;
  }

  const tabStyle = (t: Tab): React.CSSProperties => ({
    padding: '8px 20px',
    borderRadius: 6,
    border: 'none',
    cursor: 'pointer',
    fontWeight: 600,
    fontSize: 14,
    background: activeTab === t ? 'var(--color-primary)' : 'transparent',
    color: activeTab === t ? '#fff' : 'var(--color-text-muted)',
    transition: 'background 0.15s',
  });

  return (
    <div style={{ padding: 32, maxWidth: 780 }}>
      <h1 style={{ marginBottom: 6 }}>Email Content</h1>
      <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 28 }}>
        Configure how transactional emails are composed and branded. Changes apply to all newly sent emails.
      </p>

      {error && (
        <div className="alert alert-error" style={{ marginBottom: 20 }}>{error}</div>
      )}
      {success && (
        <div className="alert alert-success" style={{ marginBottom: 20 }}>{success}</div>
      )}

      {/* Tab bar */}
      <div style={{ display: 'flex', gap: 4, marginBottom: 28, background: 'var(--color-surface)', borderRadius: 8, padding: 4, width: 'fit-content', border: '1px solid var(--color-border)' }}>
        <button type="button" style={tabStyle('delivery')} onClick={() => setActiveTab('delivery')}>Delivery</button>
        <button type="button" style={tabStyle('colors')} onClick={() => setActiveTab('colors')}>Colors</button>
        <button type="button" style={tabStyle('per-type')} onClick={() => setActiveTab('per-type')}>Per Email Type</button>
      </div>

      <form onSubmit={handleSave}>

        {/* ── Tab: Delivery ─────────────────────────────────────────────── */}
        {activeTab === 'delivery' && (
          <div className="card" style={{ padding: 28, marginBottom: 24 }}>
            <h2 style={{ marginBottom: 20, fontSize: 17 }}>Delivery &amp; Identity</h2>

            <div className="form-group">
              <label>Sender display name</label>
              <input
                type="text"
                maxLength={100}
                value={senderName}
                placeholder="TenzoShare"
                onChange={(e) => setSenderName(e.target.value)}
              />
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Shown in the From field, e.g. "Acme Transfers"
              </p>
            </div>

            <div className="form-group">
              <label>Reply-To address</label>
              <input
                type="email"
                maxLength={254}
                value={replyTo}
                placeholder="support@example.com"
                onChange={(e) => setReplyTo(e.target.value)}
              />
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Optional. When set, replies go here instead of the SMTP sender address.
              </p>
            </div>

            <div className="form-group">
              <label>Support / contact email</label>
              <input
                type="email"
                maxLength={254}
                value={supportEmail}
                placeholder="support@example.com"
                onChange={(e) => setSupportEmail(e.target.value)}
              />
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Shown as a contact link in the email footer.
              </p>
            </div>

            <div className="form-group">
              <label>Subject line prefix</label>
              <input
                type="text"
                maxLength={60}
                value={subjectPrefix}
                placeholder="[Acme] "
                onChange={(e) => setSubjectPrefix(e.target.value)}
              />
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Prepended to every subject line. Include a trailing space if needed.
              </p>
            </div>

            <div className="form-group">
              <label>Header link URL</label>
              <input
                type="url"
                maxLength={512}
                value={headerLink}
                placeholder="https://yourapp.example.com"
                onChange={(e) => setHeaderLink(e.target.value)}
              />
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Wraps your logo / app name in the email header with this link.
              </p>
            </div>

            <div className="form-group">
              <label>Custom footer message</label>
              <textarea
                maxLength={500}
                rows={3}
                value={footerText}
                placeholder="This email was sent by TenzoShare. If you did not expect this email, you can safely ignore it."
                onChange={(e) => setFooterText(e.target.value)}
                style={{ resize: 'vertical' }}
              />
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Replaces the default footer message. Leave blank to use the default.
              </p>
            </div>

            <div style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 6, padding: '10px 14px', fontSize: 13, color: 'var(--color-text-muted)' }}>
              ℹ️ A "Powered by TenzoShare" footnote is shown automatically when your app name is not "TenzoShare".
            </div>
          </div>
        )}

        {/* ── Tab: Colors ───────────────────────────────────────────────── */}
        {activeTab === 'colors' && (
          <div className="card" style={{ padding: 28, marginBottom: 24 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
              <h2 style={{ margin: 0, fontSize: 17 }}>Email Colors</h2>
              <button
                type="button"
                onClick={() => {
                  setButtonColor('');
                  setButtonTextColor('');
                  setBodyBgColor('');
                  setCardBgColor('');
                  setCardBorderColor('');
                  setHeadingColor('');
                  setEmailTextColor('');
                }}
                style={{ fontSize: 13, color: 'var(--color-text-muted)', background: 'none', border: '1px solid var(--color-border)', borderRadius: 6, padding: '4px 12px', cursor: 'pointer' }}
              >
                Reset all to defaults
              </button>
            </div>

            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 24 }}>
              Leave any field blank to use the built-in default. The CTA button color falls back to your primary brand color.
            </p>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 24 }}>
              <ColorPicker
                label="CTA Button Color"
                description="Background of all action buttons"
                value={buttonColor}
                placeholder={primaryColor}
                onChange={setButtonColor}
              />
              <ColorPicker
                label="Button Text Color"
                description="Text on CTA buttons"
                value={buttonTextColor}
                placeholder="#ffffff"
                onChange={setButtonTextColor}
              />
              <ColorPicker
                label="Email Background"
                description="Outer wrapper / body background"
                value={bodyBgColor}
                placeholder="#f1f5f9"
                onChange={setBodyBgColor}
              />
              <ColorPicker
                label="Card Background"
                description="Info and detail card fill"
                value={cardBgColor}
                placeholder="#f8fafc"
                onChange={setCardBgColor}
              />
              <ColorPicker
                label="Card Border"
                description="Info card borders and dividers"
                value={cardBorderColor}
                placeholder="#e2e8f0"
                onChange={setCardBorderColor}
              />
              <ColorPicker
                label="Heading Color"
                description="Section headings and titles"
                value={headingColor}
                placeholder="#1e293b"
                onChange={setHeadingColor}
              />
              <ColorPicker
                label="Body Text Color"
                description="Paragraph and label text"
                value={emailTextColor}
                placeholder="#475569"
                onChange={setEmailTextColor}
              />
            </div>
          </div>
        )}

        {/* ── Tab: Per Email Type ───────────────────────────────────────── */}
        {activeTab === 'per-type' && (
          <div style={{ marginBottom: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 4 }}>
              Override the subject line and call-to-action button text for each email type.
              Leave blank to use the built-in default. Placeholders:{' '}
              <code>{'{{AppName}}'}</code>, <code>{'{{Title}}'}</code>, <code>{'{{RequestName}}'}</code>
            </p>

            {EMAIL_TYPES.map((t) => (
              <div key={t.key} className="card" style={{ padding: 22 }}>
                <h3 style={{ margin: '0 0 4px', fontSize: 15 }}>{t.label}</h3>
                <p className="text-sm" style={{ color: 'var(--color-text-muted)', margin: '0 0 16px', fontSize: 12 }}>
                  Default subject: <em>{t.defaultSubject}</em>
                </p>

                <div style={{ display: 'grid', gridTemplateColumns: t.ctaField ? '1fr 1fr' : '1fr', gap: 16 }}>
                  <div className="form-group" style={{ margin: 0 }}>
                    <label style={{ fontSize: 13 }}>Subject template</label>
                    <input
                      type="text"
                      maxLength={200}
                      value={perTypeSubjects[t.key] ?? ''}
                      placeholder={t.defaultSubject}
                      onChange={(e) => setPerTypeSubjects((prev) => ({ ...prev, [t.key]: e.target.value }))}
                    />
                  </div>

                  {t.ctaField && (
                    <div className="form-group" style={{ margin: 0 }}>
                      <label style={{ fontSize: 13 }}>CTA button text</label>
                      <input
                        type="text"
                        maxLength={80}
                        value={perTypeCTAs[t.key] ?? ''}
                        placeholder={t.defaultCTA}
                        onChange={(e) => setPerTypeCTAs((prev) => ({ ...prev, [t.key]: e.target.value }))}
                      />
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </form>
    </div>
  );
}
