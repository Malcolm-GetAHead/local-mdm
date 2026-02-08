# S4-01: Unified Policy Model & Translators

**Sprint**: 4 — Policy & Identity
**Parallel**: ✅ Yes
**Depends on**: S3-01, S3-02, S3-03 (need to know what each platform supports)
**Effort**: 5-6 days

## Tasks

### 1. Unified Policy Schema
- JSON schema for platform-agnostic policies
- Policy types: password, encryption, wifi, vpn, restrictions, app_management
- Each type has a common set of fields that map to all platforms
- Platform-specific overrides allowed in `platform_overrides` JSONB field
- Files: `internal/policy/model.go`, `internal/policy/validation.go`

### 2. Platform Translators
- macOS translator: policy → configuration profile XML
- Windows translator: policy → SyncML CSP commands
- Android translator: policy → Android Management API policy JSON
- Each translator reports unsupported fields (logged, not fatal)
- Files: `internal/policy/translate_macos.go`, `internal/policy/translate_windows.go`, `internal/policy/translate_android.go`

### 3. Policy Templates
- Pre-built templates: "Basic Security", "Corporate WiFi", "BYOD Restrictions"
- Templates are just policies with `is_template=true`
- Clone template to create enterprise-specific policy
- Files: `internal/policy/templates.go`

### 4. Policy Versioning
- Each policy update creates a new version
- Rollback to previous version
- Diff between versions
- Files: `internal/policy/versioning.go`

### 5. API Handlers
- `GET/POST /api/v1/policies` — CRUD
- `GET /api/v1/policies/{id}/versions` — version history
- `POST /api/v1/policies/{id}/rollback` — rollback
- `GET /api/v1/policy-templates` — list templates
- Files: `internal/api/handlers/policies.go`

## Acceptance Criteria

- [ ] Create a WiFi policy, translators produce correct output for all 3 platforms
- [ ] Unsupported fields logged but don't block deployment
- [ ] Policy version created on each update
- [ ] Rollback restores previous version
- [ ] Templates can be cloned into enterprise policies
