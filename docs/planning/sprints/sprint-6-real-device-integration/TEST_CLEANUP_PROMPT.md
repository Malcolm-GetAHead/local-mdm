# Test Cleanup: Enterprise Leak Fix

## Context
Branch: `main`. Tests create unique enterprises for isolation but don't clean them up, leaving hundreds of "Test Enterprise" rows after each `make dev-test` run. Real device data (Acme Corp `00000000-0000-0000-0000-000000000001`) must be preserved.

The destructive broad `DELETE FROM devices` / `DELETE FROM certificates` was already fixed (scoped to test data). This session fixes the leak — every test that creates an enterprise must delete it in `t.Cleanup()`.

Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.

## Before Starting
1. Run `make dev-test` to confirm green
2. Run this to see the baseline leak:
   ```sql
   docker compose exec postgres psql -U postgres -d localmdm -c "SELECT count(*) FROM enterprises WHERE id != '00000000-0000-0000-0000-000000000001';"
   ```

## The Pattern

Every test that does this:
```go
enterpriseID := uuid.New()
db.Exec("INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)", enterpriseID, "Test Enterprise", "test-"+enterpriseID.String())
```

Needs this added immediately after:
```go
t.Cleanup(func() { db.Exec("DELETE FROM enterprises WHERE id = $1", enterpriseID) })
```

The `DELETE FROM enterprises` will cascade to devices, policies, groups, compliance_results, device_commands, policy_assignments, users, and any other FK-dependent rows for that enterprise. No need to delete child rows explicitly.

For tests using `testutil.ConnectDB(t)` or `testutil.ConnectRawDB(t)`, the cleanup func has access to `db` via closure.

## Files to Fix

Search for all enterprise INSERT patterns:
```bash
grep -rn "INSERT INTO enterprises" --include="*.go" | grep -v vendor
```

Key files (from previous analysis):
- `internal/certs/expiration_monitor_test.go` — `createTestCertificateWithDevice` creates enterprises
- `internal/scep/challenge_test.go` — may create enterprises indirectly
- `internal/repository/*_test.go` — repository integration tests
- `internal/api/*_test.go` — handler tests
- `internal/auth/*_test.go` — auth integration tests
- `internal/audit/audit_test.go` — audit integration tests
- `internal/reporting/*_test.go` — reporting tests
- `tests/e2e/*_test.go` — e2e tests

## Approach

### Option A: Fix each test individually (thorough)
Add `t.Cleanup()` after every enterprise INSERT. This is the most correct approach.

### Option B: Create a helper (DRY)
Add a helper to `internal/testutil/`:
```go
// CreateTestEnterprise creates a test enterprise and registers cleanup.
func CreateTestEnterprise(t testing.TB, db *sql.DB, name string) uuid.UUID {
    t.Helper()
    id := uuid.New()
    slug := fmt.Sprintf("test-%s", id.String()[:8])
    _, err := db.Exec("INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)", id, name, slug)
    require.NoError(t, err)
    t.Cleanup(func() { db.Exec("DELETE FROM enterprises WHERE id = $1", id) })
    return id
}
```
Then migrate existing tests to use it. This prevents future leaks too.

**Recommended: Option B** — create the helper first, then migrate existing tests to use it. Any test that already has a local `createTestEnterprise` helper should be updated to use the shared one or at minimum add the cleanup.

## Verification
After fixing all files:
1. Run `make dev-test` — all 19 packages must pass
2. Check enterprise count:
   ```sql
   docker compose exec postgres psql -U postgres -d localmdm -c "SELECT count(*) FROM enterprises WHERE id != '00000000-0000-0000-0000-000000000001';"
   ```
   Should be 0 (or very close — some tests may run concurrently).
3. Verify Acme Corp is untouched:
   ```sql
   docker compose exec postgres psql -U postgres -d localmdm -c "SELECT count(*) FROM policies WHERE enterprise_id = '00000000-0000-0000-0000-000000000001';"
   ```
   Should still be 8.

## Rules
- Do NOT delete or modify Acme Corp data
- Do NOT add broad `DELETE FROM` statements — always scope by enterprise ID
- Run `make dev-test` after changes — all 19 packages must pass
- Commit as `S6-12: Add test cleanup for enterprise leak` and push
- Do not modify `.kiro/steering/` files
