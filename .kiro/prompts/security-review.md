---
name: security-review
description: Deep security analysis with threat modeling
---

Perform a comprehensive security-focused review with threat modeling.

### THREAT MODEL:
Assume attacker has:
- Network access to the service
- Knowledge of source code (if open source)
- Ability to create accounts/devices
- Time to probe for vulnerabilities

Specify the component to review and any additional attacker capabilities.

### SECURITY ANALYSIS:

#### 1. OWASP Top 10:
- [ ] Injection (SQL, NoSQL, Command, LDAP)
- [ ] Broken Authentication
- [ ] Sensitive Data Exposure
- [ ] XML External Entities (XXE)
- [ ] Broken Access Control
- [ ] Security Misconfiguration
- [ ] Cross-Site Scripting (XSS)
- [ ] Insecure Deserialization
- [ ] Using Components with Known Vulnerabilities
- [ ] Insufficient Logging & Monitoring

#### 2. Authentication & Authorization:
- [ ] Token validation (expiry, signature, claims)
- [ ] Session management
- [ ] Password storage (if applicable)
- [ ] Multi-factor authentication
- [ ] Authorization checks on all endpoints
- [ ] Privilege escalation vectors

#### 3. Data Protection:
- [ ] Encryption at rest
- [ ] Encryption in transit (TLS)
- [ ] Secrets management
- [ ] PII handling
- [ ] Data retention policies

#### 4. Input Validation:
- [ ] All inputs validated (API, database, files)
- [ ] Size limits enforced
- [ ] Type checking
- [ ] Sanitization
- [ ] Encoding

#### 5. API Security:
- [ ] Rate limiting
- [ ] CORS configuration
- [ ] CSRF protection
- [ ] Request signing (if applicable)
- [ ] API versioning

### DELIVERABLE:
For each vulnerability found:
1. **Severity**: Critical/High/Medium/Low
2. **Attack scenario**: Step-by-step exploit
3. **Impact**: What attacker gains
4. **Fix**: Complete code example
5. **Test**: How to verify fix

### CRITICAL REQUIREMENT:
Try to exploit the system. If you can break it, document how.
