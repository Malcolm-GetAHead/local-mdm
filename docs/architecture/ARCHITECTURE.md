# Architecture Documentation

**Version**: 1.0  
**Last Updated**: 2026-04-20

## System Architecture

Local MDM follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Layer (API)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   REST API   │  │  Platform    │  │  Dashboard   │  │
│  │  Endpoints   │  │  Endpoints   │  │  (HTML/HTMX) │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│                   Service Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │    Auth      │  │   Device     │  │   Policy     │  │
│  │   Service    │  │   Service    │  │   Service    │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│                 Repository Layer                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │    User      │  │   Device     │  │   Policy     │  │
│  │  Repository  │  │  Repository  │  │  Repository  │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│                   Database (PostgreSQL)                  │
└─────────────────────────────────────────────────────────┘
```

## Component Overview

### HTTP Layer (`internal/api`)

**Responsibilities**:
- HTTP request/response handling
- Routing
- Middleware (logging, CORS, authentication)
- Request validation
- Response formatting

**Key Files**:
- `server.go` - Server setup, routing, middleware
- `handlers.go` - Shared handler helpers (parseJSON, pagination, audit)
- `handlers_device.go`, `handlers_policy.go`, `handlers_enterprise.go`, `handlers_app.go`, `handlers_user.go`, `handlers_report.go`, `handlers_compliance.go`, `handlers_group.go`, `handlers_command.go`, `handlers_health.go` - Domain-specific HTTP handlers
- `web_handlers.go`, `web_handlers_pages.go` - Dashboard HTML handlers (device list, policy CRUD, groups, compliance, audit)
- `web_session.go` - OIDC login/callback, CSRF, HMAC session cookies
- `web_templates.go` - Template engine with `embed.FS`, helper functions
- `web_charts.go` - SVG pie chart generator
- `web_policy_catalog.go` - Settings catalog for policy forms
- `platform_handlers.go` - Platform enrollment and webhook handlers
- `ratelimit.go`, `auth_ratelimit.go`, `compression.go`, `idempotency.go` - Middleware

### Service Layer (`internal/service`)

**Responsibilities**:
- Business logic
- Transaction management
- Cross-repository operations
- Policy enforcement
- Event publishing

**Services**:
- `policy_service.go` - Policy management, versioning, templates, translation
- `group_service.go` - Device group management and membership
- `compliance_service.go` - Compliance evaluation and reporting
- `lifecycle_service.go` - Device lifecycle hooks (unenroll, wipe, delete)
- `device.go` - Device operations (lock, wipe, restart)
- `app.go` - App catalog and deployment
- `user.go` - User management with role validation
- `token.go` - API token create/validate/revoke

### Repository Layer (`internal/repository`)

**Responsibilities**:
- Database operations (CRUD)
- Query building
- Data mapping
- Transaction support

**Repositories**:
- `enterprise.go` - Enterprise operations
- `device.go` - Device operations
- `policy.go` - Policy operations
- `command.go` - Device command queue
- `certificate.go` - Certificate operations
- `audit_log.go` - Audit log operations
- `app.go` - App catalog operations
- `group.go` - Device groups and policy assignments
- `compliance.go` - Compliance results
- `policy_version.go` - Policy version snapshots
- `user.go` - User operations
- `token.go` - API token operations
- `policy_version.go` - Policy version snapshots

### Platform Modules (`internal/platform`)

**Responsibilities**:
- Platform-specific protocol implementation
- Device communication
- Command translation

**Modules**:

#### Windows (`internal/platform/windows`)
- `discovery.go` - MS-MDE2 discovery service
- `enrollment.go` - Device enrollment
- `management.go` - OMA-DM sync handler
- `csp/` - Configuration Service Providers

#### macOS (`internal/platform/macos`)
- `nanomdm.go` - nanoMDM integration
- `enrollment.go` - Profile generation
- `commands.go` - MDM command handlers
- `apns.go` - APNs integration

#### Android (`internal/platform/android`)
- `client.go` - Android Management API client
- `enrollment.go` - Token generation
- `policy.go` - Policy translation
- `webhook.go` - Event handler

### Supporting Packages

#### Configuration (`internal/config`)
- YAML configuration loading
- Environment variable overrides
- Configuration validation
- Reader pool config with field-level fallback

#### Database (`internal/db`)
- Writer/Reader connection pool management (Sprint 4b)
- Writer pool: all writes, transactions, non-repo consumers
- Reader pool: repository read queries (falls back to writer DSN if no reader config)
- Health checks (both pools)
- Migration support

#### Models (`internal/models`)
- Data structures
- Database mapping
- Constants and enums

#### Authentication (`internal/auth`)
- JWT token generation/validation
- Password hashing
- API key management
- RBAC enforcement

#### Certificates (`internal/certs`)
- CA certificate management
- Device certificate signing
- Certificate revocation
- APNs certificate handling

#### Audit (`internal/audit`)
- Async audit logging with buffered channel
- Enterprise-scoped audit trail

#### Metrics (`internal/metrics`)
- Prometheus metrics server on separate internal port (127.0.0.1:9090)
- HTTP request metrics middleware
- Custom business metrics

#### SCEP (`internal/scep`)
- SCEP challenge management for device certificate enrollment

#### Tracing (`internal/tracing`)
- Request tracing with unique request IDs

#### Application Errors (`internal/apperrors`)
- Structured application error types

#### Reporting (`internal/reporting`)
- Device inventory, compliance summary, enrollment trend reports
- CSV and JSON export

#### Constants (`internal/constants`)
- Shared constants (timeouts, limits, defaults)

#### Logging (`internal/logging`)
- Structured logging helpers

#### Validation (`internal/validation`)
- Input validation (JSONB size/depth, pagination parameters, sanitization)

#### Test Utilities (`internal/testutil`)
- Test helpers (database setup, common fixtures)

## Sprint 5: Service Layer Additions

Sprint 5 expanded the service layer (`internal/service/`) with new services and enhanced existing ones. All services follow the same pattern: accept repository interfaces via constructor, transport-agnostic (no `net/http`), reusable from handlers, CLI, and background jobs.

### DeviceService

Full device CRUD with integrated lifecycle hooks. Lock, wipe, and restart actions go through the service layer, which coordinates with the lifecycle hook system (cleanup of policies, group memberships, and compliance results on unenroll/wipe/delete).

### AppService

Application catalog CRUD plus deployment. `Deploy()` translates app installs into platform-specific commands via the command dispatcher — InstallApplication for macOS, App CSP SyncML for Windows, app policy updates for Android.

### UserService

User CRUD with role validation. Enforces valid role values (`super_admin`, `admin`, `operator`, `viewer`) and prevents privilege escalation (operators cannot create admins). Password hashing uses bcrypt.

### TokenService

API token lifecycle: create, validate, and revoke. Tokens use a `lmdm_` prefix for easy identification in logs and config files. The plaintext token is returned once at creation; only the SHA-256 hash is stored in the `api_tokens` table. Validation is a constant-time hash comparison.

### ComplianceService (Enhanced)

`evaluatePolicy()` now contains real evaluation logic (previously returned "unknown"). Evaluates device state against policy rules — checks OS version requirements, encryption status, password policy compliance, and app installation state. Results are written to `compliance_results` with detailed findings in the JSONB `details` column.

### ReportingService

Generates enterprise-scoped reports:
- **Device inventory**: counts by platform and status, with optional CSV export
- **Compliance summary**: aggregated compliance state across all devices and policies
- **Enrollment report**: enrollment trends over a configurable time window (default 30 days), broken down by platform and day

### EventBus (Sprint 5b)

PostgreSQL `LISTEN`/`NOTIFY` event bus using `pq.Listener`. Decouples event producers (database triggers) from consumers (compliance evaluation, lifecycle cleanup).

**Architecture**:
- 10 PostgreSQL triggers fire on a single `mdm_events` channel with JSON payload `{type, id, device_id, table, op, extra}`
- `pq.Listener` maintains a dedicated connection (not from the pool) with automatic reconnection and 30s keepalive pings
- Pre-flight `sql.Open` + `Ping` before creating the listener prevents uncontrollable reconnect goroutines on bad DSN
- Subscribers registered at startup, dispatched by event type. Fire-and-forget — errors logged, don't stop the bus
- Multi-instance safe: PostgreSQL NOTIFY is fan-out (all server instances receive all events). Subscribers must be idempotent.

**Subscribers**:
- `device.enrolled` / `device.info_updated` → compliance auto-evaluation
- `policy.updated` / `policy.assigned` / `policy.unassigned` → re-evaluate affected devices
- `group.member_added` / `group.member_removed` → re-evaluate device compliance
- `ComplianceCleanupHook` → clear compliance results on unenroll/wipe/delete

**Data flow**: Device checks in → handler updates `platform_data` → PostgreSQL trigger fires `device.info_updated` → EventBus dispatches → compliance subscriber evaluates device → results written to `compliance_results` → dashboard shows live compliance state.

## Web Dashboard (Sprint 5d)

The dashboard is a server-rendered web UI using Go HTML templates, HTMX, and Tailwind CSS. No separate frontend build pipeline — CSS is compiled in the Dockerfile.

### Stack
- **Go `html/template`** with `embed.FS` — templates compiled into the binary
- **HTMX v2.0.9** — partial page updates (table filters, member toggles, device actions)
- **Tailwind CSS v4.2.4** — compiled via standalone CLI in Dockerfile
- **CSP nonces** — URL-safe base64, all JS in external `app.js` (no inline scripts)

### SPA-like Navigation
Sidebar links use `hx-get` targeting `#page-content` (wraps header + content area). The server returns just the header+content fragment for HTMX requests (`HX-Target: page-content`), preserving the sidebar and `app.js`. Table filter HTMX requests (`HX-Target` is a table div) return only the table fragment.

