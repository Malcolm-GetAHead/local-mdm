# S5-05: End-to-End Testing & Hardening

**Sprint**: 5 — UI & Polish
**Parallel**: ⚠️ Benefits from other S5 tasks but can start with API tests
**Effort**: 4-5 days

## Tasks

### 1. E2E Test Suite
- macOS: enroll → push profile → verify compliance → lock → unenroll
- Windows: enroll → deploy WiFi CSP → verify inventory → wipe
- Android: enroll → deploy policy → install app → verify compliance
- Files: `tests/e2e/`

### 2. Security Hardening
- Input validation on all endpoints
- Rate limiting on enrollment and auth endpoints
- SQL injection prevention audit
- CORS configuration review
- TLS configuration review
- Files: review across codebase

### 3. Performance Testing
- Load test: 100 concurrent enrollments
- Load test: 1000 device inventory query
- Load test: 500 simultaneous OMA-DM sync sessions
- Identify and fix bottlenecks
- Files: `tests/load/`

### 4. Error Handling Review
- Consistent error responses across all endpoints
- No stack traces leaked to clients
- Graceful degradation when external services unavailable (Keycloak, Apple, Google)

## Acceptance Criteria

- [ ] E2E tests pass for all three platforms
- [ ] No critical security findings
- [ ] Performance targets met (< 2s API response, < 30s enrollment)
- [ ] Error responses consistent and informative
