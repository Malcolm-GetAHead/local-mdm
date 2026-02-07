# S5-02: Reporting & Audit

**Sprint**: 5 — UI & Polish
**Parallel**: ✅ Yes
**Effort**: 4-5 days

## Tasks

### 1. Standard Reports
- Device inventory report (all devices, filterable by platform/enterprise)
- Compliance report (compliant vs non-compliant, reasons)
- Enrollment report (enrollments over time)
- Files: `internal/reporting/reports.go`

### 2. Report Export
- CSV export for all reports
- JSON export for API consumers
- Files: `internal/reporting/export.go`

### 3. Audit Log Enhancements
- Search and filter audit logs (actor, action, target, date range)
- Export audit logs (CSV, JSON)
- Retention policy (configurable, auto-purge old entries)
- Files: `internal/api/handlers/audit.go`

### 4. API Endpoints
- `GET /api/v1/reports/devices` — device inventory report
- `GET /api/v1/reports/compliance` — compliance report
- `GET /api/v1/reports/enrollments` — enrollment report
- `GET /api/v1/audit-logs` — enhanced with search/filter/export

## Acceptance Criteria

- [ ] Device inventory report returns correct data with CSV export
- [ ] Compliance report shows per-device status
- [ ] Audit logs searchable by actor and date range