### Key Files
```
internal/api/
├── web_handlers.go          # Dashboard home, device list/detail/actions
├── web_handlers_pages.go    # Policy, group, compliance, audit handlers
├── web_session.go           # OIDC login/callback, CSRF, HMAC session cookies
├── web_templates.go         # Template engine, helper functions
├── web_charts.go            # SVG pie chart generator
├── web_policy_catalog.go    # Settings catalog for policy forms
├── templates/
│   ├── base.html            # Layout shell (sidebar, header, content wrapper)
│   ├── partials/            # Reusable fragments (header, sidebar, pagination)
│   └── pages/               # Page templates (dashboard, devices, policies, etc.)
web/static/
├── js/htmx.min.js           # Vendored HTMX
├── js/app.js                # All page JS (event delegation, survives content swaps)
├── css/output.css           # Compiled Tailwind
└── favicon.svg
```

### Authentication
Keycloak OIDC code flow → HMAC-SHA256 signed session cookie. CSRF protection on POST forms (HTMX requests exempt). Dedicated `session_secret` config key with fallback to Keycloak client secret.

## Data Flow

### Dashboard Navigation Flow

```
Full page load (first visit or hard refresh):
  Browser → GET /dashboard/devices → Server renders base.html + header + content → Full HTML page

HTMX sidebar navigation (subsequent clicks):
  Browser → GET /dashboard/policies (HX-Target: page-content) → Server renders header + content fragment only → HTMX swaps #page-content innerHTML

HTMX sub-page navigation (hx-boost on content links):
  Browser → GET /dashboard/devices/{id} (HX-Target: page-content) → Same fragment response → Sidebar stays, content swaps

HTMX table filter (search/filter inputs):
  Browser → GET /dashboard/devices?q=mac (HX-Target: device-table) → Server renders table body fragment only → HTMX swaps table div
```

