# Local development

## What you need

- Go 1.26+ (`snap install go --classic` or https://go.dev/dl/)
- Docker with the Compose plugin (v27+)
- [buf CLI](https://buf.build/docs/installation) (for proto generation)
- golangci-lint (for linting)

## First-time setup

```bash
git clone https://github.com/tenzoshare/tenzoshare.git
cd tenzoshare
cp infrastructure/docker/.env.example infrastructure/docker/.env
```

Open `.env` and set at minimum:

```bash
JWT_SECRET=$(openssl rand -hex 32)
PASSWORD_PEPPER=$(openssl rand -hex 16)
```

Everything else has sane defaults for local dev.

## Start the infrastructure

```bash
cd infrastructure/docker
docker compose up -d
docker compose ps    # wait until all show "healthy"
```

That starts Traefik, PostgreSQL, Redis, MinIO, and NATS.

| URL | What |
|-----|------|
| http://localhost:8080 | Traefik dashboard |
| http://localhost:9001 | MinIO console |
| http://localhost:8222 | NATS monitor |

Optional — observability stack (Prometheus, Grafana, Loki, Tempo):

```bash
docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d
# Grafana at http://localhost:3000
```

## Build and test

```bash
# from repo root
go work sync
go build ./...         # build everything
go test -race ./...    # run tests
```

For a single service:

```bash
cd services/auth
go build ./...
go test ./...
```

## Run a service locally

Each service is just a binary that reads env vars. Point it at the local infra:

```bash
export DEV_MODE=true
export LOG_LEVEL=debug
export POSTGRES_HOST=localhost
export POSTGRES_PASSWORD=tenzoshare    # from your .env
export REDIS_PASSWORD=redis_password   # from your .env
export NATS_URL=nats://localhost:4222
export JWT_SECRET=<value from .env>
export PASSWORD_PEPPER=<value from .env>
export S3_ENDPOINT=localhost:9000
export S3_ACCESS_KEY=minioadmin
export S3_SECRET_KEY=minioadmin
export S3_BUCKET=tenzoshare
export S3_USE_SSL=false

cd services/auth
go run ./cmd/server
```

The default port for each service is 808x (auth=8081, transfer=8082, ...). Override with `PORT=808x`.

## Generate proto stubs

```bash
./scripts/generate-proto.sh
# output lands in proto/gen/go/
```

## Lint

```bash
golangci-lint run ./...
```

## Tear down

```bash
cd infrastructure/docker
docker compose down        # stops containers, volumes survive
docker compose down -v     # also wipes volumes (destructive)
```
