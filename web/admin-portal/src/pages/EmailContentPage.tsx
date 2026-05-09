import React, { useEffect, useRef, useState } from 'react';
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

// ── Live preview helpers ─────────────────────────────────────────────────────

/** Minimal HTML escaping for user-controlled values injected into the preview. */
function esc(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

interface PreviewVars {
  appName: string;
  primaryColor: string;
  buttonColor: string;
  buttonTextColor: string;
  bodyBgColor: string;
  cardBgColor: string;
  cardBorderColor: string;
  headingColor: string;
  textColor: string;
  headerLink: string;
  footerText: string;
  supportEmail: string;
  ctaText: string;
}

function buildContent(type: string, v: PreviewVars): string {
  const a = esc(v.appName);
  const cta = esc(v.ctaText);
  const btn = `display:inline-block;background-color:${v.buttonColor};color:${v.buttonTextColor};text-decoration:none;font-size:15px;font-weight:700;padding:13px 32px;border-radius:6px;font-family:Arial,Helvetica,sans-serif;`;

  switch (type) {
    case 'transfer_received':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">You have received files</p>
        <p style="margin:0 0 24px;font-size:15px;color:${v.textColor};line-height:1.7;font-family:Arial,Helvetica,sans-serif;">
          <strong>Alex Johnson</strong> has shared files with you via <strong>${a}</strong>.
        </p>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
          <tr><td style="background-color:${v.cardBgColor};border:1px solid ${v.cardBorderColor};border-radius:8px;padding:20px 24px;">
            <p style="margin:0 0 8px;font-size:13px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:0.5px;font-family:Arial,Helvetica,sans-serif;">Transfer</p>
            <p style="margin:0 0 16px;font-size:18px;font-weight:700;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">Q1 2026 Reports</p>
            <p style="margin:0 0 16px;font-size:14px;color:${v.textColor};line-height:1.6;border-left:3px solid ${v.cardBorderColor};padding-left:12px;font-family:Arial,Helvetica,sans-serif;">Please review before our Monday meeting.</p>
            <table width="100%" cellpadding="0" cellspacing="0">
              <tr>
                <td style="padding:4px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;width:90px;">Expires</td>
                <td style="padding:4px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">Dec 31, 2026</td>
              </tr>
              <tr>
                <td style="padding:4px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">Access</td>
                <td style="padding:4px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">Public link</td>
              </tr>
            </table>
          </td></tr>
        </table>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td align="center" style="padding:4px 0 24px;">
            <a href="#" style="${btn}">${cta}</a>
          </td></tr>
        </table>`;

    case 'password_reset':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">Reset your password</p>
        <p style="margin:0 0 24px;font-size:15px;color:${v.textColor};line-height:1.7;font-family:Arial,Helvetica,sans-serif;">
          We received a request to reset the password for your <strong>${a}</strong> account. Click the button below to choose a new password.
        </p>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td align="center" style="padding:4px 0 28px;">
            <a href="#" style="${btn}">${cta}</a>
          </td></tr>
        </table>
        <p style="margin:0 0 8px;font-size:13px;color:#94a3b8;font-family:Arial,Helvetica,sans-serif;">Button not working? Copy and paste this link:</p>
        <p style="margin:0 0 24px;font-size:13px;word-break:break-all;font-family:Arial,Helvetica,sans-serif;">
          <a href="#" style="color:#64748b;text-decoration:underline;">https://app.example.com/reset-password?token=preview</a>
        </p>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td style="background-color:#fef9ee;border:1px solid #fcd34d;border-radius:6px;padding:14px 16px;">
            <p style="margin:0;font-size:13px;color:#92400e;line-height:1.6;font-family:Arial,Helvetica,sans-serif;">
              ⚠️ This link expires in <strong>1 hour</strong>. If you did not request a password reset, please ignore this email.
            </p>
          </td></tr>
        </table>`;

    case 'email_verification':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">Verify your email address</p>
        <p style="margin:0 0 24px;font-size:15px;color:${v.textColor};line-height:1.7;font-family:Arial,Helvetica,sans-serif;">
          Thank you for signing up for <strong>${a}</strong>. Please verify your email address to activate your account.
        </p>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td align="center" style="padding:4px 0 28px;">
            <a href="#" style="${btn}">${cta}</a>
          </td></tr>
        </table>
        <p style="margin:0 0 8px;font-size:13px;color:#94a3b8;font-family:Arial,Helvetica,sans-serif;">Button not working? Copy and paste this link:</p>
        <p style="margin:0 0 24px;font-size:13px;word-break:break-all;font-family:Arial,Helvetica,sans-serif;">
          <a href="#" style="color:#64748b;text-decoration:underline;">https://app.example.com/verify?token=preview</a>
        </p>
        <p style="margin:0;font-size:13px;color:#94a3b8;line-height:1.6;font-family:Arial,Helvetica,sans-serif;">
          This link expires in <strong>24 hours</strong>. If you did not create an account, no action is needed.
        </p>`;

    case 'download_notification':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">Your transfer was downloaded</p>
        <p style="margin:0 0 24px;font-size:15px;color:${v.textColor};line-height:1.7;font-family:Arial,Helvetica,sans-serif;">
          Someone has downloaded files from your transfer on <strong>${a}</strong>.
        </p>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:28px;">
          <tr><td style="background-color:${v.cardBgColor};border:1px solid ${v.cardBorderColor};border-radius:8px;padding:20px 24px;">
            <table width="100%" cellpadding="0" cellspacing="0">
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;width:110px;">Transfer</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">Q1 2026 Reports</td>
              </tr>
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">Downloaded by</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">client@example.com</td>
              </tr>
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">Downloaded at</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">May 9, 2026 at 14:32 UTC</td>
              </tr>
            </table>
          </td></tr>
        </table>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td align="center">
            <a href="#" style="display:inline-block;background-color:${v.buttonColor};color:${v.buttonTextColor};text-decoration:none;font-size:14px;font-weight:700;padding:11px 28px;border-radius:6px;font-family:Arial,Helvetica,sans-serif;">${cta}</a>
          </td></tr>
        </table>`;

    case 'transfer_expiry_reminder':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">Your transfer expires soon</p>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
          <tr><td style="background-color:#fff7ed;border:1px solid #fdba74;border-radius:8px;padding:16px 20px;">
            <p style="margin:0;font-size:14px;color:#9a3412;line-height:1.6;font-family:Arial,Helvetica,sans-serif;">
              ⏱ This transfer will expire in less than <strong>24 hours</strong>. Download your files before the link becomes inactive.
            </p>
          </td></tr>
        </table>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:28px;">
          <tr><td style="background-color:${v.cardBgColor};border:1px solid ${v.cardBorderColor};border-radius:8px;padding:20px 24px;">
            <table width="100%" cellpadding="0" cellspacing="0">
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;width:90px;">Transfer</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">Q1 2026 Reports</td>
              </tr>
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">Expires</td>
                <td style="padding:5px 0;font-size:13px;color:#dc2626;font-weight:600;font-family:Arial,Helvetica,sans-serif;">May 10, 2026 at 09:00 UTC</td>
              </tr>
            </table>
          </td></tr>
        </table>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td align="center">
            <a href="#" style="${btn}">${cta}</a>
          </td></tr>
        </table>`;

    case 'transfer_revoked':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">Transfer no longer available</p>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
          <tr><td style="background-color:#fef2f2;border:1px solid #fca5a5;border-radius:8px;padding:16px 20px;">
            <p style="margin:0;font-size:14px;color:#991b1b;line-height:1.6;font-family:Arial,Helvetica,sans-serif;">
              🚫 The following transfer has been revoked by the sender and is <strong>no longer accessible</strong>.
            </p>
          </td></tr>
        </table>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:28px;">
          <tr><td style="background-color:${v.cardBgColor};border:1px solid ${v.cardBorderColor};border-radius:8px;padding:20px 24px;">
            <table width="100%" cellpadding="0" cellspacing="0">
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;width:90px;">Transfer</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">Q1 2026 Reports</td>
              </tr>
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">Sender</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">alex@example.com</td>
              </tr>
            </table>
          </td></tr>
        </table>
        <p style="margin:0;font-size:14px;color:${v.textColor};line-height:1.7;font-family:Arial,Helvetica,sans-serif;">
          If you still need these files, please contact the sender directly.
        </p>`;

    case 'request_submission':
      return `
        <p style="margin:0 0 20px;font-size:16px;font-weight:600;color:${v.headingColor};font-family:Arial,Helvetica,sans-serif;">New file submission</p>
        <p style="margin:0 0 24px;font-size:15px;color:${v.textColor};line-height:1.7;font-family:Arial,Helvetica,sans-serif;">
          Someone has uploaded a file to your file request on <strong>${a}</strong>.
        </p>
        <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:28px;">
          <tr><td style="background-color:${v.cardBgColor};border:1px solid ${v.cardBorderColor};border-radius:8px;padding:20px 24px;">
            <table width="100%" cellpadding="0" cellspacing="0">
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;width:110px;">Request</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">Marketing Assets Q2</td>
              </tr>
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">File</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">campaign-brief.pdf</td>
              </tr>
              <tr>
                <td style="padding:5px 0;font-size:13px;color:#64748b;font-family:Arial,Helvetica,sans-serif;">Submitted by</td>
                <td style="padding:5px 0;font-size:13px;color:${v.headingColor};font-weight:600;font-family:Arial,Helvetica,sans-serif;">designer@agency.com</td>
              </tr>
            </table>
          </td></tr>
        </table>
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr><td align="center">
            <a href="#" style="${btn}">${cta}</a>
          </td></tr>
        </table>`;

    default:
      return '<p style="font-family:Arial,sans-serif;padding:8px;">Unknown email type.</p>';
  }
}

function buildPreviewHTML(type: string, v: PreviewVars): string {
  const a = esc(v.appName);
  const headerInner = `<span style="color:#ffffff;font-size:24px;font-weight:700;letter-spacing:0.4px;font-family:Arial,Helvetica,sans-serif;">${a}</span>`;
  const header = v.headerLink
    ? `<a href="${esc(v.headerLink)}" style="text-decoration:none;">${headerInner}</a>`
    : headerInner;
  const footerBody = v.footerText
    ? `<p style="margin:0;font-size:12px;color:#94a3b8;line-height:1.6;font-family:Arial,Helvetica,sans-serif;">${esc(v.footerText)}</p>`
    : `<p style="margin:0;font-size:12px;color:#94a3b8;line-height:1.6;font-family:Arial,Helvetica,sans-serif;">This email was sent by <strong style="color:#64748b;">${a}</strong>.<br>If you did not expect this email, you can safely ignore it.</p>`;
  const supportLine = v.supportEmail
    ? `<p style="margin:6px 0 0;font-size:12px;color:#94a3b8;font-family:Arial,Helvetica,sans-serif;">Questions? <a href="mailto:${esc(v.supportEmail)}" style="color:#64748b;text-decoration:underline;">${esc(v.supportEmail)}</a></p>`
    : '';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1.0">
  <title>${a}</title>
</head>
<body style="margin:0;padding:0;background-color:${v.bodyBgColor};font-family:Arial,Helvetica,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background-color:${v.bodyBgColor};padding:32px 16px;">
    <tr><td align="center" valign="top">
      <table cellpadding="0" cellspacing="0" style="width:100%;max-width:540px;">
        <tr>
          <td style="background-color:${v.primaryColor};border-radius:10px 10px 0 0;padding:24px 28px;text-align:center;">
            ${header}
          </td>
        </tr>
        <tr>
          <td style="background-color:#ffffff;padding:32px 28px;border-left:1px solid #e2e8f0;border-right:1px solid #e2e8f0;">
            ${buildContent(type, v)}
          </td>
        </tr>
        <tr>
          <td style="background-color:${v.cardBgColor};border:1px solid ${v.cardBorderColor};border-top:none;border-radius:0 0 10px 10px;padding:18px 28px;text-align:center;">
            ${footerBody}
            ${supportLine}
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`;
}

// ── EmailPreviewPanel ─────────────────────────────────────────────────────────

function EmailPreviewPanel({
  previewType,
  onPreviewTypeChange,
  html,
}: {
  previewType: string;
  onPreviewTypeChange: (t: string) => void;
  html: string;
}) {
  return (
    <div style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 10, overflow: 'hidden', boxShadow: '0 2px 12px rgba(0,0,0,0.10)' }}>
      {/* Fake browser chrome */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px', borderBottom: '1px solid var(--color-border)', background: 'var(--color-surface)' }}>
        <div style={{ display: 'flex', gap: 5, flexShrink: 0 }}>
          <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#ff5f57', display: 'inline-block' }} />
          <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#febc2e', display: 'inline-block' }} />
          <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#28c840', display: 'inline-block' }} />
        </div>
        <span style={{ flex: 1, fontSize: 12, fontWeight: 600, color: 'var(--color-text-muted)', textAlign: 'center' }}>Live Preview</span>
        <select
          value={previewType}
          onChange={(e) => onPreviewTypeChange(e.target.value)}
          style={{ fontSize: 12, padding: '3px 8px', borderRadius: 6, border: '1px solid var(--color-border)', background: 'var(--color-surface)', color: 'var(--color-text)', cursor: 'pointer' }}
        >
          {EMAIL_TYPES.map((t) => (
            <option key={t.key} value={t.key}>{t.label}</option>
          ))}
        </select>
      </div>
      <iframe
        title="Email preview"
        srcDoc={html}
        sandbox="allow-same-origin"
        onLoad={(e) => {
          // Auto-size to content height to avoid dead whitespace
          const iframe = e.currentTarget;
          try {
            const h = iframe.contentDocument?.documentElement?.scrollHeight;
            if (h && h > 100) iframe.style.height = h + 'px';
          } catch { /* sandboxed — falls back to default height */ }
        }}
        style={{ width: '100%', height: 620, border: 'none', display: 'block', transition: 'height 0.15s' }}
      />
    </div>
  );
}

// ── Tab type ──────────────────────────────────────────────────────────────────

type Tab = 'delivery' | 'colors' | 'per-type';

// ── Page ─────────────────────────────────────────────────────────────────────

export default function EmailContentPage() {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('delivery');

  // --- App identity (for preview) ---
  const [appName, setAppName] = useState('TenzoShare');

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

  // --- Preview ---
  const [previewType, setPreviewType] = useState('transfer_received');
  const [previewHtml, setPreviewHtml] = useState('');
  const previewTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    setLoading(true);
    getBranding()
      .then((c) => {
        setAppName(c.app_name ?? 'TenzoShare');
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

  // Debounced live preview: regenerates HTML 150 ms after any relevant state change
  useEffect(() => {
    if (previewTimerRef.current) clearTimeout(previewTimerRef.current);
    previewTimerRef.current = setTimeout(() => {
      const typeMeta = EMAIL_TYPES.find((t) => t.key === previewType);
      const resolvedCTA = perTypeCTAs[previewType] || typeMeta?.defaultCTA || 'View Details';
      const vars: PreviewVars = {
        appName: appName || 'TenzoShare',
        primaryColor: primaryColor || '#1e293b',
        buttonColor: buttonColor || primaryColor || '#1e293b',
        buttonTextColor: buttonTextColor || '#ffffff',
        bodyBgColor: bodyBgColor || '#f1f5f9',
        cardBgColor: cardBgColor || '#f8fafc',
        cardBorderColor: cardBorderColor || '#e2e8f0',
        headingColor: headingColor || '#1e293b',
        textColor: emailTextColor || '#475569',
        headerLink,
        footerText,
        supportEmail,
        ctaText: resolvedCTA,
      };
      try {
        setPreviewHtml(buildPreviewHTML(previewType, vars));
      } catch (err) {
        // Failsafe: never let a preview error crash the page
        setPreviewHtml('<p style="color:red;font-family:sans-serif;padding:20px">Preview render error.</p>');
        console.error('Email preview error:', err);
      }
    }, 150);
    return () => {
      if (previewTimerRef.current) clearTimeout(previewTimerRef.current);
    };
  }, [
    previewType, appName, primaryColor,
    buttonColor, buttonTextColor,
    bodyBgColor, cardBgColor, cardBorderColor,
    headingColor, emailTextColor,
    headerLink, footerText, supportEmail, perTypeCTAs,
  ]);

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
    <div style={{ padding: 32 }}>
      <div style={{ display: 'flex', gap: 32, alignItems: 'flex-start' }}>
      <div style={{ flex: '0 0 560px', minWidth: 0 }}>
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
      </div>{/* end left panel */}

      {/* ── Right: live preview ──────────────────────────────────────────── */}
      <div style={{ flex: 1, minWidth: 380, position: 'sticky', top: 24, alignSelf: 'flex-start' }}>
        <EmailPreviewPanel
          previewType={previewType}
          onPreviewTypeChange={setPreviewType}
          html={previewHtml}
        />
      </div>
      </div>{/* end two-pane */}
    </div>
  );
}
