# Development Planning

This directory contains sprint planning, task breakdowns, and future roadmap documentation.

## Structure

### sprints/
Sprint-based development planning:
- **sprint-1-foundation/** - Database, auth, API, PKI, security, testing
- **sprint-2-platform-core/** - macOS, Windows, Android enrollment
- **sprint-3-platform-features/** - Platform-specific commands and policies
- **sprint-4-policy-and-identity/** - Unified policy engine, compliance
- **sprint-5-ui-and-polish/** - Web dashboard, observability, deployment

Each sprint directory contains:
- `OVERVIEW.md` - Sprint goals and scope
- `S{N}-{XX}-{name}.md` - Individual task specifications
- `*-COMPLETED.md` - Completion reports

### future/
Post-v1.0 enhancements (F-01 through F-08):
- `F-01-real-device-testing.md` - Real device testing infrastructure
- `F-02-production-deployment.md` - Production deployment (Kubernetes, AWS)
- `F-03-advanced-security.md` - HSM, secrets management, advanced auth
- `F-04-disaster-recovery.md` - Backup, audit log management, HA
- `F-05-advanced-monitoring.md` - Prometheus, Grafana, alerting
- `F-06-user-documentation.md` - User guides, API docs, tutorials
- `F-07-advanced-features.md` - Advanced MDM features
- `F-08-internationalization-accessibility.md` - i18n, a11y

### Root Files
- `TASK_BREAKDOWN.md` - Complete task breakdown
- `PROGRESS.md` - Overall progress tracking
- `AGENT_ASSIGNMENT.md` - AI agent task assignments
- `NEXT_TASKS.md` - Next task priorities
- Various assessment and summary documents

## Current Status

**Sprint 1**: ✅ Complete (Foundation)
**Sprint 2-5**: 📋 Planned (Platform implementation)
**Future Enhancements**: 📋 Documented (Post-v1.0)

## Related Documentation

- **Implementation**: `docs/implementation/` - Completed implementations
- **Reviews**: `docs/reviews/` - Code review findings
- **Architecture**: `docs/architecture/` - Design decisions
- **Scope**: `docs/scope/` - Project scope and requirements
