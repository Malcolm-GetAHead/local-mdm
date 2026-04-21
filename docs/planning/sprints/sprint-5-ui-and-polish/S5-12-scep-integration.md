# S5-12: SCEP Server Integration (Embedded)

**Sprint**: 5 — Backend Polish  
**Parallel**: ✅ Yes (should complete before S5-05 E2E Testing)  
**Depends on**: S1-03 (CA infrastructure — complete)  
**Effort**: 0.5-1 day

## Problem

The macOS enrollment profile hardcodes `scepURL := serverURL + "/scep"` but no `/scep` route exists. The SCEP server was planned in S1-03 but deferred as "not critical for MVP." Without it, real macOS device enrollment fails at the certificate enrollment step.

The SCEP challenge manager (`internal/scep/challenge.go`) stores challenges in an in-memory `map[string]*Challenge`. This breaks in multi-instance deployments — a challenge generated on instance A won't be found on instance B.

## Solution

Embed the `micromdm/scep` library as a Go dependency (no separate service). Register `/scep` GET+POST handlers that use the existing `CAManager` for CSR signing. Move challenge storage to PostgreSQL for cross-instance consistency.

## Tasks

### 1. Add SCEP Library Dependency
- `go get github.com/micromdm/scep/v2`
- Wire into existing `CAManager` for CA cert and key

### 2. Register `/scep` Route
- `GET /scep` — serve CA certificate (SCEP `GetCACert` operation)
- `POST /scep` — handle `PKCSReq` (CSR signing) with challenge validation
- No auth middleware — devices don't have tokens yet during enrollment

### 3. Migrate Challenge Store to PostgreSQL
- New migration: `scep_challenges` table

```sql
CREATE TABLE scep_challenges (
    password VARCHAR(64) PRIMARY KEY,
    device_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_scep_challenges_expires ON scep_challenges(expires_at);
```

- Update `ChallengeManager` to use database instead of in-memory map
- `GenerateChallenge`: INSERT into table
- `ValidateChallenge`: `SELECT ... WHERE password = $1 AND NOT used AND expires_at > NOW() FOR UPDATE`, then `UPDATE SET used = true`
- `CleanupExpired`: `DELETE WHERE expires_at < NOW()` (run on same hourly schedule as idempotency key cleanup)
- Uses Writer pool directly (same pattern as idempotency keys, token cache)

### 4. Update Enrollment Profile URL
- Verify `scepURL` in `platform_handlers.go` points to the correct path
- Ensure the SCEP endpoint is accessible from enrolling devices (no auth, public route)

## What This Enables

- Real macOS device enrollment (F-01)
- E2E enrollment testing (S5-05)
- Multi-instance deployment without sticky sessions (F-02)

## Acceptance Criteria

- [ ] `/scep` GET returns CA certificate in DER format
- [ ] `/scep` POST accepts PKCSReq, validates challenge, signs CSR, returns cert
- [ ] Challenge generated on one instance validates on another (PostgreSQL-backed)
- [ ] Expired challenges cleaned up automatically
- [ ] Existing SCEP challenge unit tests still pass
- [ ] macOS enrollment profile's SCEP URL resolves correctly

---

*Created 2026-04-21 — gap identified during Sprint 4b forward look.*