### Device Enrollment Flow

```
1. Admin creates enrollment token
   API → EnrollmentService → DeviceRepository → DB

2. Device initiates enrollment
   Device → Platform Endpoint → EnrollmentService

3. Server validates and issues certificate
   EnrollmentService → CertService → CertRepository → DB

4. Device registered
   EnrollmentService → DeviceRepository → DB

5. Policies applied
   EnrollmentService → PolicyService → DeviceRepository → DB
```

### Policy Application Flow

```
1. Admin creates policy
   API → PolicyService → PolicyRepository → DB

2. Admin assigns to devices
   API → PolicyService → DeviceRepository → DB

3. Device checks in
   Device → Platform Endpoint → DeviceService

4. Server sends policy
   DeviceService → PolicyService → Platform Module → Device

5. Device reports status
   Device → Platform Endpoint → DeviceService → DB
```

## Security Architecture

### Authentication Flow

```
1. User login
   POST /api/v1/auth/login
   → AuthService validates credentials
   → Generate JWT access + refresh tokens
   → Return tokens

2. API request
   Authorization: Bearer <access_token>
   → Middleware validates JWT
   → Extract user context
   → Check permissions
   → Allow/deny request

3. Token refresh
   POST /api/v1/auth/refresh
   → Validate refresh token
   → Generate new access token
   → Return new token
```

