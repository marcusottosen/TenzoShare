import React, { useEffect, useState } from 'react';

interface ServiceInfo {
  name: string;
  port: number;
  path: string;
}

const SERVICES: ServiceInfo[] = [
  { name: 'Auth',         port: 8081, path: '/api/v1/auth/health' },
  { name: 'Transfer',     port: 8082, path: '/api/v1/transfers/health' },
  { name: 'Storage',      port: 8083, path: '/api/v1/files/health' },
  { name: 'Upload',       port: 8084, path: '/api/v1/uploads/health' },
  { name: 'Notification', port: 8085, path: '/api/v1/notification/health' },
  { name: 'Audit',        port: 8086, path: '/api/v1/audit/health' },
  { name: 'Admin',        port: 8087, path: '/api/v1/admin/health' },
];

type ServiceStatus = 'checking' | 'up' | 'down';

interface HealthState {
  status: ServiceStatus;
  latencyMs?: number;
}

async function checkHealth(path: string): Promise<HealthState> {
  const start = Date.now();
  try {
    const res = await fetch(path, { signal: AbortSignal.timeout(5000) });
    const latencyMs = Date.now() - start;
    return { status: res.ok ? 'up' : 'down', latencyMs };
  } catch {
    return { status: 'down' };
  }
}

export default function DashboardPage() {
  const [health, setHealth] = useState<Record<string, HealthState>>({});
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [checking, setChecking] = useState(false);

  async function runChecks() {
    setChecking(true);
    const initial: Record<string, HealthState> = {};
    SERVICES.forEach((s) => { initial[s.name] = { status: 'checking' }; });
    setHealth(initial);

    const results = await Promise.all(
      SERVICES.map(async (s) => ({ name: s.name, ...(await checkHealth(s.path)) })),
    );
    const next: Record<string, HealthState> = {};
    results.forEach((r) => { next[r.name] = { status: r.status, latencyMs: r.latencyMs }; });
    setHealth(next);
    setLastChecked(new Date());
    setChecking(false);
  }

  useEffect(() => { runChecks(); }, []);

  const upCount = Object.values(health).filter((h) => h.status === 'up').length;
  const totalCount = SERVICES.length;

  return (
    <div className="page">
      <div className="row mb-16" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 className="page-title" style={{ margin: 0 }}>System Health</h1>
        <div>
          {lastChecked && (
            <span className="text-sm" style={{ marginRight: 12 }}>
              Last checked: {lastChecked.toLocaleTimeString()}
            </span>
          )}
          <button className="btn btn-secondary" onClick={runChecks} disabled={checking}>
            {checking ? 'Checking…' : 'Refresh'}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 20 }}>
        <div style={{ fontSize: 28, fontWeight: 700 }}>
          {upCount} / {totalCount}
          <span style={{ fontSize: 14, fontWeight: 400, color: '#888', marginLeft: 8 }}>services up</span>
        </div>
        {upCount < totalCount && (
          <div className="alert alert-warning mt-8">
            {totalCount - upCount} service(s) are down or unreachable.
          </div>
        )}
      </div>

      <div className="health-grid">
        {SERVICES.map((s) => {
          const h = health[s.name];
          return (
            <div key={s.name} className="health-card">
              <div className="health-card-name">{s.name}</div>
              <div className="health-card-port">:{s.port}</div>
              {!h || h.status === 'checking' ? (
                <span className="badge badge-gray">Checking…</span>
              ) : h.status === 'up' ? (
                <span className="badge badge-green">
                  UP {h.latencyMs !== undefined && `(${h.latencyMs}ms)`}
                </span>
              ) : (
                <span className="badge badge-red">DOWN</span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
