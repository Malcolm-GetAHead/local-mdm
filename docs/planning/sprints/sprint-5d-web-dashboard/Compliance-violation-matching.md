# Compliance Violation Matching — Design Notes

**Status**: ✅ Implemented — engine stores violations as `map[string]string` keyed by config key  
**Created**: 2026-04-25  
**Implemented**: 2026-04-26  
**Context**: Sprint 5d device detail compliance tab

## Problem

The compliance engine stores violations as free-text strings in a JSONB array:

```json
{
  "policy_type": "security",
  "violations": [
    "password not set",
    "disk encryption not enabled",
    "password length 4 < required 8"
  ]
}
```

The dashboard needs to show per-setting pass/fail rows (e.g., "Require Encryption: ✗ Fail"). To do this, it must match each violation string back to the policy config key that produced it (e.g., `require_encryption`).

## Current Implementation (Sprint 5d)

A keyword map in `web_handlers.go` maps config keys to violation keywords:

```go
var violationKeywords = map[string][]string{
    "require_password":    {"password"},
    "min_password_length": {"password length"},
    "require_encryption":  {"encryption"},
    "require_firewall":    {"firewall"},
    "allow_camera":        {"camera"},
}
```

`violationMatchesKey(violation, configKey)` checks if the violation string contains any keyword for that config key. Fallback: replace underscores with spaces and do a substring match.

### Pros
- Simple, explicit, maintainable
- Fallback handles unknown keys without code changes
- No migration or model changes needed
- Works correctly for all current compliance checks

### Cons
- **Coupling**: UI layer knows about compliance engine internals (violation text format)
- **Fragile on rename**: If someone changes "disk encryption not enabled" to "full-disk encryption required" in the engine, the keyword "encryption" still matches — but more specific keywords could break
- **Ambiguity**: "password" matches both `require_password` and `min_password_length`. Currently handled by checking `min_password_length` keywords first ("password length" is more specific), but this is implicit ordering
- **New checks forgotten**: Adding a new compliance check (e.g., `require_screen_lock`) requires adding a keyword entry. The fallback handles it ("require screen lock" → underscore-to-space match) but less precisely

## Engine-Level Fix (Future)

The compliance engine (`internal/service/compliance.go`) should store violations keyed by config key:

```json
{
  "policy_type": "security",
  "violations": {
    "require_encryption": "disk encryption not enabled",
    "min_password_length": "password length 4 < required 8"
  }
}
```

Or with structured detail:

```json
{
  "policy_type": "security",
  "violations": {
    "require_encryption": {"message": "disk encryption not enabled", "expected": true, "actual": false},
    "min_password_length": {"message": "password length too short", "expected": 8, "actual": 4}
  }
}
```

### Changes Required

1. **`internal/service/compliance.go`** — `checkSecurityPolicy` and `checkRestrictionPolicy` return `map[string]string` instead of `[]string`. Each violation keyed by the config key that produced it.

2. **`internal/models/compliance.go`** or the JSONB structure — `Details.violations` changes from `[]string` to `map[string]string` (or `map[string]interface{}` for structured detail).

3. **Migration** — Not strictly needed (JSONB is schema-less), but existing `compliance_results` rows have the old format. The dashboard code would need to handle both formats during transition, or a backfill migration would update existing rows.

4. **`internal/api/web_handlers.go`** — `buildComplianceRows` simplifies dramatically: just iterate policy config keys and look up `violations[key]`. No keyword matching needed.

5. **`internal/reporting/reports.go`** — `ComplianceRow.Details` consumers may need updating if they parse the violations array.

6. **Tests** — `compliance_test.go` assertions change from checking string arrays to checking maps.

### Effort Estimate
- Engine change: ~30 minutes (mechanical refactor of checkSecurityPolicy/checkRestrictionPolicy)
- Dashboard simplification: ~15 minutes (remove keyword map, simplify buildComplianceRows)
- Test updates: ~30 minutes
- Backward compatibility: ~15 minutes (handle both formats in buildComplianceRows during transition)
- Total: ~1.5 hours

### Recommendation

Do this when adding new compliance checks (e.g., `require_screen_lock`, `max_inactivity_timeout`, OS version checks). At that point the keyword map becomes harder to maintain and the structured format pays for itself. Not worth doing as a standalone refactor — the current implementation works correctly for all 5 existing checks.
