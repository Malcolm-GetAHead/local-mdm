---
name: complete-cycle
description: Single comprehensive prompt for review, fix, and verify
---

Perform a complete sprint review and remediation cycle.

Specify:
- Sprint name/identifier
- Code scope to review
- Priority level for fixes (default: "critical and high-priority")
- Minimum test coverage percentage (default: 80%)

### PHASE 1: DEEP REVIEW (Be Brutal)
Analyze all code in the specified scope for:
- **Security vulnerabilities**: OWASP Top 10, injection, auth bypass, data exposure
- **Reliability issues**: Leaks, races, data corruption, crashes
- **Performance problems**: Memory, CPU, database, scalability
- **Architecture flaws**: Coupling, testability, maintainability
- **Testing gaps**: Coverage, edge cases, integration, error paths
- **Production readiness**: Observability, config, deployment, monitoring

**Test the code**:
- Run all tests with race detector
- Check coverage (report gaps)
- Try to break it (invalid inputs, concurrent access, resource exhaustion)
- Identify what's missing for production

**Deliverable**: Categorized issue list with:
- Severity (Critical/High/Medium/Low)
- Impact (Security/Reliability/Performance)
- Exact location (file:line)
- Exploit scenario (for security)
- Fix with complete code example
- Test case to verify fix

### PHASE 2: PRIORITIZED REMEDIATION
Create ordered task list:
- Group by priority and dependency
- Each task includes:
  * Complete implementation (not snippets)
  * Comprehensive tests
  * Verification steps
  * Time estimate

### PHASE 3: IMPLEMENTATION
For each issue at the specified priority level:
- Implement complete fix
- Add comprehensive tests (meet coverage threshold)
- Verify no regressions
- Update documentation

### PHASE 4: FINAL VERIFICATION
Run complete verification checklist:
- All tests pass (including -race)
- Coverage meets threshold
- All linters pass
- Security scan clean
- No TODOs/FIXMEs
- Documentation complete

### DELIVERABLE:
1. Issue list with fixes
2. Remediation tasks
3. Implemented fixes with tests
4. Verification report
5. Recommendation: **Proceed** or **Fix First**

### CRITICAL RULES:
- Be brutally honest (assume production tomorrow)
- Test everything (don't just read code)
- Complete implementations (no TODOs)
- If you find an issue, fix it completely
- If coverage below threshold, add tests
- If you can't verify a fix, say so

### STOP CONDITIONS:
- If >5 critical issues found, stop and report
- If fundamental architecture flaw found, stop and report
- If security vulnerability found, stop and report immediately
