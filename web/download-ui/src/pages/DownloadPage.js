import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import { useState, useEffect, useCallback } from 'react';
import { fetchTransfer, fetchDownloadUrl, TransferApiError } from '../api/transfer';
// ─── Icon components ───────────────────────────────────────────────────────
function IconBolt() {
    return (_jsx("svg", { width: "18", height: "18", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2.5", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M13 2L3 14h9l-1 8 10-12h-9l1-8z" }) }));
}
function IconFile() {
    return (_jsxs("svg", { width: "16", height: "16", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("path", { d: "M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" }), _jsx("polyline", { points: "14 2 14 8 20 8" })] }));
}
function IconDownload() {
    return (_jsxs("svg", { width: "14", height: "14", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2.5", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("path", { d: "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" }), _jsx("polyline", { points: "7 10 12 15 17 10" }), _jsx("line", { x1: "12", y1: "15", x2: "12", y2: "3" })] }));
}
function IconLock() {
    return (_jsxs("svg", { width: "22", height: "22", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("rect", { x: "3", y: "11", width: "18", height: "11", rx: "2", ry: "2" }), _jsx("path", { d: "M7 11V7a5 5 0 0 1 10 0v4" })] }));
}
function IconAlert() {
    return (_jsxs("svg", { width: "22", height: "22", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("circle", { cx: "12", cy: "12", r: "10" }), _jsx("line", { x1: "12", y1: "8", x2: "12", y2: "12" }), _jsx("line", { x1: "12", y1: "16", x2: "12.01", y2: "16" })] }));
}
function IconShield() {
    return (_jsx("svg", { width: "12", height: "12", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" }) }));
}
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
    const [passwordError, setPasswordError] = useState('');
    const [downloading, setDownloading] = useState({});
    const [downloadErrors, setDownloadErrors] = useState({});
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
        setPasswordError('');
        setView({ kind: 'loading' });
        fetchTransfer(view.slug, passwordInput)
            .then((t) => setView({ kind: 'ready', transfer: t, password: passwordInput }))
            .catch((err) => {
            const message = err instanceof TransferApiError ? err.message : 'Incorrect password.';
            setView({ kind: 'password', slug: view.slug });
            setPasswordError(message);
        });
    }, [view, passwordInput]);
    // Trigger download for a single file.
    const handleDownload = useCallback(async (fileId) => {
        if (!slug)
            return;
        const password = view.kind === 'ready' ? view.password : undefined;
        setDownloading((d) => ({ ...d, [fileId]: true }));
        setDownloadErrors((d) => { const n = { ...d }; delete n[fileId]; return n; });
        try {
            const { url } = await fetchDownloadUrl(slug, fileId, password);
            const a = document.createElement('a');
            a.href = url;
            a.download = fileId;
            a.rel = 'noopener noreferrer';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        }
        catch (err) {
            const message = err instanceof TransferApiError ? err.message : 'Download failed.';
            setDownloadErrors((d) => ({ ...d, [fileId]: message }));
        }
        finally {
            setDownloading((d) => ({ ...d, [fileId]: false }));
        }
    }, [slug, view]);
    // ─── Render ────────────────────────────────────────────────────────────────
    if (view.kind === 'loading') {
        return (_jsx(Layout, { children: _jsx("div", { className: "state-center", children: _jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 10, justifyContent: 'center' }, children: [_jsx("span", { className: "spinner" }), _jsx("span", { className: "tenzo-muted", children: "Loading transfer\u2026" })] }) }) }));
    }
    if (view.kind === 'error') {
        return (_jsx(Layout, { children: _jsxs("div", { className: "state-center", children: [_jsx("div", { className: "state-icon state-icon-error", children: _jsx(IconAlert, {}) }), _jsx("h2", { className: "tenzo-title", children: errorTitle(view.status) }), _jsx("p", { className: "tenzo-subtitle", children: view.message })] }) }));
    }
    if (view.kind === 'password') {
        return (_jsxs(Layout, { children: [_jsx("div", { className: "state-icon state-icon-warn", style: { margin: '0 auto 20px' }, children: _jsx(IconLock, {}) }), _jsx("h2", { className: "tenzo-title", style: { textAlign: 'center' }, children: "Password required" }), _jsx("p", { className: "tenzo-subtitle", style: { textAlign: 'center' }, children: "This transfer is password-protected. Enter the password to access the files." }), passwordError && (_jsx("div", { className: "alert alert-error", children: passwordError })), _jsxs("form", { onSubmit: handlePasswordSubmit, children: [_jsxs("div", { className: "form-group", children: [_jsx("label", { htmlFor: "dl-password", children: "Password" }), _jsx("input", { id: "dl-password", type: "password", className: "tenzo-input", placeholder: "Enter password", value: passwordInput, onChange: (e) => setPasswordInput(e.target.value), autoFocus: true, required: true })] }), _jsx("button", { type: "submit", className: "btn btn-primary btn-full btn-lg", style: { marginTop: 8 }, children: "Unlock transfer" })] })] }));
    }
    // view.kind === 'ready'
    const { transfer } = view;
    const downloadsLeft = transfer.max_downloads > 0
        ? Math.max(0, transfer.max_downloads - transfer.download_count)
        : null;
    return (_jsxs(Layout, { children: [transfer.is_revoked && (_jsx("div", { className: "revoked-banner", style: { marginBottom: 20 }, children: "\u26A0\uFE0F This transfer has been revoked by the sender." })), _jsx("h2", { className: "tenzo-title", children: transfer.name || 'Files ready to download' }), transfer.sender_email && (_jsxs("p", { className: "tenzo-subtitle", children: ["Shared by ", _jsx("strong", { children: transfer.sender_email })] })), transfer.description && (_jsx("p", { className: "tenzo-subtitle", children: transfer.description })), _jsxs("div", { className: "chips-row", children: [_jsxs("span", { className: "chip", children: ["Expires ", formatDate(transfer.expires_at)] }), downloadsLeft !== null && (_jsxs("span", { className: `chip ${downloadsLeft > 0 ? 'chip-teal' : ''}`, children: [downloadsLeft, " download", downloadsLeft !== 1 ? 's' : '', " remaining"] })), transfer.file_ids.length > 0 && (_jsxs("span", { className: "chip", children: [transfer.file_ids.length, " file", transfer.file_ids.length !== 1 ? 's' : ''] }))] }), _jsx("hr", { className: "tenzo-divider" }), _jsx("ul", { className: "file-list", children: transfer.file_ids.map((fid) => (_jsxs("li", { className: "file-item", children: [_jsx("div", { className: "file-icon", children: _jsx(IconFile, {}) }), _jsxs("div", { style: { flex: 1, minWidth: 0 }, children: [_jsx("div", { className: "file-name", children: fid }), downloadErrors[fid] && (_jsx("div", { style: { fontSize: 11, color: 'var(--color-error-text)', marginTop: 2 }, children: downloadErrors[fid] }))] }), _jsx("button", { className: "btn btn-primary btn-sm", onClick: () => handleDownload(fid), disabled: !!downloading[fid] || transfer.is_revoked || downloadsLeft === 0, children: downloading[fid] ? (_jsxs(_Fragment, { children: [_jsx("span", { className: "spinner", style: { width: 12, height: 12, borderWidth: 1.5 } }), "Preparing\u2026"] })) : (_jsxs(_Fragment, { children: [_jsx(IconDownload, {}), "Download"] })) })] }, fid))) }), _jsx(TenzoFooter, {})] }));
}
// ─── Layout ────────────────────────────────────────────────────────────────
function Layout({ children }) {
    return (_jsx("div", { className: "tenzo-page", children: _jsxs("div", { className: "tenzo-card", children: [_jsxs("div", { className: "tenzo-brand", children: [_jsx("div", { className: "tenzo-brand-icon", children: _jsx(IconBolt, {}) }), _jsx("span", { className: "tenzo-brand-name", children: "TenzoShare" })] }), children] }) }));
}
// ─── Footer ────────────────────────────────────────────────────────────────
function TenzoFooter() {
    return (_jsxs("div", { className: "tenzo-footer", children: [_jsx(IconShield, {}), "Files are encrypted and served securely via TenzoShare"] }));
}
// ─── Helpers ───────────────────────────────────────────────────────────────
function errorTitle(status) {
    if (status === 404)
        return 'Transfer not found';
    if (status === 403)
        return 'Access denied';
    if (status === 410)
        return 'Transfer expired';
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
