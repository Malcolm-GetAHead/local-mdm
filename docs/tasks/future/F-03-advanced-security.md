# F-03: Advanced Security Features

**Priority**: Medium  
**Effort**: 2-3 days  
**Score Impact**: +0.20 points  
**Status**: Beyond v1.0 scope

---

## Gap Analysis

### Current State
- TLS 1.3 for all endpoints (scope)
- Certificate-based device authentication (S1-03)
- JWT/OIDC authentication (S1-04)
- Rate limiting (S1-06)
- Input validation (S1-06)
- Audit logging (S1-05)

### Missing
- Mutual TLS (mTLS) for device communication
- Hardware Security Module (HSM) integration
- Certificate pinning
- Intrusion detection/prevention
- Security audit logging (separate from operational)
- Compliance reporting (SOC2, HIPAA, PCI-DSS)
- Vulnerability scanning in CI/CD
- Security headers enforcement
- API request signing

### Impact
Without advanced security:
- Higher risk of man-in-the-middle attacks
- CA private key stored on filesystem (not HSM)
- No certificate pinning (devices trust any CA)
- Security events mixed with operational logs
- Manual compliance audits required
- Vulnerabilities discovered in production

---

## Proposed Solution

### 1. Mutual TLS (mTLS)

**Implementation**:
```go
// internal/api/server.go
func NewServerWithMTLS(config *Config) *http.Server {
    // Load CA certificate for client verification
    caCert, _ := ioutil.ReadFile(config.ClientCAPath)
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)
    
    tlsConfig := &tls.Config{
        ClientAuth: tls.RequireAndVerifyClientCert,
        ClientCAs:  caCertPool,
        MinVersion: tls.VersionTLS13,
    }
    
    server := &http.Server{
        Addr:      ":8443",
        TLSConfig: tlsConfig,
    }
    
    return server
}
```

**Configuration**:
```yaml
security:
  mtls:
    enabled: true
    client_ca_path: /etc/localmdm/client-ca.pem
    require_client_cert: true
```

**Benefits**:
- Devices must present valid certificate
- Prevents unauthorized API access
- Stronger authentication than bearer tokens

### 2. Hardware Security Module (HSM) Integration

**Use Cases**:
- Store CA private key in HSM
- Sign device certificates using HSM
- Store APNs private keys in HSM
- Generate and store JWT signing keys

**Implementation Options**:

**AWS CloudHSM**:
```go
// internal/certs/hsm_aws.go
import "github.com/aws/aws-sdk-go/service/cloudhsmv2"

func SignCertificateWithHSM(csr *x509.CertificateRequest) (*x509.Certificate, error) {
    // Use CloudHSM to sign certificate
    client := cloudhsmv2.New(session.New())
    // Sign CSR with HSM-stored CA key
}
```

**PKCS#11**:
```go
// internal/certs/hsm_pkcs11.go
import "github.com/miekg/pkcs11"

func SignCertificateWithPKCS11(csr *x509.CertificateRequest) (*x509.Certificate, error) {
    // Use PKCS#11 interface to HSM
    p := pkcs11.New("/usr/lib/softhsm/libsofthsm2.so")
    // Sign CSR with HSM-stored CA key
}
```

**Configuration**:
```yaml
certificates:
  ca:
    storage: "hsm"  # file, hsm
    hsm:
      type: "aws_cloudhsm"  # aws_cloudhsm, pkcs11
      key_id: "arn:aws:cloudhsm:..."
```

### 3. Certificate Pinning

**Mobile Apps** (future):
```swift
// iOS app
let pinnedCertificates = [
    "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    "sha256/BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
]

func urlSession(_ session: URLSession, didReceive challenge: URLAuthenticationChallenge) {
    // Verify server certificate matches pinned certificate
}
```

**Configuration Profiles**:
```xml
<!-- macOS enrollment profile -->
<key>PayloadCertificateFileName</key>
<string>mdm-server.cer</string>
<key>PayloadContent</key>
<data><!-- Base64 encoded certificate --></data>
```

### 4. Security Audit Logging

**Separate from operational logs**:
```go
// internal/security/audit.go
type SecurityAuditLog struct {
    Timestamp    time.Time
    EventType    string  // authentication, authorization, data_access, admin_action
    Actor        string  // user_id or device_id
    Action       string  // login, access_device, modify_policy, delete_device
    Resource     string  // device_id, policy_id, user_id
    Result       string  // success, failure, denied
    IPAddress    string
    UserAgent    string
    RiskScore    int     // 0-100
    Metadata     map[string]interface{}
}
```

**Storage**:
- Separate database table or dedicated log stream
- Immutable (append-only)
- Encrypted at rest
- Retention: 7 years (compliance requirement)

**Events to Log**:
- Authentication attempts (success/failure)
- Authorization failures
- Device enrollment/unenrollment
- Policy changes
- Device wipe commands
- Certificate issuance/revocation
- Admin user creation/deletion
- API token creation/revocation
- Sensitive data access

### 5. Compliance Reporting

