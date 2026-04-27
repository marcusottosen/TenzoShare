import React, { useState, useEffect, useRef, useCallback, useLayoutEffect } from 'react';
import { listAuditEvents, type AuditEvent, type AuditFilters } from '../api/audit';
import { listUsers, type AdminUser } from '../api/admin';
import { useSortState } from '../hooks/useSort';
import { SortHeader } from '../components/SortHeader';

type AuditSortKey = 'created_at' | 'source' | 'action';

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

const PAGE_SIZE = 50;

const ALL_SOURCES = ['auth', 'transfer', 'storage', 'upload', 'admin'] as const;

// ── Resizable table ──────────────────────────────────────────────────────────

const DEFAULT_COL_WIDTHS = [160, 90, 130, 180, 130, 110, 200];

function ResizableTable({ colWidths, onColWidthChange, children }: {
  colWidths: number[];
  onColWidthChange: (idx: number, w: number) => void;
  children: React.ReactNode;
}) {
  const tableRef = useRef<HTMLTableElement>(null);
  const dragging = useRef<{ idx: number; startX: number; startW: number } | null>(null);

  function onMouseDown(e: React.MouseEvent, idx: number) {
    e.preventDefault();
    dragging.current = { idx, startX: e.clientX, startW: colWidths[idx] };
  }

  useLayoutEffect(() => {
    function onMouseMove(e: MouseEvent) {
      if (!dragging.current) return;
      const { idx, startX, startW } = dragging.current;
      const newW = Math.max(60, startW + (e.clientX - startX));
      onColWidthChange(idx, newW);
    }
    function onMouseUp() { dragging.current = null; }
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, [onColWidthChange]);

  return (
    <table ref={tableRef} style={{ width: '100%', borderCollapse: 'collapse', background: 'var(--color-surface)', tableLayout: 'fixed' }}>
      <colgroup>
        {colWidths.map((w, i) => (
          <col key={i} style={{ width: w }} />
        ))}
      </colgroup>
      {React.Children.map(children, (child) => {
        if (!React.isValidElement(child)) return child;
        if ((child as React.ReactElement<{children?: React.ReactNode}>).type === 'thead') {
          const thead = child as React.ReactElement<{ children: React.ReactNode }>;
          const trChildren = React.Children.toArray(
            (thead.props.children as React.ReactElement<{ children: React.ReactNode }>).props.children
          );
          const newThs = trChildren.map((th, i) => {
            if (!React.isValidElement(th)) return th;
            const thEl = th as React.ReactElement<React.ThHTMLAttributes<HTMLTableCellElement>>;
            return React.cloneElement(thEl, {
              key: i,
              style: { ...thEl.props.style, position: 'relative', overflow: 'hidden' },
              children: (
                <>
                  {thEl.props.children}
                  {i < colWidths.length - 1 && (
                    <span
                      onMouseDown={(e) => onMouseDown(e, i)}
                      style={{
                        position: 'absolute', right: 0, top: 0, bottom: 0, width: 6,
                        cursor: 'col-resize', userSelect: 'none',
                        background: 'transparent',
                        borderRight: '2px solid transparent',
                      }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.borderRightColor = 'var(--color-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.borderRightColor = 'transparent'; }}
                    />
                  )}
                </>
              ),
            });
          });
          const origTr = (thead.props.children as React.ReactElement<{ children: React.ReactNode }>);
          return React.cloneElement(thead, {},
            React.cloneElement(origTr, {}, ...newThs)
          );
        }
        return child;
      })}
    </table>
  );
}

// ── Multi-pick dropdown ──────────────────────────────────────────────────────

interface MultiPickOption { value: string; label: string; }

function MultiPickDropdown({
  label,
  options,
  selected,
  onChange,
  searchable,
  placeholder,
}: {
  label: string;
  options: MultiPickOption[];
  selected: string[];
  onChange: (vals: string[]) => void;
  searchable?: boolean;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handle(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, []);

  const filtered = searchable && search
    ? options.filter((o) => o.label.toLowerCase().includes(search.toLowerCase()))
    : options;

  function toggle(val: string) {
    onChange(selected.includes(val) ? selected.filter((v) => v !== val) : [...selected, val]);
  }

  const displayText = selected.length === 0
    ? (placeholder ?? `All ${label}`)
    : selected.length === 1
    ? (options.find((o) => o.value === selected[0])?.label ?? selected[0])
    : `${selected.length} selected`;

  return (
    <div ref={ref} style={{ position: 'relative', minWidth: 160 }}>
      <button
        type="button"
        className="btn btn-secondary btn-sm"
        style={{ width: '100%', justifyContent: 'space-between', display: 'flex', alignItems: 'center', gap: 6, height: 34, paddingLeft: 10, paddingRight: 10 }}
        onClick={() => setOpen((o) => !o)}
      >
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, textAlign: 'left' }}>{displayText}</span>
        <span style={{ fontSize: 10, opacity: 0.6, flexShrink: 0 }}>{open ? '▲' : '▼'}</span>
      </button>
      {open && (
        <div style={{
          position: 'absolute', top: '100%', left: 0, zIndex: 100,
          background: 'var(--color-surface-raised, var(--color-surface))',
          border: '1px solid var(--color-border-card)',
          borderRadius: 6, marginTop: 4, minWidth: '100%', maxWidth: 280,
          boxShadow: '0 4px 16px rgba(0,0,0,0.3)',
        }}>
          {searchable && (
            <div style={{ padding: '8px 8px 4px' }}>
              <input
                autoFocus
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search…"
                style={{ width: '100%', fontSize: 12, padding: '4px 8px', background: 'var(--color-bg)', border: '1px solid var(--color-border)', borderRadius: 4, color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' }}
              />
            </div>
          )}
          <div style={{ maxHeight: 220, overflowY: 'auto', padding: '4px 0' }}>
            {filtered.length === 0 ? (
              <div style={{ padding: '8px 12px', fontSize: 12, color: 'var(--color-text-muted)' }}>No results</div>
            ) : filtered.map((opt) => (
              <label
                key={opt.value}
                style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 12px', cursor: 'pointer', fontSize: 13, color: 'var(--color-text-primary)' }}
                onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-nav-active)')}
                onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
              >
                <input
                  type="checkbox"
                  checked={selected.includes(opt.value)}
                  onChange={() => toggle(opt.value)}
                  style={{ accentColor: 'var(--color-secondary)', flexShrink: 0 }}
                />
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{opt.label}</span>
              </label>
            ))}
          </div>
          {selected.length > 0 && (
            <div style={{ borderTop: '1px solid var(--color-border)', padding: '6px 8px' }}>
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                style={{ width: '100%', fontSize: 11 }}
                onClick={() => { onChange([]); setOpen(false); }}
              >
                Clear selection
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ── Main page ────────────────────────────────────────────────────────────────

export default function AuditPage() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(0);

  const [filterUserIds, setFilterUserIds] = useState<string[]>([]);
  const [filterSources, setFilterSources] = useState<string[]>([]);
  const [filterAction, setFilterAction] = useState('');
  const [filterStart, setFilterStart] = useState('');
  const [filterEnd, setFilterEnd] = useState('');

  const [users, setUsers] = useState<AdminUser[]>([]);
  const sort = useSortState<AuditSortKey>('created_at', 'desc', () => setPage(0));

  // Fetch users once for the user picker
  useEffect(() => {
    listUsers({ limit: 500, sort_by: 'email', sort_dir: 'asc' })
      .then((res) => setUsers(res.users ?? []))
      .catch(() => { /* non-critical */ });
  }, []);

  const load = useCallback(async (offset = 0) => {
    setLoading(true);
    setError('');
    const filters: AuditFilters = {
      limit: PAGE_SIZE,
      offset,
      sort_by: sort.sortKey,
      sort_dir: sort.sortDir,
    };
    if (filterUserIds.length) filters.user_ids = filterUserIds;
    if (filterSources.length) filters.sources = filterSources;
    if (filterAction.trim()) filters.action = filterAction.trim();
    if (filterStart) filters.start = new Date(filterStart).toISOString();
    if (filterEnd) filters.end = new Date(filterEnd).toISOString();

    try {
      const res = await listAuditEvents(filters);
      setEvents(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (err: unknown) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [filterUserIds, filterSources, filterAction, filterStart, filterEnd, sort.sortKey, sort.sortDir]);

  useEffect(() => {
    load(page * PAGE_SIZE);
  }, [page, sort.sortKey, sort.sortDir]);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setPage(0);
    load(0);
  }

  function handleClear() {
    setFilterUserIds([]);
    setFilterSources([]);
    setFilterAction('');
    setFilterStart('');
    setFilterEnd('');
    setPage(0);
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

  const userOptions: MultiPickOption[] = users.map((u) => ({ value: u.id, label: u.email }));
  const sourceOptions: MultiPickOption[] = ALL_SOURCES.map((s) => ({ value: s, label: s }));

  const [colWidths, setColWidths] = useState<number[]>(DEFAULT_COL_WIDTHS);
  const handleColWidthChange = useCallback((idx: number, w: number) => {
    setColWidths((prev) => { const next = [...prev]; next[idx] = w; return next; });
  }, []);

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Audit Logs</h1>
          <p className="page-subtitle">Real-time security and operational event trail</p>
        </div>
      </div>

      <form onSubmit={handleSearch}>
        <div className="filter-bar mb-16">
          <div className="form-group">
            <label>User</label>
            <MultiPickDropdown
              label="Users"
              options={userOptions}
              selected={filterUserIds}
              onChange={setFilterUserIds}
              searchable
              placeholder="All Users"
            />
          </div>
          <div className="form-group">
            <label>Source</label>
            <MultiPickDropdown
              label="Sources"
              options={sourceOptions}
              selected={filterSources}
              onChange={setFilterSources}
              placeholder="All Sources"
            />
          </div>
          <div className="form-group">
            <label>Action</label>
            <input
              type="text"
              value={filterAction}
              onChange={(e) => setFilterAction(e.target.value)}
              placeholder="e.g. login"
            />
          </div>
          <div className="form-group">
            <label>Start</label>
            <input
              type="datetime-local"
              value={filterStart}
              onChange={(e) => setFilterStart(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label>End</label>
            <input
              type="datetime-local"
              value={filterEnd}
              onChange={(e) => setFilterEnd(e.target.value)}
            />
          </div>
          <div>
            <button className="btn btn-primary" type="submit" disabled={loading}>
              Search
            </button>
            <button
              className="btn btn-secondary"
              type="button"
              style={{ marginLeft: 6 }}
              onClick={handleClear}
            >
              Clear
            </button>
          </div>
        </div>
      </form>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="text-sm mb-16">
        {loading ? 'Loading…' : `${total} event(s) total`}
        {totalPages > 1 && ` — page ${page + 1} of ${totalPages}`}
      </div>

      {!loading && events.length === 0 ? (
        <div className="empty-state">No audit events found.</div>
      ) : (
        <div className="table-wrap">
          <ResizableTable colWidths={colWidths} onColWidthChange={handleColWidthChange}>
            <thead>
              <tr>
                <SortHeader label="Time" sortKey="created_at" sort={sort} />
                <SortHeader label="Source" sortKey="source" sort={sort} />
                <SortHeader label="Action" sortKey="action" sort={sort} />
                <th>Email</th>
                <th>User ID</th>
                <th>IP</th>
                <th>Payload</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id}>
                  <td className="text-sm mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{fmt(e.created_at)}</td>
                  <td style={{ overflow: 'hidden' }}><span className="badge badge-blue">{e.source}</span></td>
                  <td style={{ overflow: 'hidden' }}><span className={`badge ${e.success ? 'badge-green' : 'badge-red'}`}>{e.action}</span></td>
                  <td className="text-sm" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {e.actor_email ?? '—'}
                  </td>
                  <td className="text-sm mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--color-text-muted)' }}>
                    {e.user_id ?? '—'}
                  </td>
                  <td className="text-sm mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{e.client_ip ?? '—'}</td>
                  <td className="text-sm" style={{ overflow: 'hidden' }}>
                    {e.payload && Object.keys(e.payload).length > 0 ? (
                      <details>
                        <summary style={{ cursor: 'pointer' }}>view</summary>
                        <pre style={{ fontSize: 11, marginTop: 4, whiteSpace: 'pre-wrap', maxWidth: 400 }}>
                          {JSON.stringify(e.payload, null, 2)}
                        </pre>
                      </details>
                    ) : (
                      '—'
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </ResizableTable>
        </div>
      )}

      {totalPages > 1 && (
        <div className="row mt-16" style={{ gap: 6 }}>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setPage((p) => p - 1)}
            disabled={page === 0 || loading}
          >
            ← Prev
          </button>
          <span className="text-sm" style={{ padding: '4px 8px' }}>
            {page + 1} / {totalPages}
          </span>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setPage((p) => p + 1)}
            disabled={page >= totalPages - 1 || loading}
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}

