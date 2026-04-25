import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import { useState, useEffect, useCallback } from 'react';
import { fetchTransfer, fetchDownloadUrl, TransferApiError } from '../api/transfer';
// ---------------------------------------------------------------------------
// Slug resolution
// ---------------------------------------------------------------------------
/**
 * The share link can be in any of these forms:
 *   https://your-host/t/AbCdEfGh1234
 *   https://your-host/download/AbCdEfGh1234
 *   https://your-host/?slug=AbCdEfGh1234
 *
 * We try each in order so that deployments can choose their own URL structure
 * by routing requests to this SPA.
 */
function resolveSlug() {
    const path = window.location.pathname;
    // /t/<slug> or /download/<slug>
    const pathMatch = path.match(/\/(?:t|download)\/([^/]+)/);
    if (pathMatch)
        return pathMatch[1];
    // ?slug=<slug>
    const param = new URLSearchParams(window.location.search).get('slug');
    if (param)
        return param;
    return null;
}
// ---------------------------------------------------------------------------
// DownloadPage
// ---------------------------------------------------------------------------
export default function DownloadPage() {
    const slug = resolveSlug();
    const [view, setView] = useState({ kind: 'loading' });
    const [passwordInput, setPasswordInput] = useState('');
    const [downloading, setDownloading] = useState({});
    // Initial fetch — no password yet.
    useEffect(() => {
        if (!slug) {
            setView({ kind: 'error', message: 'No transfer link found. Check your URL.' });
            return;
        }
        fetchTransfer(slug)
            .then((t) => setView({ kind: 'ready', transfer: t }))
            .catch((err) => {
            if (err instanceof TransferApiError && err.status === 401) {
                setView({ kind: 'password', slug });
            }
            else if (err instanceof TransferApiError) {
                setView({ kind: 'error', message: err.message, status: err.status });
            }
            else {
                setView({ kind: 'error', message: 'Failed to load transfer.' });
            }
        });
    }, [slug]);
    // Submit password form.
    const handlePasswordSubmit = useCallback((e) => {
        e.preventDefault();
        if (view.kind !== 'password')
            return;
        setView({ kind: 'loading' });
        fetchTransfer(view.slug, passwordInput)
            .then((t) => setView({ kind: 'ready', transfer: t, password: passwordInput }))
            .catch((err) => {
            const message = err instanceof TransferApiError ? err.message : 'Incorrect password.';
            setView({ kind: 'password', slug: view.slug });
            // show inline error without unmounting the form
            alert(message);
        });
    }, [view, passwordInput]);
    // Trigger download for a single file.
    const handleDownload = useCallback(async (fileId, fileName) => {
        if (!slug)
            return;
        const password = view.kind === 'ready' ? view.password : undefined;
        setDownloading((d) => ({ ...d, [fileId]: true }));
        try {
            const { url } = await fetchDownloadUrl(slug, fileId, password);
            // Trigger browser download via a transient <a> — works cross-browser
            // without popups being blocked (it's inside a user-gesture handler).
            const a = document.createElement('a');
            a.href = url;
            a.download = fileName;
            a.rel = 'noopener noreferrer';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        }
        catch (err) {
            const message = err instanceof TransferApiError ? err.message : 'Download failed.';
            alert(message);
        }
        finally {
            setDownloading((d) => ({ ...d, [fileId]: false }));
        }
    }, [slug, view]);
    // ---------------------------------------------------------------------------
    // Render
    // ---------------------------------------------------------------------------
    if (view.kind === 'loading') {
        return _jsx(Layout, { children: _jsx("p", { style: styles.muted, children: "Loading\u2026" }) });
    }
    if (view.kind === 'error') {
        return (_jsx(Layout, { children: _jsxs("div", { style: styles.errorBox, children: [_jsx("strong", { children: errorTitle(view.status) }), _jsx("p", { style: { margin: '8px 0 0' }, children: view.message })] }) }));
    }
    if (view.kind === 'password') {
        return (_jsxs(Layout, { children: [_jsx("h2", { style: styles.heading, children: "Password required" }), _jsx("p", { style: styles.muted, children: "This transfer is password-protected." }), _jsxs("form", { onSubmit: handlePasswordSubmit, style: styles.form, children: [_jsx("input", { type: "password", placeholder: "Enter password", value: passwordInput, onChange: (e) => setPasswordInput(e.target.value), autoFocus: true, required: true, style: styles.input }), _jsx("button", { type: "submit", style: styles.btnPrimary, children: "Unlock" })] })] }));
    }
    // view.kind === 'ready'
    const { transfer } = view;
    return (_jsxs(Layout, { children: [_jsx("h2", { style: styles.heading, children: transfer.name || 'Files ready to download' }), transfer.description && (_jsx("p", { style: { ...styles.muted, marginBottom: 20 }, children: transfer.description })), _jsxs("dl", { style: styles.meta, children: [_jsx("dt", { style: styles.dt, children: "Expires" }), _jsx("dd", { style: styles.dd, children: formatDate(transfer.expires_at) }), transfer.max_downloads > 0 && (_jsxs(_Fragment, { children: [_jsx("dt", { style: styles.dt, children: "Downloads left" }), _jsxs("dd", { style: styles.dd, children: [Math.max(0, transfer.max_downloads - transfer.download_count), ' / ', transfer.max_downloads] })] }))] }), _jsx("ul", { style: styles.fileList, children: transfer.file_ids.map((fid) => (_jsxs("li", { style: styles.fileItem, children: [_jsx("span", { style: styles.fileId, children: fid }), _jsx("button", { onClick: () => handleDownload(fid, fid), disabled: !!downloading[fid], style: downloading[fid] ? styles.btnDisabled : styles.btnPrimary, children: downloading[fid] ? 'Preparing…' : 'Download' })] }, fid))) }), transfer.is_revoked && (_jsx("div", { style: styles.errorBox, children: "This transfer has been revoked by the sender." }))] }));
}
// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------
function Layout({ children }) {
    return (_jsxs("div", { style: styles.page, children: [_jsx("header", { style: styles.header, children: _jsx("span", { style: styles.logo, children: "TenzoShare" }) }), _jsx("main", { style: styles.main, children: children })] }));
}
// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function errorTitle(status) {
    if (status === 404)
        return 'Transfer not found';
    if (status === 403)
        return 'Access denied';
    if (status === 401)
        return 'Authentication required';
    return 'Something went wrong';
}
function formatDate(iso) {
    return new Date(iso).toLocaleString(undefined, {
        dateStyle: 'medium',
        timeStyle: 'short',
    });
}
// ---------------------------------------------------------------------------
// Inline styles — intentionally plain so there are zero build-time deps.
// Replace with any CSS solution you prefer.
// ---------------------------------------------------------------------------
const styles = {
    page: {
        minHeight: '100vh',
        fontFamily: 'system-ui, sans-serif',
        background: '#f5f5f5',
        color: '#111',
    },
    header: {
        background: '#0f172a',
        padding: '12px 24px',
    },
    logo: {
        color: '#fff',
        fontWeight: 700,
        fontSize: '1.1rem',
        letterSpacing: '-0.01em',
    },
    main: {
        maxWidth: 560,
        margin: '48px auto',
        padding: '0 16px',
    },
    heading: {
        fontSize: '1.4rem',
        fontWeight: 600,
        margin: '0 0 16px',
    },
    muted: {
        color: '#666',
        margin: 0,
    },
    meta: {
        display: 'grid',
        gridTemplateColumns: 'max-content 1fr',
        gap: '4px 16px',
        margin: '0 0 24px',
        fontSize: '0.9rem',
    },
    dt: {
        color: '#555',
        fontWeight: 500,
    },
    dd: {
        margin: 0,
    },
    form: {
        display: 'flex',
        gap: 8,
        marginTop: 16,
    },
    input: {
        flex: 1,
        padding: '8px 12px',
        border: '1px solid #ccc',
        borderRadius: 6,
        fontSize: '1rem',
    },
    fileList: {
        listStyle: 'none',
        margin: 0,
        padding: 0,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
    },
    fileItem: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        background: '#fff',
        border: '1px solid #e2e8f0',
        borderRadius: 8,
        padding: '12px 16px',
    },
    fileId: {
        fontFamily: 'monospace',
        fontSize: '0.8rem',
        color: '#444',
        wordBreak: 'break-all',
        flex: 1,
        marginRight: 12,
    },
    btnPrimary: {
        padding: '8px 18px',
        background: '#0f172a',
        color: '#fff',
        border: 'none',
        borderRadius: 6,
        cursor: 'pointer',
        fontWeight: 500,
        fontSize: '0.9rem',
        whiteSpace: 'nowrap',
    },
    btnDisabled: {
        padding: '8px 18px',
        background: '#94a3b8',
        color: '#fff',
        border: 'none',
        borderRadius: 6,
        cursor: 'not-allowed',
        fontWeight: 500,
        fontSize: '0.9rem',
        whiteSpace: 'nowrap',
    },
    errorBox: {
        background: '#fef2f2',
        border: '1px solid #fca5a5',
        borderRadius: 8,
        padding: '16px',
        color: '#991b1b',
        fontSize: '0.95rem',
    },
};