### Device Authentication

```
1. Enrollment
   → Server issues client certificate
   → Device stores certificate

2. Subsequent requests
   → Device presents certificate
   → Server validates certificate
   → Check revocation status
   → Allow/deny request
```

### Authorization Model

**Role-Based Access Control (RBAC)**:

```
super_admin
  ├─ Manage all enterprises
  ├─ System configuration
  └─ User management

admin (per enterprise)
  ├─ Manage devices
  ├─ Manage policies
  ├─ Manage users
  └─ View audit logs

operator (per enterprise)
  ├─ Manage devices
  ├─ Manage policies
  └─ View audit logs

viewer (per enterprise)
  ├─ View devices
  ├─ View policies
  └─ View audit logs
```

### Dashboard Security (Sprint 5d)
- **Session**: HMAC-SHA256 signed cookies, 8-hour TTL, dedicated `session_secret` config key
- **CSRF**: HMAC token on all POST forms, HTMX requests exempt (same-origin)
- **CSP**: Nonces for external scripts (`app.js`, `htmx.min.js`), no `unsafe-inline`
- **Auth**: Keycloak OIDC code flow, enterprise_id from JWT claim

## Database Design Principles

### Multi-Tenancy

- All resources scoped to `enterprise_id`
- Row-level isolation
- Shared schema approach
- Future: Schema-per-tenant option

### Soft Deletes

- All main tables have `deleted_at` column
- Queries filter `WHERE deleted_at IS NULL`
- Allows data recovery
- Audit trail preservation

### JSONB Usage

- Flexible policy configuration
- Platform-specific device data
- Settings and metadata
- Indexed with GIN for queries

### Timestamps

- `created_at` - Record creation
- `updated_at` - Last modification (auto-updated via trigger)
- `deleted_at` - Soft delete timestamp

## Scalability Considerations

### Horizontal Scaling

- Stateless API servers
- JWT-based authentication (no session state)
- Database connection pooling
- Load balancer ready

### Database Scaling

- Read replicas supported via Writer/Reader pool split (Sprint 4b)
- Connection pooling with configurable limits per pool
- Prepared statements
- Index optimization

### Caching Strategy

- PostgreSQL-backed token cache with TTL (Sprint 4, replaced Redis)
- PostgreSQL-backed idempotency key cache with 24h TTL (Sprint 4)
- No external cache dependency — PostgreSQL handles all caching needs at current scale

## Monitoring & Observability

### Health Checks

```
GET /health
- Database connectivity
- Certificate expiration
- APNs connectivity (macOS)
- Android API connectivity
```

### Metrics

- Request rate and latency (Prometheus on 127.0.0.1:9090)
- Device enrollment rate
- Policy application success rate
- Database query performance
- Certificate expiration alerts

### Logging

- Structured JSON logging
- Request/response logging
- Error tracking
- Audit logging

## Deployment Architecture

### Development

```
Docker Compose (all services on Alpine Linux)
├── localmdm        — Go API server + web dashboard (port 8080)
├── nanomdm         — Apple MDM protocol handler (port 9000)
├── postgres        — PostgreSQL 15 (databases: localmdm, keycloak, nanomdm)
├── keycloak        — OIDC identity provider (port 8180)
├── adminer         — Database UI (port 8081)
└── test-runner     — go test with race detector, mdmb installed
```

### Production (Recommended)

```
Load Balancer (TLS termination)
    │
    ├─ MDM Server Instance 1
    ├─ MDM Server Instance 2
    └─ MDM Server Instance 3
         │
         └─ PostgreSQL (Primary + Replicas)
```

## Technology Choices

### Why Go?

- Strong standard library (crypto, HTTP, XML)
- Excellent concurrency support
- Single binary deployment
- Good performance
- Strong typing for protocol implementation

### Why PostgreSQL?

- JSONB for flexible schemas
- ACID compliance
- Excellent query performance
- Wide deployment support
- Strong community

### Why gorilla/mux?

- Simple and powerful routing
- Middleware support
- Well-documented
- Battle-tested

### Why HTMX + Go Templates (not React)?

- No separate frontend build pipeline — CSS compiled in Dockerfile
- Server-rendered HTML with progressive enhancement
- HTMX handles partial updates (table filters, member toggles, SPA navigation)
- Go `html/template` with `embed.FS` — templates compiled into binary
- Tailwind CSS v4 standalone CLI — no Node.js in production

