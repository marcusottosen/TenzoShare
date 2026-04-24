# TenzoShare

> Modern, open-source, Docker-native file transfer platform focused on secure, auditable, and enterprise-ready file exchange.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://golang.org/)
[![React](https://img.shields.io/badge/react-18+-61DAFB.svg)](https://react.dev/)

## Overview

TenzoShare is a self-hosted alternative to proprietary file transfer tools like LinShare, Citrix ShareFile, and ProjectSend. It is built with a microservices architecture, API-first design, and Docker-native deployment.

**Core Value Proposition:**
- **Open Source** — Full transparency, no vendor lock-in
- **Security-First** — AES-256-GCM encryption, audit trails, compliance-ready
- **Docker-Native** — Deploy anywhere in under 5 minutes
- **API-First** — Every feature accessible via REST or gRPC
- **Enterprise-Ready** — SSO (OIDC), SCIM, RBAC, quotas, retention policies

## Quick Start

```bash
# Clone the repository
git clone https://github.com/tenzoshare/tenzoshare.git
cd tenzoshare

# Copy environment configuration
cp infrastructure/docker/.env.example infrastructure/docker/.env

# Start all services
docker compose -f infrastructure/docker/docker-compose.yml up -d

# Access the application
# User Portal:  http://localhost:3000
# Admin Portal: http://localhost:3001
# API:          http://localhost:80/api/v1
```

## Architecture

TenzoShare uses a microservices architecture with the following core services:

| Service | Port | Description |
|---------|------|-------------|
| API Gateway (Traefik) | 80 / 443 | Routing, TLS termination, rate limiting |
| Auth Service | 8081 | Authentication, JWT, MFA, SSO |
| Transfer Service | 8082 | Send/receive workflows, link generation |
| Storage Service | 8083 | S3 abstraction, encryption, pre-signed URLs |
| Upload Service | 8084 | Tus resumable uploads |
| Notification Service | 8085 | Email notifications, NATS consumer |
| Audit Service | 8086 | Immutable audit logging |
| Admin Service | 8087 | User management, quotas, policies |
| Web UI | 3000 | User-facing SPA |
| Admin UI | 3001 | Admin dashboard SPA |

See [docs/architecture/README.md](docs/architecture/README.md) for full architecture documentation.

## Technology Stack

- **Backend**: Go 1.23+, Go Fiber, Connect (gRPC)
- **Frontend**: React 18, TypeScript 5, shadcn/ui, Tailwind CSS
- **Database**: PostgreSQL 16+
- **Cache**: Redis 7+ / Valkey 7+
- **Object Storage**: MinIO (local), AWS S3, Azure Blob, GCS
- **Message Queue**: NATS JetStream 2.10+
- **API Gateway**: Traefik 2.x
- **Auth**: ory/fosite (OAuth 2.0 + OIDC)
- **File Upload**: tus/tusd (resumable uploads)

## Repository Structure

```
tenzoshare/
├── services/          # Go microservices (auth, transfer, storage, etc.)
├── web/               # React frontends (user-portal, admin-portal)
├── shared/            # Shared Go libraries (pkg/logger, pkg/crypto, etc.)
├── proto/             # Centralized Protobuf/gRPC definitions (Buf)
├── infrastructure/    # Docker Compose, Swarm, K8s, Terraform configs
├── docs/              # Architecture, deployment, development docs
├── scripts/           # Dev tooling scripts
└── tests/             # E2E (Playwright), load (k6), integration tests
```

## Development Setup

See [docs/development/local-setup.md](docs/development/local-setup.md) for detailed instructions.

```bash
# Prerequisites: Go 1.23+, Node.js 20+, Docker, buf CLI
./scripts/dev-setup.sh

# Run all tests
./scripts/test-all.sh

# Regenerate proto code
./scripts/generate-proto.sh

# Run database migrations
./scripts/migrate-db.sh
```

## Roadmap

| Phase | Timeline | Status |
|-------|----------|--------|
| Phase 0: Foundation | Q2 2026 | In Progress |
| Phase 1: MVP | Q3 2026 | Planned |
| Phase 2: Enterprise | Q4 2026 | Planned |
| Phase 3: Scale & Extend | Q1-Q2 2027 | Planned |

See [Requirements.md](Requirements.md) for the full product requirements document.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/development/contributing.md](docs/development/contributing.md).

## License

Apache 2.0 — see [LICENSE](LICENSE).
