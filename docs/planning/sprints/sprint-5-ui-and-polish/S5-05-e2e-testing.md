# S5-05: End-to-End Testing & Hardening

**Sprint**: 5 — UI & Polish
**Parallel**: ⚠️ Benefits from other S5 tasks (especially S5-09, S5-10)
**Depends on**: All platform tasks (Sprints 2-3), Sprint 4 (policy/compliance/groups)
**Effort**: 4-5 days

## Tasks

### 1. E2E Test Suite
Full lifecycle flows incorporating Sprint 4 policy and compliance:

- **macOS**: enroll → add to group → assign policy (via group) → push profile → check-in → compliance evaluation → verify compliant → lock → unenroll → lifecycle hooks fire
- **Windows**: enroll → assign policy (direct) → deploy WiFi CSP → check-in → compliance evaluation → verify inventory → wipe → lifecycle hooks fire
- **Android**: enroll → add to group → assign policy (via group) → deploy policy → install app → check-in → compliance evaluation → verify compliant
- **Cross-platform**: create policy template → clone to enterprise → assign to enterprise-wide → verify all platforms receive translated policy → rollback policy version → verify rollback applied

Files: `tests/e2e/`

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
