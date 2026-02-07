# Keycloak Integration Notes for Local MDM

## Two Integration Surfaces

### 1. Control Plane Authentication (Admin/API)

Keycloak replaces custom JWT auth for the Local MDM dashboard and REST API.

- Local MDM acts as an OIDC Relying Party (client) to Keycloak
- Admin users authenticate via Keycloak login
- API tokens issued by Keycloak (or exchanged for Local MDM JWTs)
- RBAC roles (super_admin, admin, operator, viewer) can map to Keycloak realm/client roles
- Multi-tenant enterprise isolation can leverage Keycloak organizations or realm-per-tenant

### 2. macOS Device SSO (Platform SSO)

Local MDM pushes MDM profiles that configure the Weblogin SSO Extension on managed Macs.

Flow:
```
1. Mac enrolls via MDM (NanoMDM handles enrollment)
2. Local MDM pushes Platform SSO configuration profile
3. SSO Extension registers device + user with Keycloak
4. Mac login window / app auth → SSO Extension → Keycloak (passwordless via Secure Enclave)
```

### MDM-Keycloak Device Lifecycle

The PSSO extension's device API (`/device/{serial}`) enables syncing device state:

- When a device enrolls in MDM → it can be pre-registered or allowed to self-register in Keycloak
- When a device is unenrolled/wiped → Local MDM calls Keycloak's device DELETE API to revoke PSSO registration
- Device serial numbers are the shared identifier between MDM and Keycloak

### What Local MDM Needs to Provide

1. **Keycloak OIDC client configuration** for admin auth
2. **MDM configuration profile** for Platform SSO (pushed to macOS devices)
3. **Device lifecycle hooks** — sync enrollment/unenrollment events with Keycloak device API
4. **`psso-admin` service account** — for calling Keycloak's device management API

### Database Considerations

Keycloak runs its own database (PostgreSQL supported). It can share the same PostgreSQL instance as Local MDM but should use a separate database/schema. The PSSO extension adds a custom `devices` table to Keycloak's schema.

### Deployment Topology

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │ (TLS termination)│
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼───┐  ┌──────▼─────┐  ┌─────▼──────┐
     │ Local MDM  │  │  Keycloak  │  │   SCEP     │
     │  Server    │  │  + PSSO    │  │   Server   │
     └─────┬──────┘  └─────┬──────┘  └────────────┘
           │               │
           └───────┬───────┘
              ┌────▼────┐
              │PostgreSQL│
              │(separate │
              │databases)│
              └─────────┘
```
