# Sprint 5b: EventBus & Compliance Wiring

**Status**: ✅ COMPLETE  
**Branch**: `sprint-5b/eventbus` (not yet merged to main)  
**Duration**: 1 session (~2 hours implementation + hardening)  
**Planned Effort**: 3-5 days  
**Actual Effort**: <1 day  
**Goal**: Build the Go-side LISTEN/NOTIFY EventBus listener, wire compliance auto-evaluation into device and policy lifecycle events, fix device state parsing in check-in handlers

---

## Completion Summary

All 7 tasks complete. 8 commits, 25+ files changed.

| ID | Task | Status | Notes |
|---|---|---|---|
| S5b-01 | EventBus LISTEN/NOTIFY Go listener | ✅ | `pq.Listener` with pre-flight check, reconnect, keepalive. 9 unit tests + 4 integration tests. |
| S5b-02 | New triggers migration | ✅ | Migration 000011. Fixed `notify_mdm_event()` for DELETE ops. Added `extra` JSONB field for policy_id/target_type/target_id/group_id context. |
| S5b-03 | Wire compliance into EventBus | ✅ | 7 subscribers: device.enrolled, device.info_updated, policy.updated, policy.assigned, policy.unassigned, group.member_added, group.member_removed. All resolve affected devices and evaluate. |
| S5b-04 | Device state parsing | ✅ | macOS: enriched CheckinHandler (serial, name, model, OS, build, enrolled status). Windows: 3 security CSP URIs. Android: persist webhook data to platform_data. |
| S5b-05 | Lifecycle hooks + Android gap | ✅ | ComplianceCleanupHook (unenroll/wipe/delete). DeleteByDevice on ComplianceRepository. Android WebhookHandler calls lifecycle on unenrollment. |
| S5b-06 | Load testing framework | ✅ | 3 k6 scenarios + `run_and_record.sh` that appends to `results_history.csv` (living document with git ref, timestamp, metrics). |
| S5b-07 | Test coverage | ✅ | Service: 67% → 70.9%. 4 reporting integration tests with real data. |

## Additional Work (Post-Plan)

Discovered and fixed during implementation and retro:

| Item | Description |
|---|---|
| EventBus connection fix | `pq.Listener` spawns uncontrollable reconnect goroutine. Added pre-flight `sql.Open+Ping` with 5s timeout + `connect_timeout` DSN parameter. |
| CA manager failure → fatal | Was a warning (server started broken). Now returns error — server won't start without valid CA. |
| NanoMDM URL validation | Warns at startup if `nanomdm_url` is empty. |
| Dead nil checks removed | `cmdDispatcher` is always created — nil checks in Start/Shutdown were misleading. |
| Auth failure test fixed | Was using `&db.DB{}` with nil pools — failed on repo creation, never tested auth. Now uses real DB. |
| Test configs fixed | Added Database + Certificates config to cert_monitor and server_auth integration tests. |
| Trigger payload extended | `extra` JSONB field with `policy_id`, `target_type`, `target_id` for policy_assignments; `group_id` for group_memberships. Enables proper subscriber resolution. |
| `policy.unassigned` subscriber | Was missing entirely. Now wired — re-evaluates affected devices on policy removal. |
| Stale comment removed | `// lifecycleService already created above` — leftover from refactor. |
| Recovery key escrow | Added to F-03 with gap analysis (7 items: migration, repo, profile payloads, response parsing, API endpoint). |
| F-07 roadmap expansion | 12 new features: iOS, kiosk, lost mode, selective wipe, OS updates, inventory, zero-touch, alerting, self-service, app store, conditional access sync, SCIM. |
| Load test history | `results_history.csv` — living document tracking performance across sprints. |

## Architecture Decisions

| Decision | Rationale |
|---|---|
| `pq.Listener` over raw `sql.DB.Conn()` | Battle-tested, handles reconnection/keepalive natively. Pre-flight check prevents uncontrollable reconnect goroutine. |
| Pre-flight DB check before `pq.NewListener` | `pq.NewListener` spawns a background goroutine on construction that can't be stopped. Verify connectivity first. |
| `Extra` field in event payload | `policy_assignments` doesn't have `device_id` — subscribers need `policy_id`/`target_type`/`target_id` to resolve affected devices. |
| CA failure is fatal | A server without a CA can't do SCEP, cert signing, or device auth. Silent degradation is worse than failing to start. |
| Fire-and-forget subscribers | Subscriber errors logged but don't stop the EventBus. Events are not retried. Acceptable for compliance evaluation (idempotent, will re-evaluate on next event). |
| App store via package managers | Chocolatey (Windows), Munki (macOS), Managed Google Play (Android). MDM is policy layer, package manager is delivery. |

## Definition of Done — Final Status

- [x] EventBus listener receives NOTIFY events from PostgreSQL triggers
- [x] New triggers fire on platform_data changes, policy unassignment, group membership changes
- [x] Compliance auto-evaluates after device check-in (platform_data update)
- [x] Compliance re-evaluates when policy is updated or assigned/unassigned
- [x] macOS/Windows/Android check-in handlers populate security state in platform_data
- [x] At least one lifecycle hook registered (compliance cleanup)
- [x] Load test scenarios exist and run against local environment
- [x] All existing tests pass (22 packages, race detector, Docker)

---

*Created: 2026-04-22 — Split from Sprint 5 deferred items*  
*Completed: 2026-04-24*