## External Dependencies

Local MDM integrates with several micromdm projects for Apple device management. Full documentation is in [docs/dependencies/](../dependencies/).

| Dependency | Integration Point | Purpose |
|---|---|---|
| [NanoMDM](../dependencies/nanomdm/) | `internal/platform/macos` | Apple MDM protocol — enrollment, commands, push notifications |
| [NanoDEP](../dependencies/nanodep/) | `internal/platform/macos` | Apple DEP/ADE — automated device enrollment via ABM/ASM |
| [SCEP](../dependencies/scep/) | `internal/certs` | Certificate enrollment for device identity certificates |
| [NanoLIB](../dependencies/nanolib/) | transitive | Shared utilities used by NanoMDM and NanoDEP |
| [Keycloak](../dependencies/keycloak/) | `internal/auth` + macOS profiles | OIDC IdP for admin auth + macOS Platform SSO via UiO extensions |

### Integration Architecture

```
Local MDM Control Plane
├── Auth Layer
│   └── Keycloak (OIDC) ──────── Admin/API authentication + RBAC
├── macOS Module
│   ├── NanoMDM (Go library) ─── MDM check-ins, commands, APNs push
│   ├── NanoDEP (Go library) ─── DEP token mgmt, device sync, profile assignment
│   ├── SCEP server ──────────── Device identity certificate enrollment
│   └── Platform SSO profile ─── Configures Keycloak SSO on managed Macs
│       ├── Keycloak PSSO Extension (server-side)
│       └── Weblogin SSO Extension (on-device)
├── Windows Module (custom OMA-DM/MS-MDE2)
└── Android Module (Google Management API)
```

### Database Architecture

The PostgreSQL instance hosts three separate databases:

- **`localmdm`** — Local MDM application tables (devices, policies, users, etc.)
- **`nanomdm`** — NanoMDM's own tables (enrollments, commands, push certs). Separate database to avoid table name conflicts (both have `devices` and `users` tables).
- **`keycloak`** — Keycloak identity provider tables

NanoDEP uses the `localmdm` database (its tables have a `dep_` prefix, no conflicts).

### NanoMDM Service

NanoMDM runs as a **separate Docker service** (not a Go library embedded in Local MDM). It handles the raw Apple MDM protocol (plist XML, CMS signatures) and forwards events to Local MDM via JSON webhooks.

- **Devices → NanoMDM** (port 9000): `/checkin`, `/mdm` — Apple MDM protocol
- **NanoMDM → Local MDM** (port 8080): `POST /api/v1/macos/webhook` — JSON webhook
- **Local MDM → NanoMDM**: HTTP API for sending commands to devices

In production (ECS Fargate), NanoMDM runs as a separate ECS service behind the ALB with path-based routing.

## Extension Points

### Adding New Platforms

1. Create module in `internal/platform/{platform}/`
2. Implement enrollment interface
3. Implement management interface
4. Add routes in `internal/api/server.go`
5. Update policy translator

### Adding New Policy Types

1. Define policy type constant
2. Add to policy config schema
3. Implement platform-specific translator
4. Add validation logic
5. Update API documentation

### Adding New CSPs (Windows)

1. Create CSP handler in `internal/platform/windows/csp/`
2. Implement SyncML message handling
3. Add to CSP registry
4. Add tests

## Testing Strategy

### Unit Tests

- Service layer logic
- Repository operations
- Policy translation
- Certificate operations

### Integration Tests

- API endpoints
- Database operations
- Platform modules

### End-to-End Tests

- Device enrollment flow
- Policy application
- Device commands

### Browser Tests (Sprint 5d)

- Playwright playbook DSL (`tests/browser/browser-playbook.md`)
- 199 tests covering: login, navigation, CRUD workflows, tab switching, dark mode, mobile
- Real Keycloak OIDC login (no cookie bypass)
- Console error tracking (JS errors, HTTP 4xx/5xx fail the run)
- Run: `make seed && make browser-test`

### Test Coverage Goals

- Service layer: 80%+
- Repository layer: 70%+
- API layer: 48%+ (web handlers tested via Playwright + Go unit tests)
- Overall: 70%+

---

**Current Sprint**: 6 (Real Device Integration) — in progress

## Sprint 6: Real Device Integration

### nginx TLS Proxy