**SOC2 Type II**:
- Access control reports
- Change management logs
- Incident response logs
- Encryption verification
- Backup/restore testing logs

**HIPAA**:
- PHI access logs (if storing health data)
- Encryption at rest/in transit verification
- User access reviews
- Breach notification procedures

**PCI-DSS** (if processing payments):
- Cardholder data access logs
- Network segmentation verification
- Vulnerability scan reports
- Penetration test results

**Implementation**:
```go
// internal/compliance/reports.go
func GenerateSOC2Report(startDate, endDate time.Time) (*Report, error) {
    report := &Report{
        Type: "SOC2",
        Period: fmt.Sprintf("%s to %s", startDate, endDate),
    }
    
    // Access control: all authentication events
    report.AccessControl = getAuthenticationEvents(startDate, endDate)
    
    // Change management: all policy/config changes
    report.ChangeManagement = getChangeEvents(startDate, endDate)
    
    // Incident response: all security incidents
    report.Incidents = getSecurityIncidents(startDate, endDate)
    
    return report, nil
}
```

### 6. Vulnerability Scanning

**CI/CD Integration**:
```yaml
# .github/workflows/security.yml
name: Security Scan

on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      # Dependency scanning
      - name: Run Snyk
        uses: snyk/actions/golang@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
      
      # Container scanning
      - name: Run Trivy
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: localmdm:latest
          format: 'sarif'
          output: 'trivy-results.sarif'
      
      # SAST scanning
      - name: Run Semgrep
        uses: returntocorp/semgrep-action@v1
      
      # Secret scanning
      - name: Run Gitleaks
        uses: gitleaks/gitleaks-action@v2
```

**Scheduled Scans**:
- Daily dependency scans
- Weekly container scans
- Monthly penetration tests (external)

### 7. API Request Signing

**HMAC Signature**:
```go
// internal/api/middleware/signature.go
func VerifySignature(r *http.Request) error {
    signature := r.Header.Get("X-Signature")
    timestamp := r.Header.Get("X-Timestamp")
    
    // Verify timestamp (prevent replay attacks)
    if time.Since(parseTimestamp(timestamp)) > 5*time.Minute {
        return errors.New("request expired")
    }
    
    // Verify signature
    body, _ := ioutil.ReadAll(r.Body)
    expectedSig := hmac.SHA256(apiSecret, timestamp + string(body))
    
    if !hmac.Equal([]byte(signature), expectedSig) {
        return errors.New("invalid signature")
    }
    
    return nil
}
```

---

## Implementation Tasks

### Task 1: mTLS Configuration (0.5 days)
- Configure TLS with client certificate verification
- Generate client certificates for devices
- Update enrollment to include client cert
- Test mTLS enforcement

### Task 2: HSM Integration (1 day)
- Choose HSM provider (AWS CloudHSM, PKCS#11)
- Implement HSM key storage
- Migrate CA key to HSM
- Update certificate signing to use HSM
- Test certificate issuance

### Task 3: Security Audit Logging (0.5 days)
- Create security audit log schema
- Implement audit log writer
- Add audit logging to critical operations
- Create audit log viewer/exporter

### Task 4: Compliance Reporting (0.5 days)
- Define compliance report templates
- Implement report generation
- Create API endpoints for reports
- Document compliance procedures

### Task 5: Vulnerability Scanning (0.5 days)
- Set up Snyk, Trivy, Semgrep in CI/CD
- Configure scan schedules
- Set up alerting for critical vulnerabilities
- Document remediation procedures

---

## Acceptance Criteria

- [ ] mTLS enforced for device communication
- [ ] CA private key stored in HSM
- [ ] Security audit logs separate from operational logs
- [ ] SOC2 compliance report generated
- [ ] Vulnerability scans run in CI/CD
- [ ] Critical vulnerabilities block deployment
- [ ] Certificate pinning documented for mobile apps
- [ ] API request signing implemented (optional)

---

## Cost Considerations

**AWS CloudHSM**: $1.60/hour (~$1,200/month)  
**Snyk**: $0-$99/month (depends on team size)  
**Trivy**: Free (open source)  
**Semgrep**: Free for open source  
**Penetration Testing**: $5,000-$20,000/year

**Total**: ~$15,000-$35,000/year

---

## Security Best Practices

1. **Defense in Depth**: Multiple layers of security
2. **Least Privilege**: Minimal permissions by default
3. **Zero Trust**: Verify every request
4. **Encryption Everywhere**: At rest and in transit
5. **Audit Everything**: Comprehensive logging
6. **Regular Testing**: Continuous vulnerability scanning
7. **Incident Response**: Documented procedures

---

## Future Enhancements

- Web Application Firewall (WAF)
- DDoS protection (Cloudflare, AWS Shield)
- Runtime Application Self-Protection (RASP)
- Security Information and Event Management (SIEM)
- Threat intelligence integration
- Automated incident response

---

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [SOC2 Requirements](https://www.aicpa.org/soc)
- [S1-06: Security Hardening](../sprint-1-foundation/S1-06-security-hardening.md)
