# Sprint-Based Naming Update

**Date**: 2026-02-08  
**Change**: Renamed v1.0-poc → sprint-1

---

## Rationale

**Before**: "v1.0 POC" implied a version/release  
**After**: "sprint-1" reflects sprint-based development workflow

This better aligns with the actual development process:
- Sprint 1: Foundation (complete)
- Sprint 2: Platform Core (next)
- Sprint 3: Platform Features
- Sprint 4: Policy & Identity
- Sprint 5: UI & Polish

---

## Changes Made

### Directory Structure
```
docs/implementation/v1.0-poc/  →  docs/implementation/sprint-1/
docs/reviews/v1.0-poc/         →  docs/reviews/sprint-1/
```

### File References
- Updated all markdown files referencing "v1.0-poc" → "sprint-1"
- Updated all markdown files referencing "v1.0 POC" → "Sprint 1"
- Updated main README.md
- Updated documentation index files

### Files Updated
- README.md (main project)
- docs/implementation/sprint-1/README.md
- docs/reviews/sprint-1/README.md
- docs/INDEX.md
- docs/DOCUMENTATION_STRUCTURE.md
- docs/REORGANIZATION.md
- All implementation docs
- All review docs
- All planning docs

---

## New Structure

```
docs/
├── implementation/
│   └── sprint-1/              # Sprint 1 implementations
│       ├── critical/          # C-01, C-02
│       ├── high/              # H-01 to H-08
│       ├── medium/            # M-01 to M-12
│       ├── low/               # L-01 to L-07
│       └── bugfixes/          # Standalone fixes
│
└── reviews/
    └── sprint-1/              # Sprint 1 code review
        ├── ISSUE_TRACKING.md
        ├── CRITICAL_ISSUES.md
        ├── HIGH_PRIORITY_ISSUES.md
        ├── MEDIUM_PRIORITY_ISSUES.md
        └── LOW_PRIORITY_ISSUES.md
```

---

## Future Sprints

This naming convention supports future sprints:

```
docs/implementation/
├── sprint-1/  # Foundation (complete)
├── sprint-2/  # Platform Core (next)
├── sprint-3/  # Platform Features
├── sprint-4/  # Policy & Identity
└── sprint-5/  # UI & Polish

docs/reviews/
├── sprint-1/  # Foundation review (complete)
├── sprint-2/  # Platform Core review (future)
├── sprint-3/  # Platform Features review (future)
├── sprint-4/  # Policy & Identity review (future)
└── sprint-5/  # UI & Polish review (future)
```

---

## Benefits

### ✅ Clear Sprint Progression
- Easy to see which sprint work belongs to
- Natural progression: sprint-1 → sprint-2 → sprint-3

### ✅ Consistent with Planning
- Matches `docs/planning/sprints/sprint-X/` structure
- Aligns with sprint-based workflow

### ✅ Scalable
- Easy to add sprint-2, sprint-3, etc.
- Clear separation between sprints
- Historical tracking maintained

### ✅ Accurate Naming
- "Sprint 1" accurately describes the work phase
- Not tied to version numbers
- Reflects iterative development

---

## Sprint 1 Status

**Sprint 1: Foundation** - ✅ Complete (95.8%)
- S1-01: Database & repository layer ✅
- S1-02: Configuration & server setup ✅
- S1-03: Certificate & PKI management ✅
- S1-04: Keycloak OIDC authentication ✅
- S1-05: API framework & middleware ✅
- S1-06: Security hardening ✅
- S1-07: Testing framework ✅

**Issues Resolved**: 23/24 (H-06 deferred to future sprint)

---

## Next Steps

**Sprint 2: Platform Core** - Ready to begin
- S2-01: macOS NanoMDM integration
- S2-02: macOS NanoDEP integration
- S2-03: Windows discovery & enrollment
- S2-04: Windows OMA-DM sync
- S2-05: Android enrollment
- S2-06: Device service layer

---

## Migration Notes

**No action required** - All references automatically updated.

If you have external links or bookmarks:
- Old: `docs/implementation/v1.0-poc/`
- New: `docs/implementation/sprint-1/`

- Old: `docs/reviews/v1.0-poc/`
- New: `docs/reviews/sprint-1/`

---

**Updated**: 2026-02-08  
**Status**: ✅ Complete
