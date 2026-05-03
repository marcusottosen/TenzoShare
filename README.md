# TenzoShare

**Self-hosted, API- and Docker-based, encrypted file transfer**

[![License: AGPL 3.0](https://img.shields.io/badge/license-AGPL%203.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg)](https://golang.org/)
[![React](https://img.shields.io/badge/react-19-61DAFB.svg)](https://react.dev/)
[![Docker](https://img.shields.io/badge/docker-compose-2496ED.svg)](infrastructure/docker/)

[Website](https://tenzoshare.com) &nbsp;·&nbsp;
[Features](https://tenzoshare.com/features) &nbsp;·&nbsp;
[Security](https://tenzoshare.com/security) &nbsp;·&nbsp;
[Roadmap](https://tenzoshare.com/roadmap) &nbsp;·&nbsp;
[Open Source](https://tenzoshare.com/opensource)

---

TenzoShare lets teams and organisations share files securely — without handing data to a third party. Files are AES-256-GCM encrypted at rest. Transfers are link-based with expiry dates, password protection, and download caps. Every action is written to an immutable audit log. The whole stack runs in Docker with a single `docker compose up`.
The project is build to be API-first segregating the backend from the frontend.

---

## Features

- **Secure transfers** — download links with expiry, per-transfer passwords, download caps, and one-click revocation
- **View-only mode** — recipients can preview files in-browser without being able to save them (compliance-friendly)
- **File requests** — share a dropbox link; guests upload directly to you without needing an account
- **Resumable uploads** — large files via the [Tus](https://tus.io/) protocol; pauses and resumes across network interruptions
- **Encryption at rest** — AES-256-GCM per-file encryption; plaintext never touches disk
- **Two-factor authentication** — TOTP with any authenticator app
- **API keys** — personal access tokens for CLI and scripting workflows
- **Role-based access control** — admin / user / guest roles, enforced at the API layer
- **Immutable audit log** — append-only event log for every auth, file, and admin action; searchable and exportable
- **Admin panel** — user management, quota controls, storage policies, branding, system health
- **Observability** — optional Prometheus + Grafana + Loki stack; per-request and per-user log search
- **API-first** — every UI action is a documented API call; the UIs are reference consumers, not special clients

→ Feature list at [tenzoshare.com/features](https://tenzoshare.com/features)

---

## Quick Start

**Requirements:** Docker with Compose v2.

```bash
git clone https://github.com/tenzoshare/tenzoshare.git
cd tenzoshare

# Generate the RSA key pair used for JWT signing
openssl genrsa -out infrastructure/docker/jwt_private.pem 4096
openssl rsa -in infrastructure/docker/jwt_private.pem -pubout -out infrastructure/docker/jwt_public.pem

# Configure your environment
cp infrastructure/docker/.env.example infrastructure/docker/.env
# Edit .env — set PASSWORD_PEPPER and strong passwords

# Start the stack
cd infrastructure/docker
docker compose up -d
```

| Service | URL |
|---|---|
| User portal | `http://localhost` |
| Admin panel | `http://localhost/admin` |
| API | `http://localhost/api/v1` |
| Traefik dashboard | `http://localhost:8080` |

Default admin credentials are set via `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` in `.env`.

See [docs/development/local-development.md](docs/development/local-development.md) for a full local setup guide.

---

## Security

| Layer | Detail |
|---|---|
| Passwords | Argon2id (t=1, m=64 MB, p=4) + server-side pepper |
| JWT | RS256 — services hold only the public key; private key lives in auth only |
| Files at rest | AES-256-GCM per-file encryption; random 12-byte nonce per chunk |
| Transport | TLS 1.3 via Traefik; `Strict-Transport-Security` in production |
| Response headers | `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, strict CSP |
| Rate limiting | 5 login attempts / 15 min / IP; account lockout after configurable failures |
| API keys | Only SHA-256 hash stored; raw key shown once at creation |
| Audit log | Append-only, date-partitioned PostgreSQL table |
| CI scanning | `gosec` + Trivy on every push — CRITICAL/HIGH findings block merge |

→ Full security model at [tenzoshare.com/security](https://tenzoshare.com/security)

---

## Stack

| Layer | Technologies |
|---|---|
| Backend | Go 1.26, Fiber v3, Connect-go (gRPC), pgx v5, NATS JetStream, Zap |
| Frontend | React 19, TypeScript 6, Vite, Tailwind CSS v4 |
| Infrastructure | PostgreSQL 17, Redis 7, MinIO, NATS 2.12, Traefik v3 |
| Observability | Prometheus, Grafana, Loki, Promtail (optional — `COMPOSE_PROFILES=observability`) |

---

## Repository Layout

```
tenzoshare/
├── services/        # microservices (Go) — auth, transfer, storage, upload, notification, audit, admin
├── web/             # React SPAs — user-portal, admin-portal, download-ui, request-ui
├── shared/          # shared Go library — config, crypto, logger, middleware, telemetry
├── proto/           # protobuf definitions (buf)
├── infrastructure/  # Docker Compose, Traefik config, Prometheus, Grafana
├── docs/            # architecture notes, ADRs, API specs, local dev guide
├── scripts/         # proto generation and other helpers
└── tests/           # E2E (Playwright), integration, load (k6)
```

---

## Roadmap

| Phase | Status |
|---|---|
| Phase 0 — Foundation | ✅ Complete |
| Phase 1 — MVP | 🔄 In progress |
| Phase 2 — Enterprise (SSO, WebAuthn, ClamAV, Webhooks, Helm) |
| Phase 3 — Scale (SAML, LDAP, multi-tenancy, CLI, SDKs) |

→ Detailed roadmap at [tenzoshare.com/roadmap](https://tenzoshare.com/roadmap)

---

## Contributing

Contributions are welcome — bug reports, feature requests, documentation improvements, and code.

1. Check [open issues](https://github.com/tenzoshare/tenzoshare/issues) or open a new one to discuss your idea
2. Fork the repo and create a branch from `main`
3. Make your changes with tests (target 80%+ coverage on service layers)
4. Open a pull request with a clear description of what and why

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide and code style notes.

```bash
# Build and test everything locally
go work sync
go build ./...
go test -race ./...
```

---

## License

[AGPL 3.0](LICENSE) — if you run a modified version of TenzoShare as a network service, the AGPL requires you to make the modified source available to users of that service.
