# TenzoShare

Self-hosted file transfer — open source alternative to ShareFile / LinShare.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8.svg)](https://golang.org/)
[![React](https://img.shields.io/badge/react-19-61DAFB.svg)](https://react.dev/)

Go microservices backend, React frontend, everything runs in Docker. Passwords are Argon2id, files are AES-256-GCM encrypted at rest, transfers are link-based with optional expiry and password protection.

## Quick start

```bash
git clone https://github.com/tenzoshare/tenzoshare.git
cd tenzoshare
cp infrastructure/docker/.env.example infrastructure/docker/.env
# edit .env — set JWT_SECRET and PASSWORD_PEPPER at minimum
docker compose -f infrastructure/docker/docker-compose.yml up -d
```

Once up: API at `http://localhost/api/v1`, Traefik dashboard at `http://localhost:8080`.

## Services

| Service | Port | What it does |
|---------|------|--------------|
| Traefik (gateway) | 80 / 443 | routing, TLS |
| auth | 8081 | register, login, JWT, MFA, OIDC |
| transfer | 8082 | create/revoke transfer links |
| storage | 8083 | file metadata, MinIO ops, presign |
| upload | 8084 | resumable uploads via Tus |
| notification | 8085 | outbound email via NATS events |
| audit | 8086 | immutable event log |
| admin | 8087 | user management, system config |

Full details: [docs/architecture/README.md](docs/architecture/README.md)

## Stack

Backend: Go 1.26, Fiber v3, Connect-go (gRPC), pgx v5, go-redis v9, NATS JetStream, Zap  
Frontend: React 19, TypeScript, Vite  
Infra: PostgreSQL 17, Redis 7, MinIO, NATS 2.12, Traefik v3  
Auth: fosite (OIDC), golang-jwt, pquerna/otp (TOTP)  
Uploads: tusd v2 (Tus resumable upload protocol)

## Repo layout

```
tenzoshare/
├── services/          # one directory per microservice
├── web/               # React frontend
├── shared/            # shared Go library (config, logger, crypto, middleware, ...)
├── proto/             # protobuf definitions (buf)
├── infrastructure/    # Docker Compose stacks, Traefik config, init SQL
├── docs/              # architecture notes, dev guides, ADRs
├── scripts/           # generate-proto.sh and other helpers
└── tests/             # E2E (Playwright), load (k6), integration tests
```

## Development

Needs: Go 1.26, Docker, [buf CLI](https://buf.build/docs/installation), golangci-lint

```bash
go work sync
go build ./...          # build everything
go test -race ./...     # run tests
./scripts/generate-proto.sh   # regenerate gRPC stubs
```

See [docs/development/local-development.md](docs/development/local-development.md) for the full setup walkthrough.

## Roadmap

| Phase | Status |
|-------|--------|
| Phase 0: Foundation | done |
| Phase 1: MVP | in progress |
| Phase 2: Enterprise features | planned |

See [Requirements.md](Requirements.md) for the full spec.

## License

Apache 2.0 — see [LICENSE](LICENSE).
