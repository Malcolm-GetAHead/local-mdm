# Architecture Documentation

**Version**: 1.0  
**Last Updated**: 2026-02-05

## System Architecture

Local MDM follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Layer (API)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   REST API   │  │  Platform    │  │   Webhooks   │  │
│  │  Endpoints   │  │  Endpoints   │  │              │  │
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
- `server.go` - Server setup and routing
- `handlers.go` - HTTP handlers
- `middleware.go` - Middleware functions (future)

### Service Layer (`internal/services`)

**Responsibilities**:
- Business logic
- Transaction management
- Cross-repository operations
- Policy enforcement
- Event publishing

**Planned Services**:
- `auth/` - Authentication and authorization
- `device/` - Device management
- `policy/` - Policy management
- `enrollment/` - Device enrollment flows
- `command/` - Device command execution

### Repository Layer (`internal/repositories`)

**Responsibilities**:
- Database operations (CRUD)
- Query building
- Data mapping
- Transaction support

**Planned Repositories**:
- `user.go` - User operations
- `device.go` - Device operations
- `policy.go` - Policy operations
- `certificate.go` - Certificate operations
- `audit.go` - Audit log operations

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

#### Database (`internal/db`)
- Connection management
- Health checks
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

#### Webhooks (`internal/webhooks`)
- Webhook registration
- Event publishing
- Delivery retry logic

## Data Flow

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

- Read replicas for reporting
- Connection pooling
- Prepared statements
- Index optimization

### Caching Strategy (Future)

- Redis for session data
- Device status cache
- Policy cache
- Certificate cache

## Monitoring & Observability

### Health Checks

```
GET /health
- Database connectivity
- Certificate expiration
- APNs connectivity (macOS)
- Android API connectivity
```

### Metrics (Future)

- Request rate and latency
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
Developer Machine
├── Docker Compose
│   ├── PostgreSQL
│   └── Adminer
└── Go Application (local)
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

### Test Coverage Goals

- Service layer: 80%+
- Repository layer: 70%+
- Overall: 70%+

---

**Next Steps**:
1. Implement service layer
2. Add repository layer
3. Implement authentication
4. Begin Windows module
