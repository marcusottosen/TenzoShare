# TenzoShare — AI Agent Reference

Secure file-transfer platform. Go monorepo + React SPAs.

---

## Tech Stack

| Layer | Tech |
|---|---|
| Language | Go 1.26.3, go work monorepo |
| HTTP | Fiber v3.1.0 — handler signature is `func(c fiber.Ctx) error` (no pointer) |
| Auth | RS256 JWT (access 15m, refresh 7d), Argon2id+pepper passwords, TOTP MFA |
| DB | PostgreSQL via pgx/v5 pool — schemas: `auth`, `transfer`, `storage`, `audit`, `admin_svc` |
| Cache | Redis — JWT JTI revocation blacklist + rate limiting |
| Queue | NATS JetStream — streams: `UPLOADS`, `NOTIFICATIONS`, `AUDIT` |
| Storage | MinIO (S3-compatible), AES-256-GCM encrypted at rest |
| Frontend | React 19 + TypeScript + Tailwind v4 + Vite |

---

## Repo Layout

```
shared/pkg/          # shared Go libraries used by all services
  config/            # env-var config loader
  crypto/            # AES-256-GCM, Argon2id, random tokens
  database/          # pgxpool factory + RunMigrations()
  middleware/        # Fiber: JWTAuth, RequireRole, TokenRevocation, CORS, SecurityHeaders
  errors/            # typed AppError with HTTP status codes
  cache/             # Redis wrapper
  jetstream/         # NATS JetStream client
  jwtkeys/           # RSA key parsing (PKCS8/PKCS1)

services/
  auth/      :8081   # register, login, MFA, JWT, API keys, password reset
  transfer/  :8082   # transfers + file requests (dropbox)
  storage/   :8083   # file upload/download, MinIO, encryption
  upload/    :8084   # tus resumable uploads → MinIO
  notification/:8085 # NATS email consumer, SMTP
  audit/     :8086   # NATS audit consumer, partitioned audit_logs
  admin/     :8087   # admin CRUD, stats, branding, system config

web/
  user-portal/   :3000   # React SPA — main user interface
  admin-portal/  :3001   # React SPA — admin interface
  request-ui/    :3002   # React SPA — public guest upload form
  download-ui/   :3003   # React SPA — public download page

infrastructure/docker/   # docker-compose.yml, .env.example, traefik/, postgres/
```

---

## Key Conventions

**Go services**
- Each service has its own `go.mod` under `services/<name>/`; all use `go work` at repo root.
- Module path pattern: `github.com/tenzoshare/tenzoshare/services/<name>`
- Build with `-mod=vendor` (vendor dir at repo root). Run `go work vendor` after adding deps.
- Migrations are SQL files in `services/<name>/migrations/`, embedded via `migrations/migrations.go`, applied at startup with `database.RunMigrations()`. The tracking table is `<schema>.schema_migrations`.
- `shared/pkg/errors` — use `apperrors.NotFound()`, `apperrors.Unauthorized()`, etc. Never return raw errors as HTTP responses.
- Fiber v3: no `*fiber.Ctx`, always `fiber.Ctx`. `c.Locals("userID")`, `c.Locals("userRole")` set by `JWTAuth` middleware.
- `cfg.App.DevMode` controls CORS permissiveness, HSTS, and cookie Secure flag.

**Tests**
- Use `package service` (not `_test`) to access unexported fields.
- Stub repos implement interfaces in-memory; `AuthService` has nil-safe `cache` and `js` fields.
- Run: `cd /home/marcus/TenzoShare && go test ./shared/... ./services/auth/... ./services/transfer/...`
- No external dependencies needed for unit tests (Redis/NATS/DB are stubbed).

**Frontend (React)**
- All API calls go through `/api/v1/...` — proxied by nginx to Traefik gateway.
- Auth tokens stored in `localStorage` (`access_token`, `refresh_token`).
- Tailwind v4 — use utility classes; dark card class: `widget-dark` / `bg-[#1E293B]`.

**Infrastructure**
- `docker-compose.yml` in `infrastructure/docker/`. Copy `.env.example` → `.env` and fill secrets.
- `CORS_ALLOWED_ORIGINS` — comma-separated list; empty = all blocked in prod; dev mode reflects any origin.
- Traefik routes by path prefix; services are only reachable via Traefik (not exposed directly).
- `COMPOSE_PROFILES=observability` adds Grafana/Loki/Prometheus/Promtail.
- PostgreSQL `init.sql` creates the full schema on first run; service migrations handle upgrades.

---

## Security Invariants — Do Not Break

- All passwords hashed with Argon2id + `cfg.App.Pepper` (never bcrypt, never without pepper).
- JWT signed RS256; private key only in auth+transfer services. All others use public key only.
- Refresh tokens stored as SHA-256 hash only. Raw token returned once, never stored.
- API keys stored as SHA-256 hash + prefix only.
- File content always AES-256-GCM encrypted in MinIO; `encryption_iv` stored in `storage.files`.
- `TokenRevocation` middleware must run after `JWTAuth` on all authenticated routes.
- Rate limiting on login (5/15min/IP), register (10/hr/IP), password-reset (5/hr/IP) via Redis INCR.
- `SecurityHeaders()` and `CORS()` must be the first middleware on every Fiber app.
- No cross-schema foreign keys — services reference each other by UUID only.

---

## Building & Deploying

All services run in Docker. Build and redeploy any service with:

```bash
# Build a single service image
docker compose -f infrastructure/docker/docker-compose.yml build <service>

# Restart the container with the new image
docker compose -f infrastructure/docker/docker-compose.yml up -d <service>
```

Service names in docker-compose.yml match their `container_name` (e.g. `tenzoshare-web`, `tenzoshare-auth`, `tenzoshare-transfer`).

Frontend SPAs are built inside Docker (multi-stage Dockerfile) — do **not** run `npm run build` locally; the Docker build handles it. The local `dist/` folder may be root-owned from a previous Docker build; ignore it.

---

## Adding a New Migration

1. Add `services/<name>/migrations/NNN_description.sql` (idempotent — use `IF NOT EXISTS`).
2. Also add the full consolidated change to `infrastructure/docker/postgres/init.sql` (for fresh installs).
3. Record the filename in the `schema_migrations` INSERT block in `init.sql`.

## Adding a New Service Dependency

```bash
cd services/<name> && go get <pkg>
go work vendor   # from repo root — regenerates /vendor/
```
