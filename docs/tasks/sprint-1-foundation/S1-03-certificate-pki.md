# S1-03: Certificate Infrastructure (PKI)

**Sprint**: 1 — Foundation
**Parallel**: ✅ No blockers — can start immediately
**Effort**: 3-4 days

## Objective

Internal PKI for device identity certificates, SCEP integration, and APNs certificate management.

## Tasks

### 1. CA Certificate Management
- Generate self-signed root CA (RSA 4096, configurable validity)
- Store CA cert + key on disk (configurable path)
- Load existing CA on startup
- CA certificate rotation support (generate new, keep old for validation)
- Files: `internal/certs/ca.go`

### 2. Device Certificate Signing
- Accept CSR, sign with CA, return certificate
- Configurable validity period
- Serial number tracking
- Store issued certs in database via CertificateRepository
- Files: `internal/certs/signing.go`

### 3. SCEP Integration
- Use micromdm/scep as standalone service (Docker container)
- Local MDM proxies SCEP requests to SCEP server
- SCEP server handles CSR signing using CA from step 1
- CA certificate served at SCEP endpoint for device retrieval
- Files: `internal/certs/scep_proxy.go`, `docker-compose.yml` (add scep service)

**SCEP Server Setup**:
```yaml
# docker-compose.yml
services:
  scep:
    image: micromdm/scep:latest
    ports:
      - "8081:8080"
    volumes:
      - ./secrets/ca_cert.pem:/ca/ca.pem
      - ./secrets/ca_key.pem:/ca/ca.key
    environment:
      - SCEP_CHALLENGE=changeme
```

**Local MDM SCEP Proxy**:
- `GET /scep` → proxy to `http://scep:8080/scep`
- `POST /scep` → proxy to `http://scep:8080/scep`
- Add SCEP URL to enrollment profiles: `https://mdm.example.com/scep`

### 4. Certificate Revocation
- Revoke certificate by serial number (update DB)
- Generate CRL (Certificate Revocation List) in DER format
- Serve CRL at HTTP endpoint
- Files: `internal/certs/revocation.go`

### 5. APNs Certificate Management
- Upload APNs push certificate + key (PEM)
- Validate certificate (check expiry, extract topic)
- Store per-enterprise
- Expiration alerting (log warning when < 30 days)
- Files: `internal/certs/apns.go`

## Dependencies on Other Sprint 1 Tasks

| Dependency | Required For | Can Stub? |
|---|---|---|
| S1-01 CertificateRepository | Storing issued certs, revocation | Yes — use in-memory map initially |
| S1-02 Config | CA path, SCEP URL | Yes — hardcode defaults initially |

## Interfaces to Export

```go
type CertificateService interface {
    GetCACertificate() (*x509.Certificate, error)
    SignCSR(csr *x509.CertificateRequest, validity time.Duration) (*x509.Certificate, error)
    RevokeCertificate(serialNumber string) error
    GetCRL() ([]byte, error)
}

type APNsService interface {
    UploadPushCert(enterpriseID uuid.UUID, certPEM, keyPEM []byte) error
    GetPushCert(enterpriseID uuid.UUID) (*tls.Certificate, string, error) // cert, topic, err
}
```

## Acceptance Criteria

- [ ] CA cert generated on first run, loaded on subsequent runs
- [ ] CSR signed and valid certificate returned
- [ ] Signed cert chains to CA cert
- [ ] Revoked cert appears in CRL
- [ ] CRL is valid DER-encoded X.509 CRL
- [ ] APNs cert upload extracts correct topic
- [ ] APNs cert expiry warning logged
