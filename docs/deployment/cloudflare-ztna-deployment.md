# TenzoShare Cloudflare Deployment Options

## Deployment Approaches

### Option 1: Subdomain-Based Zones (Recommended)
**Example Domain:** portofaalborg.com

| Zone | Subdomain | Port | Security |
|------|-----------|------|----------|
| **User Portal** | fileshare.portofaalborg.com | 3000 | OIDC auth via Cloudflare Access |
| **Admin Portal** | fileshareadmin.portofaalborg.com | 3001 | OIDC + MFA + admin allowlist |
| **Download UI** | download.portofaalborg.com | 3003 | Public (bypass auth) |
| **Upload UI** | upload.portofaalborg.com | 3002 | Public (bypass auth) |

**Cloudflare Tunnel Config:**
```yaml
ingress:
  - hostname: fileshare.portofaalborg.com
    service: http://localhost:3000
  - hostname: fileshareadmin.portofaalborg.com
    service: http://localhost:3001
  - hostname: download.portofaalborg.com
    service: http://localhost:3003
  - hostname: upload.portofaalborg.com
    service: http://localhost:3002
  - service: http_status:404
```

**Cloudflare Access Applications:**
- **Admin Zone:** fileshareadmin.portofaalborg.com → Require: specific admin emails + MFA
- **User Zone:** fileshare.portofaalborg.com → Require: @portofaalborg.com email domain
- **Public Zone:** download/upload subdomains → Bypass (no auth)

---

### Option 2: Path-Based Zones
**Example:** Everything under fileshare.portofaalborg.com

| Zone | Path | Port | Security |
|------|------|------|----------|
| User Portal | / | 3000 | OIDC auth |
| Admin Portal | /admin | 3001 | OIDC + MFA + allowlist |
| Download | /d/* | 3003 | Public bypass |
| Upload | /u/* | 3002 | Public bypass |

**Pros:** Single domain  
**Cons:** More complex routing, harder to isolate security zones

---

### Option 3: Hybrid (Subdomains + Paths)
**Example:** portofaalborg.com

| Zone | Domain/Path | Security |
|------|-------------|----------|
| User Portal | app.portofaalborg.com | OIDC auth |
| Admin Portal | app.portofaalborg.com/admin | OIDC + MFA + admin allowlist |
| Public | share.portofaalborg.com/* | Public bypass |

---

## Security Zone Breakdown

### Zone 1: Admin (Highest Security)
**Paths/Domains to protect:**
- Admin portal subdomain (e.g., fileshareadmin.portofaalborg.com)
- API paths: `/api/v1/admin/*`, `/api/v1/audit/*`

**Cloudflare Access Policy:**
- Require OIDC authentication
- Require MFA
- Email allowlist: admin1@portofaalborg.com, admin2@portofaalborg.com
- OR OIDC group: `tenzoshare-admins`
- Session: 4 hours

---

### Zone 2: Authenticated Users (Standard Security)
**Paths/Domains to protect:**
- User portal subdomain (e.g., fileshare.portofaalborg.com)
- API paths: `/api/v1/auth/*`, `/api/v1/users/*`, `/api/v1/transfers/*`, `/api/v1/files/*`

**Cloudflare Access Policy:**
- Require OIDC authentication
- Email domain: @portofaalborg.com (or allow any authenticated user)
- Session: 24 hours
- MFA: optional (rely on IdP policy)

**User Flow:**
1. User goes to fileshare.portofaalborg.com
2. Cloudflare challenges with OIDC login
3. User logs in with company credentials
4. TenzoShare auto-provisions user if OIDC_AUTO_PROVISION=true
5. TenzoShare issues JWT for API calls

---

### Zone 3: Public (No Auth)
**Paths/Domains to protect:**
- Download UI (e.g., download.portofaalborg.com)
- Upload UI (e.g., upload.portofaalborg.com)
- API paths: `/api/v1/t/*` (transfer downloads), `/api/v1/r/*` (guest uploads)

**Cloudflare Access Policy:**
- Bypass (no authentication)
- Rate limiting: 100 req/min for downloads, 10 req/hour for uploads
- WAF enabled
- Bot protection enabled

**Note:** TenzoShare still enforces password protection on transfers if set by sender

---

## Quick Setup Example: portofaalborg.com

### 1. DNS Records (all proxied ☁️)
```
fileshare.portofaalborg.com       CNAME  <tunnel-id>.cfargotunnel.com
fileshareadmin.portofaalborg.com  CNAME  <tunnel-id>.cfargotunnel.com
download.portofaalborg.com        CNAME  <tunnel-id>.cfargotunnel.com
upload.portofaalborg.com          CNAME  <tunnel-id>.cfargotunnel.com
```

### 2. Cloudflare Access Applications

**Admin Zone:**
- Application name: TenzoShare Admin
- Domain: fileshareadmin.portofaalborg.com
- Policy: Allow → Emails: [admin list] → Require MFA

**User Zone:**
- Application name: TenzoShare Users
- Domain: fileshare.portofaalborg.com
- Policy: Allow → Email domain: portofaalborg.com

**Public Zone:**
- Application name: TenzoShare Public
- Domains: download.portofaalborg.com, upload.portofaalborg.com
- Policy: Bypass

### 3. TenzoShare Config (.env)
```bash
BASE_URL=https://fileshare.portofaalborg.com
OIDC_ENABLED=true
OIDC_ISSUER_URL=https://your-idp.com/
OIDC_CLIENT_ID=tenzoshare-client
OIDC_REDIRECT_URL=https://fileshare.portofaalborg.com/auth/callback
OIDC_AUTO_PROVISION=true
CF_ACCESS_ENABLED=true
CORS_ALLOWED_ORIGINS=https://fileshare.portofaalborg.com,https://fileshareadmin.portofaalborg.com,https://download.portofaalborg.com,https://upload.portofaalborg.com
```

---

## Alternative: Single Domain Setup

If you only want to use `fileshare.portofaalborg.com`:

**Option A: Different ports exposed directly**
- fileshare.portofaalborg.com → User portal (3000)
- fileshare.portofaalborg.com:3001 → Admin portal
- fileshare.portofaalborg.com:3003 → Download UI

**Problem:** Cloudflare Access works per-hostname, not per-port. All traffic to `fileshare.portofaalborg.com` gets same policy.

**Option B: Path-based routing with multiple Access apps**
- fileshare.portofaalborg.com/ → User portal
- fileshare.portofaalborg.com/admin → Admin (Access app with path rule)
- fileshare.portofaalborg.com/download/* → Public

**Challenge:** Requires Traefik path rewrites, more complex to maintain.

**Recommendation:** Use separate subdomains (Option 1) for cleaner security boundaries.
