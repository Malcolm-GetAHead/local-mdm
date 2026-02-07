---
name: comprehensive-review
description: Deep analysis for production readiness
---

Perform a comprehensive, production-ready code review.

Specify the component to review and optionally a focus area (security, performance, reliability, etc.).

### REVIEW SCOPE:
- **Security**: Authentication, authorization, injection attacks, data exposure, secrets management
- **Reliability**: Error handling, resource leaks, race conditions, data integrity, failure modes
- **Performance**: Memory usage, CPU usage, database queries, caching, scalability
- **Architecture**: Separation of concerns, coupling, testability, maintainability
- **Testing**: Coverage, edge cases, integration tests, error paths, concurrent operations
- **Production Readiness**: Observability, configuration, deployment, rollback, monitoring
- **Compliance**: Audit logging, data retention, access controls

### ANALYSIS DEPTH:
1. **Static analysis**: Code patterns, anti-patterns, vulnerabilities
2. **Dynamic analysis**: Run tests, check race conditions, measure coverage
3. **Security analysis**: OWASP Top 10, common vulnerabilities, attack vectors
4. **Operational analysis**: What breaks under load? What's missing for production?

### DELIVERABLES:
1. **Categorized issue list** (Critical/High/Medium/Low) with:
   - Exact file and line numbers
   - Exploit scenarios for security issues
   - Impact assessment (security/reliability/performance)
   - Concrete fix with code examples
   - Test cases to verify fix

2. **Prioritized remediation tasks** with:
   - Implementation steps
   - Complete code examples (not snippets)
   - Dependencies between tasks
   - Time estimates
   - Verification procedures

### CRITICAL REQUIREMENTS:
- Be brutally honest. Assume this goes to production tomorrow.
- What would break? What would get hacked? What would cause an outage?
- Test the code where possible. Don't just read it - run it, break it, stress it.
