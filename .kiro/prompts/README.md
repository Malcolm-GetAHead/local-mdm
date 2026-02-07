# Kiro Reusable Prompts

**Purpose**: Standardized, comprehensive prompts for code review and quality assurance  
**Location**: `.kiro/prompts/`

---

## Quick Reference

| Prompt | Use Case | Time | Priority |
|--------|----------|------|----------|
| [comprehensive-review](#1-comprehensive-review) | End of sprint, major feature | 30-60 min | ⭐⭐⭐ |
| [complete-cycle](#2-complete-cycle) | Full review + fix + verify | 2-4 hours | ⭐⭐⭐ |
| [implement-fix](#3-implement-fix) | Fix specific issue | 1h-1d | ⭐⭐⭐ |
| [final-verification](#4-final-verification) | Before next phase | 15-30 min | ⭐⭐⭐ |
| [security-review](#5-security-review) | Security audit prep | 30-45 min | ⭐⭐ |
| [performance-review](#6-performance-review) | Before load testing | 30-45 min | ⭐⭐ |

---

## 1. Comprehensive Review

**File**: `comprehensive-review.md`  
**Purpose**: Deep analysis for production readiness  
**Best For**: End of sprint, before deployment, major feature complete

### Arguments
- `{{COMPONENT}}` (required): What to review
- `{{FOCUS_AREA}}` (optional): Specific emphasis area

### Example
```
I need a comprehensive, production-ready code review of Sprint 1 Foundation.
Focus on security and reliability.
```

### What It Does
- ✅ Security analysis (OWASP Top 10, vulnerabilities)
- ✅ Reliability check (leaks, races, data integrity)
- ✅ Performance review (memory, CPU, database)
- ✅ Architecture evaluation (coupling, testability)
- ✅ Testing gaps (coverage, edge cases)
- ✅ Production readiness (observability, config)
- ✅ Runs tests and measures coverage
- ✅ Provides categorized issue list with fixes

### Output
- Issue list (Critical/High/Medium/Low)
- Exact file:line locations
- Complete fix examples
- Test cases
- Prioritized remediation tasks

---

## 2. Complete Cycle

**File**: `complete-cycle.md`  
**Purpose**: Single comprehensive prompt for review, fix, and verify  
**Best For**: End of sprint for complete quality assurance

### Arguments
- `{{SPRINT_NAME}}` (required): Sprint identifier
- `{{SCOPE}}` (required): Code scope to analyze
- `{{PRIORITY_LEVEL}}` (optional, default: "critical and high-priority"): Which issues to fix
- `{{MIN_COVERAGE}}` (optional, default: 80): Minimum test coverage %

### Example
```
COMPREHENSIVE SPRINT REVIEW & REMEDIATION

I need a complete review, fix, and verification cycle for Sprint 1 Foundation.

Scope: entire codebase
Priority: critical and high-priority issues
Minimum coverage: 80%
```

### What It Does
- ✅ **Phase 1**: Deep review (all aspects)
- ✅ **Phase 2**: Prioritized remediation tasks
- ✅ **Phase 3**: Implement fixes with tests
- ✅ **Phase 4**: Final verification
- ✅ Tests everything, breaks it, fixes it
- ✅ Provides go/no-go recommendation

### Output
- Complete issue list with fixes
- Remediation tasks with time estimates
- Implemented fixes with tests
- Verification report
- Proceed or Fix First recommendation

---

## 3. Implement Fix

**File**: `implement-fix.md`  
**Purpose**: Complete implementation of identified issue with comprehensive testing  
**Best For**: After review identifies specific issues

### Arguments
- `{{ISSUE_ID}}` (required): Issue identifier
- `{{ISSUE_DESCRIPTION}}` (required): Brief description
- `{{IMPACT}}` (required): Impact type (Security/Reliability/Performance)
- `{{PRIORITY}}` (required): Priority level (Critical/High/Medium/Low)
- `{{FILES}}` (required): Affected files

### Example
```
Implement fix for CRITICAL-01: Authentication bypass via JWKS race condition

CONTEXT:
- Issue: JWKS refresh happens asynchronously, allowing stale keys
- Impact: Security
- Priority: Critical
- Files affected: internal/auth/oidc.go:115-125
```

### What It Does
- ✅ Implements complete fix (not just symptom)
- ✅ Adds comprehensive tests (>80% coverage)
- ✅ Tests happy path, errors, edge cases
- ✅ Updates documentation
- ✅ Verifies no regressions
- ✅ Checks for race conditions

### Output
- Complete implementation
- Test suite with >80% coverage
- Before/after comparison
- Verification that issue is resolved

---

## 4. Final Verification

**File**: `final-verification.md`  
**Purpose**: Comprehensive verification before next phase  
**Best For**: Before moving to next sprint, before deployment, after major fixes

### Arguments
- `{{NEXT_PHASE}}` (required): What comes next
- `{{MIN_COVERAGE}}` (optional, default: 80): Minimum test coverage %

### Example
```
Perform final verification before proceeding to Sprint 2.
Minimum coverage: 80%
```

### What It Does
- ✅ Code quality checks (linters, style, docs)
- ✅ Testing verification (pass, coverage, race)
- ✅ Security checks (secrets, validation, auth)
- ✅ Reliability checks (errors, resources, context)
- ✅ Production readiness (config, metrics, health)
- ✅ Runs all verification commands

### Output
- Pass/Fail for each checklist item
- List of remaining issues
- Recommendation: Proceed or Fix First

---

## 5. Security Review

**File**: `security-review.md`  
**Purpose**: Deep security analysis with threat modeling  
**Best For**: Before deployment, after auth changes, security audit prep

### Arguments
- `{{COMPONENT}}` (required): What to review
- `{{ATTACKER_CAPABILITIES}}` (optional): Additional attacker capabilities

### Example
```
Perform security-focused review of authentication system.

Assume attacker has:
- Network access to the service
- Valid user credentials (non-admin)
- Knowledge of source code
```

### What It Does
- ✅ OWASP Top 10 analysis
- ✅ Authentication & authorization review
- ✅ Data protection checks
- ✅ Input validation verification
- ✅ API security assessment
- ✅ Threat modeling with attack scenarios
- ✅ Attempts to exploit the system

### Output
- Vulnerabilities with severity
- Step-by-step attack scenarios
- Impact assessment
- Complete fixes with code
- Verification tests

---

## 6. Performance Review

**File**: `performance-review.md`  
**Purpose**: Identify performance bottlenecks and scalability issues  
**Best For**: Before load testing, performance optimization, scaling up

### Arguments
- `{{COMPONENT}}` (required): What to review
- `{{TARGET_RPS}}` (optional, default: 1000): Target requests/second
- `{{P50_MS}}` (optional, default: 50): p50 latency target (ms)
- `{{P95_MS}}` (optional, default: 200): p95 latency target (ms)
- `{{P99_MS}}` (optional, default: 500): p99 latency target (ms)
- `{{CONCURRENT_USERS}}` (optional, default: 10000): Concurrent users
- `{{UPTIME_DAYS}}` (optional, default: 30): Target uptime (days)
- `{{PEAK_RPS}}` (optional, default: 2x TARGET_RPS): Peak load
- `{{SPIKE_RPS}}` (optional, default: 5x TARGET_RPS): Spike load

### Example
```
Perform performance and scalability review of API handlers.

Target: 1000 req/s, p50 < 50ms, p95 < 200ms, p99 < 500ms
Concurrent users: 10000
Uptime: 30 days
```

### What It Does
- ✅ Database query analysis (EXPLAIN, indexes, N+1)
- ✅ Memory analysis (leaks, allocations, GC)
- ✅ CPU analysis (algorithms, blocking ops)
- ✅ I/O analysis (file, network, serialization)
- ✅ Concurrency analysis (locks, channels, races)
- ✅ Caching strategy review
- ✅ Load testing scenarios
- ✅ Runs benchmarks

### Output
- Bottlenecks with measurements
- Impact analysis (how much slower)
- Optimizations with code
- Before/after benchmarks

---

## Recommended Workflow

### Sprint Start
```bash
# Review sprint plan
Use: comprehensive-review.md
Focus: Architecture and dependencies
```

### During Implementation
```bash
# Implement features with quality
Use: implement-fix.md (for each task)
Ensure: Tests, docs, security
```

### Sprint End
```bash
# Option 1: Quick review + verify
Use: comprehensive-review.md → final-verification.md

# Option 2: Complete cycle (recommended)
Use: complete-cycle.md
```

### Before Deployment
```bash
# Security audit
Use: security-review.md

# Performance validation
Use: performance-review.md

# Final check
Use: final-verification.md
```

---

## Tips for Best Results

### 1. Be Specific
❌ "Review the code"  
✅ "Review internal/auth/* for production readiness, focus on security"

### 2. Provide Context
❌ "Fix the bug"  
✅ "Fix CRITICAL-01: Auth bypass in oidc.go:115, impact: security, priority: critical"

### 3. Set Clear Goals
❌ "Make it faster"  
✅ "Target: 1000 req/s, p95 < 200ms, 30 days uptime"

### 4. Demand Verification
❌ "Implement the fix"  
✅ "Implement fix with >80% test coverage, verify no regressions"

### 5. Use Complete Cycle for Thoroughness
When in doubt, use `complete-cycle.md` - it does everything.

---

## Customization

To customize prompts for your project:

1. Copy prompt file
2. Modify sections as needed
3. Add project-specific requirements
4. Update arguments
5. Save with descriptive name

Example:
```bash
cp comprehensive-review.md custom-api-review.md
# Edit to add API-specific checks
```

---

## Integration with CI/CD

Add verification to your pipeline:

```yaml
# .github/workflows/quality.yml
- name: Final Verification
  run: |
    # Use final-verification.md checklist
    go test ./... -v -race -cover
    golangci-lint run ./...
    gosec ./...
```

---

## Questions?

- **Which prompt should I use?** See Quick Reference table above
- **Can I combine prompts?** Yes, or use `complete-cycle.md`
- **How do I customize?** Copy and modify the prompt file
- **What if I find new issues?** Use `implement-fix.md` for each issue

---

## Version History

- **v1.0** (2026-02-07): Initial prompt library
  - 6 comprehensive prompts
  - Covers review, implementation, verification
  - Security and performance focused
