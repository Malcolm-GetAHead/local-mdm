---
name: implement-fix
description: Complete implementation of identified issue with comprehensive testing
---

Implement a fix for a specific issue with comprehensive testing.

Provide:
- Issue ID and description
- Impact type (Security/Reliability/Performance)
- Priority level (Critical/High/Medium/Low)
- Affected files and line numbers

### REQUIREMENTS:
1. Fix the issue completely (not just the symptom)
2. Add comprehensive tests:
   - Happy path
   - Error paths
   - Edge cases
   - Concurrent operations (if applicable)
   - Performance tests (if applicable)
3. Update documentation
4. Ensure no regressions (run full test suite)

### IMPLEMENTATION CHECKLIST:
- [ ] Root cause identified
- [ ] Fix implemented with minimal code
- [ ] Unit tests added (>80% coverage for new code)
- [ ] Integration tests added (if applicable)
- [ ] Error handling comprehensive
- [ ] Edge cases covered
- [ ] Documentation updated
- [ ] No new security issues introduced
- [ ] No performance regressions
- [ ] All tests passing
- [ ] No race conditions (run with -race)

### DELIVERABLES:
1. Complete implementation (not TODO comments)
2. Test suite with >80% coverage
3. Before/after comparison showing fix
4. Verification that issue is resolved

### CRITICAL REQUIREMENTS:
- Write production-ready code, not prototypes
- Every line must have a purpose
- If you can't test it, refactor until you can
- Consider what happens when this code fails
