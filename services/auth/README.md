# Auth Service

Handles all authentication and authorization for TenzoShare.

## Responsibilities

- Local email/password registration and login
- JWT issuance (access tokens + refresh tokens) using RS256
- Token refresh and revocation (blacklist via Redis)
- Multi-factor authentication (TOTP via `pquerna/otp`)
- OIDC relying-party integration (Microsoft Entra ID, Google, Okta, Keycloak, Authentik)
- Password reset flow (email-based)
- Account lockout after repeated failed attempts
- Session management

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 8081 | HTTP/gRPC (Connect) | Internal gRPC service |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8081` | Listen port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `tenzoshare` | Database name |
| `DB_USER` | `tenzoshare` | Database user |
| `DB_PASSWORD` | — | Database password |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `JWT_PRIVATE_KEY_PATH` | — | Path to RS256 private key |
| `JWT_PUBLIC_KEY_PATH` | — | Path to RS256 public key |
| `BASE_URL` | `http://localhost` | Base URL for redirect URIs |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

## Database Tables

- `users` — User accounts and credentials
- `sessions` — Active refresh token records
- `mfa_devices` — TOTP/WebAuthn registrations
- `password_reset_tokens` — Single-use reset tokens

## gRPC API

See `proto/auth/v1/auth.proto` for the full service definition.
