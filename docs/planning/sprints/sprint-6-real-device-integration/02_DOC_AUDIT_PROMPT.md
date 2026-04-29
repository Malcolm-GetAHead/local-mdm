# Documentation & Test Audit: Holistic Review

## Context
Branch: `main`. Sprint 6 is complete. Multiple sessions have added features, fixed bugs, and updated docs incrementally. This session does a full audit to catch anything that's drifted.

Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.

## Part 1: Documentation Accuracy Audit

Read every documentation file and verify it matches the current codebase. For each file, note: accurate, stale (what's wrong), or missing (what should be added).

### Files to review
```
README.md
CONTRIBUTING.md
CHANGELOG.md
GETTING_STARTED.md (if exists)
docs/SECURITY.md
docs/TESTING.md
docs/dev/SETUP.md
docs/dev/QUICK_REFERENCE.md
docs/architecture/ARCHITECTURE.md
docs/schemas/API.md
docs/schemas/DATABASE.md
docs/planning/future/*.md (all future sprint plans)
tests/device-testing/VM_SETUP.md
tests/device-testing/README.md
tests/device-testing/QUICKSTART.md
```

### What to check for each file
- **Stale references**: old IPs, removed features (Redis), wrong file paths, outdated command examples
- **Missing features**: Sprint 6 additions (webhook pipeline, auto-queue, nginx TLS, CRL, Windows OMA-DM) not documented
- **Incorrect instructions**: setup steps that no longer work, wrong config keys, missing prerequisites
- **Broken links**: references to files that don't exist or moved
- **Consistency**: do different docs contradict each other?

### Known areas of concern
- `docs/TESTING.md` — may not document webhook testing, real device testing, or the `make dev-test` data destruction warning
- `docs/SECURITY.md` — may not mention CRL, nginx TLS, or the CA persistence requirement
- `docs/schemas/API.md` — may be missing Sprint 4+ endpoints (policies, groups, compliance, commands)
- `docs/schemas/DATABASE.md` — may be missing tables added in later sprints
- `docs/architecture/ARCHITECTURE.md` — was updated for Sprint 6 but may have stale sections from earlier sprints
- Future plans in `docs/planning/future/` — some items may already be implemented

## Part 2: Test Coverage Audit

### Integration tests we should have but might not
Run `go test -cover ./...` and review packages below 80%. For each:
1. Are the uncovered lines meaningful (business logic) or trivial (error returns on stdlib calls)?
2. Are there integration tests that should run in Docker but are skipped locally?
3. Are there tests that use mocks where a real DB test would catch more bugs?

### Test data isolation
Verify that `make dev-test` doesn't destroy real device data:
1. Run `make seed` to populate Acme Corp
2. Run `make dev-test`
3. Check that Acme Corp devices, policies, groups still exist
4. Check that no test enterprises leaked (count should be 0 after tests)

### Skipped tests
Search for `t.Skip` across the codebase. For each:
- Is the skip condition still valid?
- Should the test run in Docker (`make dev-test`) even if it skips locally?

## Part 3: Fix What You Find

For documentation issues:
- Fix stale content directly
- Add missing sections where the information is clear from the code
- Flag items that need owner input (architecture decisions, future plans)

For test issues:
- Fix skipped tests that should now run
- Add missing integration tests if they're straightforward
- Document any test gaps that need more work

## Rules
- Run `make dev-test` after any code/test changes — all 19 packages must pass
- Commit fixes as `S6-13: Documentation and test audit fixes`
- Do not modify `.kiro/steering/` files
- Do not invent new features — this is an audit, not a feature sprint
- If you find a lot of issues, batch them into a single commit with a detailed message
