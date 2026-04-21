# Repository Integration Test Coverage Review

**Date**: 2026-04-21  
**Scope**: All repository files in `internal/repository/` tested against live PostgreSQL  
**Overall Coverage**: 49.1% (target: 80%)

---

## Summary

Sprint 1/2 repositories have solid integration test coverage (80-100% on most methods). Sprint 3/4 repositories have **zero direct integration tests** — they were only tested indirectly through service-layer mocks in handler tests. This means the actual SQL queries, column mappings, and constraint handling for 4 repo files have never been validated against PostgreSQL.

PostgreSQL is available in the docker-compose stack and all existing integration tests run against it successfully. There is no infrastructure blocker — these tests simply weren't written.

---

## Per-File Coverage

### ✅ Well-Tested (Sprint 1/2 repos)

| File | Constructor | Create | Get | List | Update | Delete | Other | Notes |
|------|------------|--------|-----|------|--------|--------|-------|-------|
| device.go | 100% | 100% | 100% (ByID, BySerial) | 100% | 84.6% | 90.9% | GetByPlatformID: 0% | Solid. Only GetByPlatformID untested |
| enterprise.go | 85.7% | 100% | 100% (ByID, BySlug) | 100% | 83.3% | 90% | — | Solid |
| policy.go | 85.7% | 100% | 100% | 100% | 83.3% | 80% | Assign: 100%, Unassign: 100% | Solid |
| command.go | 85.7% | 100% | 0% (ByID) | 84.6% (Pending) | — | — | MarkSent/Completed/Failed: 100%, ListByDevice: 0% | Good. Two read methods untested |
| certificate.go | 85.7% | — | 0% (BySerial) | 94.7% | — | — | — | GetBySerial untested |
| audit_log.go | 85.7% | — | — | 46.2% | — | — | — | List partially tested |
| transaction.go | 100% | — | — | — | — | — | All helpers 82-100% | Solid |

### ❌ Zero Coverage (Sprint 3/4 repos)

| File | Methods | Lines | Sprint | Notes |
|------|---------|-------|--------|-------|
| **app.go** | NewAppRepository, Create, GetByID, List, Update, Delete | ~120 | Sprint 3 | App catalog CRUD. No integration tests at all |
| **compliance.go** | NewComplianceRepository, Upsert, GetByDevice, GetSummary | ~80 | Sprint 4 | Compliance results. No integration tests at all |
| **group.go** (GroupRepository) | NewGroupRepository, Create, GetByID, List, Update, Delete, AddMember, RemoveMember, ListMembers, ListGroupsForDevice | ~180 | Sprint 4 | Device groups + membership. No integration tests at all |
| **group.go** (PolicyAssignmentRepository) | NewPolicyAssignmentRepository, Create, Delete, ListByTarget, ListByPolicy, GetEffectivePolicies, scanAssignments | ~120 | Sprint 4 | Policy assignment targeting. No integration tests at all |
| **policy_version.go** | NewPolicyVersionRepository, Create, ListByPolicy, GetByVersion, LatestVersion | ~90 | Sprint 4 | Policy versioning/rollback. No integration tests at all |

**Total untested: ~590 lines of SQL-heavy code across 5 files (4 repo files, group.go has 2 repos)**

---

## What's At Risk

These repos contain SQL queries that have never been validated against the actual database schema:

1. **Column mapping errors** — Go struct fields may not match actual column names/types
2. **FK constraint violations** — Insert/update queries may reference columns or tables incorrectly
3. **JSONB handling** — compliance_results and policy_versions store JSONB data that may not serialize correctly
4. **Pagination** — List methods use `ExecutePaginatedQuery` which is tested generically but not with these specific queries
5. **Soft delete behavior** — group.go Delete may not handle `deleted_at` correctly
6. **Priority-based policy resolution** — `GetEffectivePolicies` has complex SQL with JOINs and ORDER BY priority that's never been tested against real data

The service-layer tests use mocks that return canned data, so they validate business logic but not SQL correctness.

---

## Recommended Test Plan

Each repo needs a test file following the existing pattern in `repository_test.go` and `new_repos_test.go`:

### app_integration_test.go
- [x] Create app with valid data
- [x] GetByID returns created app
- [x] List returns apps for enterprise (pagination)
- [x] Update modifies fields
- [x] Delete soft-deletes (not found after delete)
- [x] Create with duplicate name (if constrained)

### compliance_integration_test.go
- [x] Upsert creates new compliance result
- [x] Upsert updates existing result (same device + policy)
- [x] GetByDevice returns results for device
- [x] GetSummary returns enterprise-level counts

### group_integration_test.go
- [x] Create group
- [x] GetByID returns group
- [x] List groups for enterprise
- [x] Update group name/description
- [x] Delete group (soft delete)
- [x] AddMember adds device to group
- [x] RemoveMember removes device from group
- [x] ListMembers returns group members
- [x] ListGroupsForDevice returns groups a device belongs to
- [x] AddMember with duplicate device (idempotent or error?)

### policy_assignment_integration_test.go
- [x] Create assignment (device target)
- [x] Create assignment (group target)
- [x] Create assignment (enterprise target)
- [x] Delete assignment
- [x] ListByTarget returns assignments for a target
- [x] ListByPolicy returns assignments for a policy
- [x] GetEffectivePolicies resolves priority correctly (lower number = higher priority? or higher number?)
- [x] GetEffectivePolicies with overlapping group + device assignments

### policy_version_integration_test.go
- [x] Create version snapshot
- [x] ListByPolicy returns versions in order
- [x] GetByVersion returns specific version
- [x] LatestVersion returns most recent
- [x] Version data (JSONB) round-trips correctly

---

## Effort Estimate

~1-2 days for all 5 test files. Each follows the established pattern:
1. `testutil.SetupTestDB(t)` for connection
2. Create prerequisite data (enterprise, device) with `t.Cleanup`
3. Exercise each method
4. Assert results

The existing `repository_test.go` and `new_repos_test.go` serve as templates.

---

## Also Noted: Partially Tested Methods in Sprint 1/2 Repos

These aren't zero-coverage but have gaps worth filling opportunistically:

- [x] `device.GetByPlatformID` — 0% (used by Windows/Android enrollment handlers)
- [x] `command.GetByID` — 0% (used by command status checking) **BUG FOUND: NULL error_message scan fails**
- [x] `command.ListByDevice` — 0% (used by command history endpoint) **BUG FOUND: NULL error_message scan fails**
- [x] `certificate.GetBySerial` — 0% (used by cert revocation)
- [x] `audit_log.List` — 46.2% (error paths untested)
