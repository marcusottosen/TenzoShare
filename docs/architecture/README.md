# Architecture

Monorepo. Seven Go microservices sharing a common library. Traefik in front, PostgreSQL + Redis + MinIO + NATS behind.

## Layout

```
TenzoShare/
├── go.work                   # Go workspace — ties all modules together
├── services/                 # One directory per microservice
│   ├── auth/                 # AuthService       — port 8081
│   ├── transfer/             # TransferService   — port 8082
│   ├── storage/              # StorageService    — port 8083
│   ├── upload/               # UploadService     — port 8084  (Tus resumable uploads)
│   ├── notification/         # NotificationService — port 8085
│   ├── audit/                # AuditService      — port 8086
│   └── admin/                # AdminService      — port 8087
├── shared/                   # Shared Go library (config, logger, middleware, crypto, …)
├── proto/                    # Protobuf definitions (buf.yaml, buf.gen.yaml)
│   └── gen/go/               # Generated Go stubs (gitignored until buf runs)
├── web/                      # React 19 + Vite frontend
├── infrastructure/
│   └── docker/               # Docker Compose stacks, Traefik config, init SQL
├── scripts/                  # Helper shell scripts
├── docs/                     # Architecture records, dev guides, API specs
└── .github/workflows/        # CI/CD pipelines
```

## Services

| Service      | Does what                                                      | Deps               |
|--------------|----------------------------------------------------------------|--------------------|
| auth         | register, login, JWT issue/refresh, MFA (TOTP), OIDC           | postgres, redis    |
| transfer     | transfer links, expiry, password protection, revoke            | postgres, nats     |
| storage      | file metadata, MinIO ops, presigned URLs                       | postgres, minio    |
| upload       | resumable uploads via Tus (tusd)                               | minio, nats        |
| notification | outbound email, triggered by NATS events                       | nats               |
| audit        | append-only event log                                          | postgres, nats     |
| admin        | user management, system config                                 | postgres, redis    |

## Communication

- Sync inter-service calls: gRPC via Connect (`connectrpc.com/connect`)
- Async: NATS JetStream — durable consumers, at-least-once
- External HTTP: Fiber v3 JSON, routed through Traefik v3

## Infrastructure

| | Image | Role |
|---|---|---|
| Traefik | `traefik:v3.6.14` | gateway, TLS |
| PostgreSQL | `postgres:17-alpine` | primary DB |
| Redis | `redis:7-alpine` | sessions, rate limiting |
| MinIO | `minio/minio:latest` | object storage |
| NATS | `nats:2.12.7-alpine` | message broker |

Optional observability stack (separate compose profile): Prometheus, Grafana, Loki, Tempo.

## Security notes

- Passwords hashed with Argon2id (time=1, mem=64 MB, threads=4) + server pepper
- Files encrypted at rest with AES-256-GCM
- JWT: HMAC-signed, access=15 min, refresh=168 h
- `DEV_MODE=true` — relaxed CORS, `Secure:false` cookies (local dev only)
- `DEV_MODE=false` — HSTS, `Secure`/`SameSite:Strict`, TLS 1.3

## ADRs

See [adr/](adr/).
