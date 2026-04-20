# Documentation Index

Complete guide to Local MDM documentation.

## Quick Links

- **Getting Started**: [Setup Guide](dev/SETUP.md) | [Quick Reference](dev/QUICK_REFERENCE.md)
- **Current Status**: [Sprint 3 Complete](planning/sprints/sprint-3-platform-features/OVERVIEW.md) - Commands, Profiles & Apps delivered
- **Implementation**: [Sprint 3 Plan](planning/sprints/sprint-3-platform-features/OVERVIEW.md)
- **Future Plans**: [Post-v1.0 Roadmap](planning/future/README.md)

## Documentation Structure

### 📋 Core Documentation
- **README.md** (root) - Project overview
- **TESTING.md** - Testing guidelines
- **SECURITY.md** - Security guidelines
- **V1-POC-COMPLETION.md** - Sprint 1 completion summary

### 🏗️ Architecture & Design
**Location**: `architecture/`
- `ARCHITECTURE.md` - System architecture and components
- `RATE_LIMITING.md` - Rate limiting design

### 📐 Schemas & Specifications
**Location**: `schemas/`
- `API.md` - REST API reference
- `DATABASE.md` - Database schema and migrations

### 🎯 Project Scope
**Location**: `scope/`
- `SCOPE.md` - Project requirements and goals
- `SCOPING_SUMMARY.md` - Scoping decisions
- `FEATURE_REQUIREMENTS.md` - Feature specifications

### 💻 Development
**Location**: `dev/`
- `SETUP.md` - Development environment setup
- `QUICK_REFERENCE.md` - Common commands and workflows

### 🔧 Implementation Documentation
**Location**: `implementation/`

#### Sprint 1 (`implementation/sprint-1/`)
- **critical/** - Critical fixes (C-01, C-02)
  - Rate limiting implementation
- **high/** - High priority (H-01 to H-08)
  - Circuit breaker, error sanitization, tracing, etc.
- **medium/** - Medium priority (M-01 to M-12)
  - Health checks, request ID, cert monitoring, IP allowlist, etc.
- **low/** - Low priority (L-01 to L-07)
  - Error wrapping, comments, constants, benchmarks, etc.
- **bugfixes/** - Standalone bugfixes
  - Async logger, IPv6 support, etc.

### 🔍 Code Reviews
**Location**: `reviews/`

#### Sprint 1 Review (`reviews/sprint-1/`)
- `README.md` - Review overview (23/24 resolved)
- `ISSUE_TRACKING.md` - Master issue tracking
- `EXECUTIVE_SUMMARY.md` - High-level findings
- `DEPLOYMENT_READY.md` - Deployment readiness
- `CRITICAL_ISSUES.md` - Critical findings (1/1 ✅)
- `HIGH_PRIORITY_ISSUES.md` - High priority (7/8 ✅)
- `MEDIUM_PRIORITY_ISSUES.md` - Medium priority (8/8 ✅)
- `LOW_PRIORITY_ISSUES.md` - Low priority (7/7 ✅)
- `QUICK_REFERENCE.md` - Quick reference
- `SECURITY_ANALYSIS.md` - Security assessment
- `TEST_VERIFICATION_PLAN.md` - Test strategy
- `REMEDIATION_PLAN.md` - Remediation approach

#### Historical Reviews (`reviews/historical/`)
- `sprint-1/` - Sprint 1 foundation reviews
- `prd-rdy-review/` - Production readiness review (archived)
- `prd-dry-review/` - DRY review (archived)

### 📅 Planning
**Location**: `planning/`

#### Sprint Planning (`planning/sprints/`)
- **sprint-1-foundation/** - Database, auth, API, PKI (✅ Complete)
- **sprint-2-platform-core/** - Platform enrollment (✅ Complete)
- **sprint-3-platform-features/** - Platform commands (✅ Complete)
- **sprint-4-policy-and-identity/** - Policy engine (📋 Planned)
- **sprint-5-ui-and-polish/** - UI and deployment (📋 Planned)

#### Future Enhancements (`planning/future/`)
Post-v1.0 roadmap (F-01 to F-08):
- `F-01-real-device-testing.md` - Device testing infrastructure
- `F-02-production-deployment.md` - Kubernetes, AWS deployment
- `F-03-advanced-security.md` - HSM, advanced auth
- `F-04-disaster-recovery.md` - Backup, HA, audit log management
- `F-05-advanced-monitoring.md` - Prometheus, Grafana
- `F-06-user-documentation.md` - User guides, tutorials
- `F-07-advanced-features.md` - Advanced MDM features
- `F-08-internationalization-accessibility.md` - i18n, a11y

#### Planning Documents
- `TASK_BREAKDOWN.md` - Complete task breakdown
- `PROGRESS.md` - Overall progress tracking
- `AGENT_ASSIGNMENT.md` - AI agent assignments
- `NEXT_TASKS.md` - Next priorities
- Various assessment documents

### 🔗 Dependencies
**Location**: `dependencies/`
- **nanomdm/** - Apple MDM server documentation
- **nanodep/** - Apple DEP server documentation
- **nanolib/** - Shared library documentation
- **scep/** - SCEP server documentation
- **keycloak/** - Keycloak integration notes

### 🚀 Deployment
**Location**: `deployment/`
- `SECRETS.md` - Secrets management

### 🔧 Operations
**Location**: `operations/`
- `DATA_MIGRATION.md` - Data migration procedures

### 👥 User Documentation
**Location**: `user/`
- (Future user-facing documentation)

## Finding What You Need

### "How do I get started?"
→ [Setup Guide](dev/SETUP.md)

### "What's the current status?"
→ [Sprint 1 Review](reviews/sprint-1/README.md)

### "How was feature X implemented?"
→ [Implementation Docs](implementation/sprint-1/README.md)

### "What's planned for the future?"
→ [Future Roadmap](planning/future/README.md)

### "What are the code review findings?"
→ [Sprint 1 Review](reviews/sprint-1/ISSUE_TRACKING.md)

### "How do I run tests?"
→ [Testing Guide](TESTING.md)

### "What's the system architecture?"
→ [Architecture](architecture/ARCHITECTURE.md)

### "What APIs are available?"
→ [API Documentation](schemas/API.md)

### "How do I deploy this?"
→ [Deployment Docs](deployment/SECRETS.md) (v1.0 local dev)
→ [F-02 Production Deployment](planning/future/F-02-production-deployment.md) (future)

## Documentation Standards

### Implementation Documents
Each implementation document should include:
- Problem description
- Solution approach
- Code changes
- Test coverage
- Reviewer feedback (if applicable)

### Review Documents
Each review document should include:
- Issue identification
- Priority classification
- Evidence/examples
- Resolution status
- Verification notes

### Planning Documents
Each planning document should include:
- Goals and objectives
- Task breakdown
- Dependencies
- Acceptance criteria
- Effort estimates

## Maintenance

This documentation structure supports:
- ✅ Clear separation of concerns
- ✅ Easy navigation by topic
- ✅ Historical tracking
- ✅ Future planning
- ✅ Implementation tracking
- ✅ Review tracking

Last updated: 2026-04-20