nginx reverse proxy terminates TLS on ports 443/8443, forwarding to the Go server on port 8080. The server cert is signed by the project CA and includes SANs for `192.168.1.102` and `enterpriseenrollment.localmdm.local`. CA certs persist via Docker volume mount (`./internal/api/certs:/app/certs`) — without this mount, every `docker compose build` regenerates the CA and breaks all enrolled devices.

```
Device (HTTPS) → nginx:443 → localmdm:8080 (HTTP)
                  ↓ TLS termination
                  Server cert signed by project CA
                  CA cert trusted on VMs
```

### NanoMDM Webhook Data Pipeline

NanoMDM forwards all check-in and command result events to Local MDM via webhook (`POST /api/v1/macos/webhook`). The `CheckinHandler` processes these events:

```
Device check-in → NanoMDM → webhook JSON → CheckinHandler
  ├── Authenticate: create/update device (name, serial, model, OS from plist)
  ├── TokenUpdate: set status=enrolled, store push_magic
  ├── CheckOut: set status=unenrolled, fire lifecycle hooks
  └── Acknowledge: parse command results → update platform_data
```

Command results (`processCommandResult`) parse base64-encoded plist responses:
- **SecurityInfo** → FileVault, firewall, secure boot, activation lock → compliance evaluation
- **DeviceInformation** → 35 fields (name, OS, storage, battery, network, update settings)
- **ProfileList** → installed profiles with payload counts
- **InstalledApplicationList** → apps with versions and bundle sizes
- **CertificateList** → certificates (common name, identity flag)
- **UserList** → local users with admin/secure token status
- **AvailableOSUpdates / OSUpdateStatus** → pending updates

### Auto-Queue Flow

On `Idle` status (no pending commands), the handler auto-queues 9 info commands with a 15-minute per-device cooldown to prevent storm cycles:

```
Device reports Idle → maybeAutoQueue(udid)
  ├── Check cooldown map (15min per device)
  ├── If cooldown elapsed:
  │   ├── Create 9 command records in Local MDM DB
  │   ├── Send 9 plist commands to NanoMDM API
  │   └── Update cooldown timestamp
  └── If within cooldown: skip

Commands: SecurityInfo, DeviceInformation (35 queries), ProfileList,
InstalledApplicationList, CertificateList, ManagedApplicationList,
AvailableOSUpdates, OSUpdateStatus, UserList
```

## Future: Network Architecture & VPN Deployment

### Design Principle

Minimize internet-facing attack surface. Expose only MDM enrollment and sync endpoints publicly. Route all other management traffic over a device VPN provisioned by MDM itself.

### Network Zones

```
Internet-facing (minimal surface):
  ├── MDM enrollment (Discovery, Policy, Enrollment)
  ├── MDM sync (OMA-DM for Windows, Apple MDM check-in for macOS)
  └── WireGuard VPN endpoint

VPN-only (management traffic):
  ├── Software deployment / package distribution
  ├── Remote administration (SSH, RDP)
  ├── Monitoring and telemetry collection
  ├── Internal application access
  └── Extended profile/policy delivery
```

### Enrollment → VPN Bootstrap Chain

```
1. Device enrolls via MDM (internet-facing endpoints)
2. MDM pushes WireGuard VPN profile as first policy action
3. Device connects to VPN automatically
4. All subsequent management flows over VPN
5. Internet-facing MDM endpoints remain for re-enrollment and periodic sync only
```

### Per-Platform VPN Delivery

- **macOS**: MDM pushes `.mobileconfig` with VPN payload type `com.wireguard.ios` — native WireGuard support, no agent required
- **Windows**: OMA-DM pushes VPN config via `VPNv2` CSP — WireGuard supported natively in Windows 11
- **Per-device keys**: Each device gets a unique WireGuard keypair generated at enrollment time, pushed via MDM profile

### Device Provisioning (First Touch)

| Scenario | Manual Steps | Post-Enrollment |
|----------|-------------|-----------------|
| macOS + ABM | Zero-touch (DEP enrollment) | WireGuard auto-pushed |
| macOS without ABM | User installs enrollment profile via Safari | WireGuard auto-pushed |
| Windows + manufacturer image | PPKG or bootstrap script in image | WireGuard auto-pushed |
| Windows manual | IT runs bootstrap script (CA cert + hosts + open enrollment) | WireGuard auto-pushed |

### Bootstrap Script (Windows)

One-time provisioning script for IT to run during device setup — not a persistent agent:

