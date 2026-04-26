import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import { useState, useEffect, useCallback, useRef } from 'react';
import { RequestApiError } from '../types';
import { fetchRequest, uploadFile } from '../api/requests';
// ─── Slug resolution ───────────────────────────────────────────────────────
function resolveSlug() {
    const m = window.location.pathname.match(/\/r\/([^/]+)/);
    return m ? m[1] : null;
}
// ─── Helpers ───────────────────────────────────────────────────────────────
function fmtBytes(bytes) {
    if (bytes === 0)
        return '0 B';
    const k = 1024;
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${units[i]}`;
}
function fmtDate(iso) {
    return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}
// ─── Icon components ───────────────────────────────────────────────────────
function IconBolt() {
    return (_jsx("svg", { width: "18", height: "18", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2.5", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M13 2L3 14h9l-1 8 10-12h-9l1-8z" }) }));
}
function IconUpload() {
    return (_jsxs("svg", { width: "20", height: "20", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("polyline", { points: "16 16 12 12 8 16" }), _jsx("line", { x1: "12", y1: "12", x2: "12", y2: "21" }), _jsx("path", { d: "M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3" })] }));
}
function IconFile() {
    return (_jsxs("svg", { width: "14", height: "14", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("path", { d: "M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" }), _jsx("polyline", { points: "14 2 14 8 20 8" })] }));
}
function IconCheck() {
    return (_jsxs("svg", { width: "40", height: "40", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("path", { d: "M22 11.08V12a10 10 0 1 1-5.93-9.14" }), _jsx("polyline", { points: "22 4 12 14.01 9 11.01" })] }));
}
function IconClock() {
    return (_jsxs("svg", { width: "22", height: "22", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("circle", { cx: "12", cy: "12", r: "10" }), _jsx("polyline", { points: "12 6 12 12 16 14" })] }));
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
function IconClose() {
    return (_jsxs("svg", { width: "14", height: "14", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2.5", strokeLinecap: "round", strokeLinejoin: "round", children: [_jsx("line", { x1: "18", y1: "6", x2: "6", y2: "18" }), _jsx("line", { x1: "6", y1: "6", x2: "18", y2: "18" })] }));
}
// ─── Layout ────────────────────────────────────────────────────────────────
function Layout({ children }) {
    return (_jsx("div", { className: "tenzo-page", children: _jsxs("div", { className: "tenzo-card", children: [_jsxs("div", { className: "tenzo-brand", children: [_jsx("div", { className: "tenzo-brand-icon", children: _jsx(IconBolt, {}) }), _jsx("span", { className: "tenzo-brand-name", children: "TenzoShare" })] }), children] }) }));
}
// ─── Footer ────────────────────────────────────────────────────────────────
function TenzoFooter() {
    return (_jsxs("div", { className: "tenzo-footer", children: [_jsx(IconShield, {}), "Files are encrypted and delivered securely via TenzoShare"] }));
}
// ─── Main component ────────────────────────────────────────────────────────
export default function RequestPage() {
    const slug = resolveSlug();
    const [view, setView] = useState({ kind: 'loading' });
    const [files, setFiles] = useState([]);
    const [submitterName, setSubmitterName] = useState('');
    const [message, setMessage] = useState('');
    const [submitting, setSubmitting] = useState(false);
    const [dragOver, setDragOver] = useState(false);
    const fileInputRef = useRef(null);
    useEffect(() => {
        if (!slug) {
            setView({ kind: 'error', message: 'No request link found. Please check the URL.' });
            return;
        }
        fetchRequest(slug)
            .then((req) => {
            if (req.is_expired || !req.is_active) {
                setView({ kind: 'closed', reason: req.is_active ? 'expired' : 'inactive' });
            }
            else {
                setView({ kind: 'open', request: req });
            }
        })
            .catch((err) => {
            const msg = err instanceof RequestApiError ? err.message : 'Failed to load request.';
            const status = err instanceof RequestApiError ? err.status : undefined;
            setView({ kind: 'error', message: msg, status });
        });
    }, [slug]);
    const addFiles = useCallback((incoming) => {
        const arr = Array.from(incoming);
        const entries = arr.map((f) => ({
            id: `${f.name}-${f.size}-${Date.now()}-${Math.random()}`,
            file: f,
            status: 'pending',
            progress: 0,
        }));
        setFiles((prev) => [...prev, ...entries]);
    }, []);
    const handleDrop = useCallback((e) => {
        e.preventDefault();
        setDragOver(false);
        if (e.dataTransfer.files.length > 0)
            addFiles(e.dataTransfer.files);
    }, [addFiles]);
    const handleDragOver = (e) => { e.preventDefault(); setDragOver(true); };
    const handleDragLeave = () => setDragOver(false);
    const handleFileInput = (e) => {
        if (e.target.files && e.target.files.length > 0)
            addFiles(e.target.files);
        e.target.value = '';
    };
    const removeFile = (id) => {
        setFiles((prev) => prev.filter((f) => f.id !== id));
    };
    const handleSubmit = async () => {
        if (!slug || files.length === 0)
            return;
        setSubmitting(true);
        const pending = files.filter((f) => f.status === 'pending' || f.status === 'error');
        let allOk = true;
        for (const entry of pending) {
            setFiles((prev) => prev.map((f) => (f.id === entry.id ? { ...f, status: 'uploading', progress: 0 } : f)));
            try {
                await uploadFile(slug, entry.file, submitterName, message, (pct) => {
                    setFiles((prev) => prev.map((f) => (f.id === entry.id ? { ...f, progress: pct } : f)));
                });
                setFiles((prev) => prev.map((f) => (f.id === entry.id ? { ...f, status: 'done', progress: 100 } : f)));
            }
            catch (err) {
                const errMsg = err instanceof RequestApiError ? err.message : 'Upload failed';
                setFiles((prev) => prev.map((f) => (f.id === entry.id ? { ...f, status: 'error', error: errMsg } : f)));
                allOk = false;
            }
        }
        setSubmitting(false);
        if (allOk)
            setView({ kind: 'success' });
    };
    // ─── Render ──────────────────────────────────────────────────────────────
    if (view.kind === 'loading') {
        return (_jsx(Layout, { children: _jsx("div", { className: "state-center", children: _jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 10, justifyContent: 'center' }, children: [_jsx("span", { className: "spinner" }), _jsx("span", { className: "tenzo-muted", children: "Loading request\u2026" })] }) }) }));
    }
    if (view.kind === 'error') {
        return (_jsx(Layout, { children: _jsxs("div", { className: "state-center", children: [_jsx("div", { className: "state-icon state-icon-error", children: _jsx(IconAlert, {}) }), _jsx("h2", { className: "tenzo-title", children: view.status === 404 ? 'Request not found' : 'Something went wrong' }), _jsx("p", { className: "tenzo-subtitle", children: view.message })] }) }));
    }
    if (view.kind === 'closed') {
        const isExpired = view.reason === 'expired';
        return (_jsxs(Layout, { children: [_jsxs("div", { className: "state-center", children: [_jsx("div", { className: `state-icon ${isExpired ? 'state-icon-warn' : 'state-icon-error'}`, children: isExpired ? _jsx(IconClock, {}) : _jsx(IconLock, {}) }), _jsx("h2", { className: "tenzo-title", children: isExpired ? 'This request has expired' : 'This request is closed' }), _jsx("p", { className: "tenzo-subtitle", children: isExpired
                                ? 'The upload deadline has passed. Please contact the requester for a new link.'
                                : 'This request has been closed by the requester.' })] }), _jsx(TenzoFooter, {})] }));
    }
    if (view.kind === 'success') {
        return (_jsxs(Layout, { children: [_jsxs("div", { className: "state-center", children: [_jsx("div", { className: "state-icon state-icon-teal", children: _jsx(IconCheck, {}) }), _jsx("h2", { className: "tenzo-title", children: "Files submitted!" }), _jsx("p", { className: "tenzo-subtitle", children: "Your files have been uploaded successfully. The requester has been notified." })] }), _jsx(TenzoFooter, {})] }));
    }
    const req = view.request;
    const hasFiles = files.length > 0;
    const allDone = hasFiles && files.every((f) => f.status === 'done');
    const isUploading = files.some((f) => f.status === 'uploading');
    const pendingCount = files.filter((f) => f.status !== 'done').length;
    const canSubmit = hasFiles && !submitting && !allDone && !isUploading;
    return (_jsxs(Layout, { children: [_jsx("h2", { className: "tenzo-title", children: req.name }), req.description && (_jsx("p", { className: "tenzo-subtitle", children: req.description })), _jsxs("div", { className: "chips-row", children: [_jsxs("span", { className: "chip", children: ["Expires ", fmtDate(req.expires_at)] }), req.max_size_mb > 0 && (_jsxs("span", { className: "chip", children: ["Max ", req.max_size_mb, " MB per file"] })), req.max_files > 0 && (_jsxs("span", { className: "chip", children: ["Up to ", req.max_files, " file", req.max_files !== 1 ? 's' : ''] })), req.allowed_types && (_jsx("span", { className: "chip", children: req.allowed_types }))] }), _jsxs("div", { className: `drop-zone${dragOver ? ' active' : ''}`, onDrop: handleDrop, onDragOver: handleDragOver, onDragLeave: handleDragLeave, onClick: () => fileInputRef.current?.click(), role: "button", tabIndex: 0, onKeyDown: (e) => e.key === 'Enter' && fileInputRef.current?.click(), "aria-label": "Upload files", children: [_jsx("div", { className: "drop-zone-icon", children: _jsx(IconUpload, {}) }), _jsx("p", { className: "drop-zone-text", children: dragOver ? 'Drop files here' : 'Drag & drop files here, or click to browse' }), _jsx("p", { className: "drop-zone-hint", children: req.max_size_mb > 0 ? `Up to ${req.max_size_mb} MB per file` : 'No file size limit' })] }), _jsx("input", { ref: fileInputRef, type: "file", multiple: true, style: { display: 'none' }, onChange: handleFileInput, "aria-hidden": "true" }), files.length > 0 && (_jsx("ul", { className: "file-list", children: files.map((entry) => (_jsxs("li", { className: "file-item", children: [_jsxs("div", { className: "file-item-row", children: [_jsx("div", { className: "file-icon", children: _jsx(IconFile, {}) }), _jsx("span", { className: "file-name", children: entry.file.name }), _jsx("span", { className: "file-size", children: fmtBytes(entry.file.size) }), entry.status === 'done' && (_jsx("span", { className: "file-status-done", "aria-label": "Uploaded", children: "\u2713" })), entry.status === 'uploading' && (_jsx("span", { className: "spinner", style: { width: 14, height: 14, borderWidth: 1.5 } })), entry.status === 'pending' && (_jsx("button", { className: "remove-btn", onClick: () => removeFile(entry.id), title: "Remove file", type: "button", "aria-label": `Remove ${entry.file.name}`, children: _jsx(IconClose, {}) }))] }), entry.status === 'error' && entry.error && (_jsx("div", { className: "file-status-error", children: entry.error })), (entry.status === 'uploading' || entry.status === 'done' || entry.status === 'error') && (_jsx("div", { className: "progress-bar-wrap", children: _jsx("div", { className: `progress-bar progress-bar-${entry.status}`, style: { width: `${entry.progress}%` }, role: "progressbar", "aria-valuenow": entry.progress, "aria-valuemin": 0, "aria-valuemax": 100 }) }))] }, entry.id))) })), _jsx("hr", { className: "tenzo-divider" }), _jsxs("div", { className: "form-group", children: [_jsxs("label", { htmlFor: "submitter-name", children: ["Your name ", _jsx("span", { className: "form-label-optional", children: "(optional)" })] }), _jsx("input", { id: "submitter-name", type: "text", className: "tenzo-input", value: submitterName, onChange: (e) => setSubmitterName(e.target.value), placeholder: "e.g. Jane Smith", maxLength: 100 })] }), _jsxs("div", { className: "form-group", children: [_jsxs("label", { htmlFor: "req-message", children: ["Message ", _jsx("span", { className: "form-label-optional", children: "(optional)" })] }), _jsx("textarea", { id: "req-message", className: "tenzo-input", value: message, onChange: (e) => setMessage(e.target.value), placeholder: "Add a note for the requester\u2026", maxLength: 500, rows: 3 })] }), _jsx("button", { type: "button", className: "btn btn-primary btn-full btn-lg", style: { marginTop: 8 }, onClick: handleSubmit, disabled: !canSubmit, children: isUploading ? (_jsxs(_Fragment, { children: [_jsx("span", { className: "spinner", style: { width: 14, height: 14, borderWidth: 2, borderColor: 'rgba(255,255,255,0.3)', borderTopColor: '#fff' } }), "Uploading\u2026"] })) : allDone ? ('All files uploaded ✓') : (`Submit ${pendingCount > 0 ? `${pendingCount} file${pendingCount !== 1 ? 's' : ''}` : 'files'}`) }), _jsx(TenzoFooter, {})] }));
}
