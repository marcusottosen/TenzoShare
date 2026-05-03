# Logging & Observability

TenzoShare supports three logging setups. Pick one before starting the stack:

| Mode | Who it's for | How to enable |
|------|-------------|---------------|
| **1 — Docker logs only** | Local dev, quick eval | `COMPOSE_PROFILES=` (default, leave empty) |
| **2 — Built-in stack** | Self-hosted, no existing monitoring | `COMPOSE_PROFILES=observability` |
| **3 — BYO tools** | Orgs with existing Splunk/Datadog/etc. | `COMPOSE_PROFILES=` + configure your own driver |

Set `COMPOSE_PROFILES` in `infrastructure/docker/.env`, then run `docker compose up -d`. That's it — no CLI flags needed.

All services write **structured JSON logs to stdout/stderr only** (no local files). Every backend service also exposes two Docker-internal-only endpoints:

| Endpoint | Response |
|----------|----------|
| `GET /health` | `{"status":"ok","service":"<name>","uptime":"..."}` |
| `GET /metrics` | Prometheus text format — Go runtime + process stats |

---

## Mode 1 — Docker Logs Only (default)

Logs are still captured by Docker's `json-file` driver (10 MB × 3 files per container):

```bash
docker logs tenzoshare-auth
docker compose logs -f
docker logs tenzoshare-transfer --since 1h
```

To disable logging entirely for a service, set `logging.driver: none` in `docker-compose.yml`.

---

## Mode 2 — Built-in Stack

`COMPOSE_PROFILES=observability` starts four additional containers:

| Container | Role |
|-----------|------|
| `tenzoshare-loki` | Log store |
| `tenzoshare-promtail` | Collects Docker stdout via socket, ships to Loki |
| `tenzoshare-prometheus` | Scrapes each service's `/metrics` every 15s |
| `tenzoshare-grafana` | Dashboards — `http://<host>:3010` |

Grafana is on port **3010**, direct on the Docker host (not through Traefik). Credentials come from `.env`:

```bash
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=<your-password>
```

### Pre-provisioned Dashboards

| Dashboard | Contents |
|-----------|----------|
| **Service Logs** | Live log stream, filterable by service and level; error rate over time |
| **API Metrics** | Goroutines, heap, GC, CPU, RSS, open FDs per service |
| **Access Logs** | Traefik gateway stream — status codes, slow requests, request rate by method |
| **Transfer Activity** | Transfer/download/auth events; error rate by service |

### What Promtail Collects

Promtail uses Docker service discovery — every container named `tenzoshare-*` is collected automatically. The following JSON fields are parsed as queryable Loki labels or structured metadata:

| Field | Source |
|-------|--------|
| `level`, `msg`, `ts`, `caller`, `error` | All Go services (Zap) |
| `status`, `method`, `path` | Traefik, nginx |
| `duration` | Traefik |
| `service` | Container name (derived label) |

### Example Loki Queries

```logql
# All errors
{service=~"tenzoshare-.+"} | json | level = "error"

# Auth failures
{service="tenzoshare-auth"} | json | msg =~ "login failed|invalid credentials"

# Slow gateway requests (>1s)
{service="tenzoshare-gateway"} | json | duration > 1000

# Transfer creation events
{service="tenzoshare-transfer"} | json | msg = "transfer created"
```

---

## Mode 3 — BYO Tools

### Log Forwarding

Change the `logging` driver per service in `docker-compose.yml`:

**Syslog:**
```yaml
logging:
  driver: syslog
  options:
    syslog-address: "udp://your-syslog-server:514"
    tag: "tenzoshare-auth"
```

**Splunk:**
```yaml
logging:
  driver: splunk
  options:
    splunk-token: "<HEC-token>"
    splunk-url: "https://your-splunk:8088"
    splunk-format: json
    tag: "tenzoshare-transfer"
```

**Fluentd / Fluent Bit:**
```yaml
logging:
  driver: fluentd
  options:
    fluentd-address: "localhost:24224"
    tag: "tenzoshare.storage"
```

### Metrics Scraping

`/metrics` endpoints are reachable within the Docker network without authentication. To expose them on the host for an external scraper, add `ports` to the relevant service:

```yaml
# Example: expose all service metrics on the Docker host
services:
  auth:         { ports: ["9101:8081"] }
  transfer:     { ports: ["9102:8082"] }
  storage:      { ports: ["9103:8083"] }
  upload:       { ports: ["9104:8084"] }
  notification: { ports: ["9105:8085"] }
  audit:        { ports: ["9106:8086"] }
  admin:        { ports: ["9107:8087"] }
```

> **Security:** Metrics endpoints are unauthenticated. Only expose them on trusted interfaces or behind a firewall.

External Prometheus scrape config:
```yaml
scrape_configs:
  - job_name: tenzoshare
    static_configs:
      - targets: ["<host>:9101","<host>:9102","<host>:9103","<host>:9104","<host>:9105","<host>:9106","<host>:9107"]
```

### Health Checks

`GET /health` returns HTTP 200 with `{"status":"ok","service":"<name>","uptime":"..."}`. Works with any HTTP monitoring tool (Zabbix, Checkmk, Nagios, UptimeRobot, etc.).

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COMPOSE_PROFILES` | _(empty)_ | `observability` enables the built-in stack |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DEV_MODE` | `false` | `true` emits human-readable logs (local dev only) |
| `GRAFANA_ADMIN_USER` | `admin` | Grafana login username |
| `GRAFANA_ADMIN_PASSWORD` | _(required)_ | Grafana login password |
| `PROMETHEUS_RETENTION_TIME` | `15d` | Delete metrics older than this (e.g. `7d`, `30d`) |
| `PROMETHEUS_RETENTION_SIZE` | `5GB` | Cap Prometheus storage at this size — whichever limit hits first applies |
| `LOKI_RETENTION_PERIOD` | `30d` | Delete logs older than this (e.g. `14d`, `90d`) |