```powershell
# Install CA cert, configure DNS, open enrollment UI
certutil -addstore Root \\share\ca.crt
Add-Content C:\Windows\System32\drivers\etc\hosts "mdm.company.com enterpriseenrollment.company.com"
Start-Process "ms-device-enrollment:?mode=mdm"
```

### Software Distribution

Package managers deployed via MDM as part of the first-sync policy:

- **Windows**: Chocolatey — installed via OMA-DM script CSP, packages served from internal repo over VPN
- **macOS**: Munki — installed via MDM profile, catalog and packages served from internal repo over VPN

Package repositories live inside the VPN network only — devices must be VPN-connected to install/update software. This prevents package tampering and ensures only managed devices can access internal packages.

### Remote Access (RustDesk)

Self-hosted RustDesk for IT remote support. Internet-facing so IT can reach devices even when VPN is down.

```
Internet-facing:
  └── RustDesk relay/rendezvous server (encrypted P2P relay only)

Managed devices:
  └── RustDesk client (pushed via MDM, locked to our relay server)
```

**Deployment**: MDM pushes RustDesk client as a managed application on first sync. Client is pre-configured with the organization's relay server address — no user configuration needed.

**Security**: RustDesk client connections are end-to-end encrypted. The relay server only facilitates NAT traversal, it cannot inspect traffic.

### Device Firewall Policy

MDM pushes host firewall rules to lock down outbound connectivity. Only approved destinations are allowed for management services:

**Windows** (via Firewall CSP):
```
Outbound allow:
  ├── MDM server (enrollment + sync)     → mdm.company.com:443
  ├── WireGuard VPN endpoint             → vpn.company.com:51820/udp
  ├── RustDesk relay server              → rustdesk.company.com:21115-21119
  └── Standard traffic (HTTP/S, DNS)     → any (or corporate proxy)

Outbound deny:
  └── RustDesk to any OTHER relay server → block rustdesk ports to non-approved IPs
```

**macOS** (via configuration profile, pf/packet filter):
```
Same rules as Windows, delivered via MDM profile payload.
```

**Key principle**: RustDesk client is firewalled so it can ONLY connect to the organization's relay server. This prevents:
- Attacker installing their own RustDesk relay and hijacking remote access
- Unauthorized remote access via public RustDesk infrastructure
- Data exfiltration over RustDesk tunnels to external relays

### Operational Considerations

**DNS**: Production requires public DNS for `enterpriseenrollment.<domain>` → MDM server (enrollment auto-discovery). Internal DNS over VPN for all management services (package repos, monitoring, internal apps). Eliminates hosts file entries.

**Split DNS via WireGuard**: WireGuard client config pushes a DNS server accessible only over the VPN tunnel. Devices resolve internal hostnames (package repos, monitoring, internal apps) via VPN DNS, and everything else via their normal DNS. This enables:
- Internal service discovery without public DNS records
- Access to internal resources by hostname (e.g., `packages.internal`, `monitoring.internal`)
- DNS-based access control — internal names only resolve for VPN-connected managed devices
- WireGuard `DNS` and `AllowedIPs` config controls which traffic routes through the tunnel (split tunnel — only internal ranges, not all traffic)

**Certificate lifecycle**: CA cert and server certs expire. Plan for automated renewal and MDM-pushed CA cert updates before expiry. Devices that miss a CA rotation lose trust and can't sync — need a re-enrollment path.

**Device offboarding**: When a device is unenrolled, wiped, or deleted, automatically revoke its WireGuard key on the VPN server and remove RustDesk access. The EventBus lifecycle hooks (`ComplianceCleanupHook` pattern) can trigger this — add a `VPNCleanupHook` and `RemoteAccessCleanupHook` subscriber.

**Stale device alerting**: Devices that miss MDM sync for a configurable threshold (e.g., 24 hours) should trigger an alert. Could indicate stolen device, network issue, or tampered MDM client. EventBus + a periodic check job can implement this. Alert channels: email, webhook to Slack/Teams, dashboard notification.

**Break-glass access**: If the MDM server goes down, IT loses the ability to push new configs. Mitigations:
- RustDesk relay is independent of MDM — remote access survives MDM outage
- WireGuard configs are persistent on-device — VPN survives MDM outage
- Pre-provision a static break-glass VPN config that doesn't depend on MDM for renewal
- Document manual recovery procedures for MDM server restore
