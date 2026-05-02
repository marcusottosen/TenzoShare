# TenzoShare

Self-hosted, encrypted file transfer — open-source alternative to ShareFile and LinShare.

[![License](https://img.shields.io/badge/license-AGPL%203.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8.svg)](https://golang.org/)
[![React](https://img.shields.io/badge/react-19-61DAFB.svg)](https://react.dev/)

TenzoShare is a self-hosted file transfer platform built for teams and organisations that need auditable, encrypted file exchange without handing data to a third party. Files are AES-256-GCM encrypted at rest, transfers are link-based with optional expiry, password protection, and download limits. Everything runs in Docker.

---

## Features

- **Secure transfers** — one-time or multi-use download links, expiry dates, per-transfer passwords, download caps
- **View-only mode** — recipients can preview files in-browser without being able to download them
- **File requests** — send a dropbox link; recipients upload directly to you
- **Resumable uploads** — large files via the Tus protocol; survives network interruption
- **File encryption** — AES-256-GCM, 2 MiB chunks, nonce per chunk; plaintext never touches disk
- **TOTP MFA** — per-account two-factor with authenticator apps
- **API keys** — personal access tokens for CLI and scripting workflows
- **Per-user quotas** — storage limits enforced at upload time
- **Audit log** — immutable, append-only event log for every auth, transfer, and file action
- **Admin panel** — user management, quota configuration, system-wide policy
- **Fully API-first** — every UI action is an API call; the UIs are just reference consumers

---

## Security

| Layer | Detail |
|-------|--------|
| **Passwords** | Argon2id (t=1, m=64 MB, p=4) + server-side pepper per service |
| **JWT** | RS256 — services hold only the public key; private key lives in auth + transfer only |
| **Files at rest** | AES-256-GCM per-file encryption in MinIO; random 12-byte nonce per chunk |
| **Download tokens** | Short-lived HS256 tokens (15 min) embedded in presigned URLs |
| **View-only enforcement** | Server appends `?inline=1` to download URLs; `Content-Disposition: inline` returned by storage |
| **Transport** | TLS 1.3 via Traefik; `Strict-Transport-Security` in production |
| **Response headers** | `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, strict CSP, `Permissions-Policy` |
| **Rate limiting** | 5 login attempts / 15 min / IP (Redis); account lockout after 10 consecutive failures |
| **API keys** | Only SHA-256 hash stored; raw key shown once at creation |
| **TOTP secrets** | AES-256-GCM encrypted before storing |
| **CORS** | Per-service allowlist via `CORS_ALLOWED_ORIGINS` |
| **Audit log** | Append-only, date-partitioned PostgreSQL table |
| **CI scanning** | `gosec` + Trivy on every push (CRITICAL/HIGH block merge) |

---

## Quick Start

**Prerequisites:** Docker with Compose v2.

```bash
git clone https://github.com/tenzoshare/tenzoshare.git
cd tenzoshare

# Generate RSA key pair for JWT signing
openssl genrsa -out infrastructure/docker/jwt_private.pem 4096
openssl rsa -in infrastructure/docker/jwt_private.pem -pubout -out infrastructure/docker/jwt_public.pem

# Copy and configure environment
cp infrastructure/docker/.env.example infrastructure/docker/.env
# Edit .env — set strong values for PASSWORD_PEPPER and any secrets

# Start the stack
docker compose -f infrastructure/docker/docker-compose.yml up -d
```

| Endpoint | URL |
|----------|-----|
| User portal | `http://localhost` |
| Admin panel | `http://localhost/admin` |
| API | `http://localhost/api/v1` |
| Traefik dashboard | `http://localhost:8080` |

Default admin credentials are set via `ADMIN_EMAIL` / `ADMIN_PASSWORD` in `.env`.

---

## Services

| Service | What it does |
|---------|--------------|
| **Traefik** | Gateway, routing, TLS termination |
| **auth** | Registration, login, JWT issuance, MFA, API keys |
| **transfer** | Transfer lifecycle — create, list, revoke, track |
| **storage** | File metadata, MinIO operations, encryption/decryption, presigned URLs |
| **upload** | Resumable uploads via Tus (tusd v2) |
| **notification** | Outbound email; consumes NATS events asynchronously |
| **audit** | Immutable event log; receives from all services via NATS |
| **admin** | User management, quotas, system configuration |

---

## Stack

| Layer | Technologies |
|-------|-------------|
| **Backend** | Go 1.26, Fiber v3, Connect-go (gRPC), pgx v5, go-redis v9, NATS JetStream, Zap |
| **Frontend** | React 19, TypeScript, Vite, Tailwind CSS v4 |
| **Infrastructure** | PostgreSQL 17, Redis 7, MinIO, NATS 2.12, Traefik v3 |
| **Auth** | golang-jwt (RS256), pquerna/otp (TOTP) |
| **Uploads** | tusd v2 (Tus resumable upload protocol) |

---

## Repository Layout

```
tenzoshare/
├── services/        # one directory per microservice (Go)
├── web/             # React frontends (user-portal, download-ui, request-ui, admin-portal)
├── shared/          # shared Go library — config, logger, crypto, middleware, telemetry
├── proto/           # protobuf definitions (buf)
├── infrastructure/  # Docker Compose stacks, Traefik config, secrets
├── docs/            # architecture notes, ADRs, API specs, dev guides
├── scripts/         # generate-proto.sh and other helpers
└── tests/           # E2E (Playwright), integration, load (k6)
```

---

## Development

Requirements: Go 1.26, Docker, [buf CLI](https://buf.build/docs/installation), golangci-lint

```bash
go work sync
go build ./...               # build all services
go test -race ./...          # run tests with race detector
./scripts/generate-proto.sh  # regenerate gRPC stubs from proto definitions
```

See [docs/development/local-development.md](docs/development/local-development.md) for a complete local setup walkthrough.

---

## Roadmap

| Phase | Status |
|-------|--------|
| Phase 0: Foundation | done |
| Phase 1: MVP | in progress |
| Phase 2: Enterprise features | planned |

Full specification: [Requirements.md](Requirements.md)

---

## License

AGPL 3.0 — see [LICENSE](LICENSE).

If you run a modified version of TenzoShare as a network service, the AGPL requires you to make the modified source available to users of that service.
