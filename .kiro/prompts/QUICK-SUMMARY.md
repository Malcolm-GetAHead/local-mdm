# Kiro Prompts - Quick Summary

## 📋 All Prompts at a Glance

### 1. **comprehensive-review.md** ⭐⭐⭐
**One-liner**: Deep production-ready analysis  
**Arguments**: `COMPONENT` (required), `FOCUS_AREA` (optional)  
**Time**: 30-60 min  
**Use**: End of sprint, before deployment

### 2. **complete-cycle.md** ⭐⭐⭐
**One-liner**: Review + Fix + Verify in one go  
**Arguments**: `SPRINT_NAME`, `SCOPE`, `PRIORITY_LEVEL` (opt), `MIN_COVERAGE` (opt)  
**Time**: 2-4 hours  
**Use**: Complete sprint quality assurance

### 3. **implement-fix.md** ⭐⭐⭐
**One-liner**: Fix specific issue with tests  
**Arguments**: `ISSUE_ID`, `ISSUE_DESCRIPTION`, `IMPACT`, `PRIORITY`, `FILES`  
**Time**: 1 hour - 1 day  
**Use**: After review identifies issues

### 4. **final-verification.md** ⭐⭐⭐
**One-liner**: Checklist before proceeding  
**Arguments**: `NEXT_PHASE` (required), `MIN_COVERAGE` (opt)  
**Time**: 15-30 min  
**Use**: Before next sprint/deployment

### 5. **security-review.md** ⭐⭐
**One-liner**: Security audit with threat modeling  
**Arguments**: `COMPONENT`, `ATTACKER_CAPABILITIES` (opt)  
**Time**: 30-45 min  
**Use**: Security audit prep, after auth changes

### 6. **performance-review.md** ⭐⭐
**One-liner**: Find bottlenecks and scalability issues  
**Arguments**: `COMPONENT`, `TARGET_RPS`, `P50_MS`, `P95_MS`, `P99_MS`, `CONCURRENT_USERS`, `UPTIME_DAYS` (all optional)  
**Time**: 30-45 min  
**Use**: Before load testing, optimization

---

## 🎯 Which Prompt Should I Use?

```
Need comprehensive review? → comprehensive-review.md
Need everything done? → complete-cycle.md
Need to fix an issue? → implement-fix.md
Need to verify quality? → final-verification.md
Need security audit? → security-review.md
Need performance check? → performance-review.md
```

---

## 📝 Quick Copy-Paste Examples

### End of Sprint
```
COMPREHENSIVE SPRINT REVIEW & REMEDIATION

I need a complete review, fix, and verification cycle for Sprint 1.
Scope: entire codebase
Priority: critical and high-priority issues
Minimum coverage: 80%
```

### Fix Specific Issue
```
Implement fix for CRITICAL-01: SQL injection in ORDER BY

CONTEXT:
- Issue: Dynamic ORDER BY allows SQL injection
- Impact: Security
- Priority: Critical
- Files affected: internal/repository/device.go:95
```

### Before Deployment
```
Perform final verification before proceeding to Production.
Minimum coverage: 85%
```

### Security Audit
```
Perform security-focused review of entire application.

Assume attacker has:
- Network access
- Valid user credentials
- Knowledge of source code
```

---

## 💡 Pro Tips

1. **Start with complete-cycle.md** for thorough quality assurance
2. **Use implement-fix.md** for each issue found
3. **Always run final-verification.md** before proceeding
4. **Run security-review.md** before any deployment
5. **Run performance-review.md** before scaling up

---

## 📂 File Locations

All prompts are in: `.kiro/prompts/`

- `comprehensive-review.md`
- `complete-cycle.md`
- `implement-fix.md`
- `final-verification.md`
- `security-review.md`
- `performance-review.md`
- `README.md` (full documentation)
- `QUICK-SUMMARY.md` (this file)
