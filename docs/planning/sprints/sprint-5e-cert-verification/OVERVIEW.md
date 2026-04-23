# Sprint 5e: NanoMDM Cert Verification + Test Hygiene

**Status**: 🔲 Not Started  
**Duration**: 1-2 days  
**Goal**: Resolve the NanoMDM certificate verification failure that prevents mdmb (and potentially real devices) from completing check-in through NanoMDM, and clean up remaining test assertion inconsistencies  
**Depends on**: Sprint 5c complete

---

## Why This Sprint

Sprint 5c proved the full Apple MDM enrollment flow works — SCEP cert issuance, Mdm-Signature verification, device record creation — but only when our code handles the check-in directly. When mdmb sends the check-in through NanoMDM, NanoMDM rejects it with:

```
x509: certificate signed by unknown authority (possibly because of
"crypto/rsa: verification error" while trying to verify candidate
authority certificate "Local MDM Root CA")
```

This happens even with everything running on the same Linux distro in Docker. The cert is valid — our Go code verifies it successfully using the same CA cert. The issue is specific to how NanoMDM's `smallstep/pkcs7 v0.2.1` parses the Mdm-Signature PKCS#7 structure created by mdmb's `go.mozilla.org/pkcs7`.

**Why this matters for production**: Real Apple devices use Apple's native CMS implementation, not Go's pkcs7 libraries. NanoMDM is designed and tested against Apple's implementation. However, we can't verify this claim until F-01 (real device testing). If NanoMDM's cert verification is broken for any non-Apple CMS implementation, it could also break with certain proxy configurations, certificate formats, or edge cases. We need to understand the root cause, not just work around it.

---

## Tasks

| ID | Task | Effort | Details |
|---|---|---|---|
| S5e-01 | Root cause the pkcs7 cert verification mismatch | 0.5-1 day | See investigation plan below |
| S5e-02 | Fix or work around the verification failure | 0.5 day | Depends on root cause |
| S5e-03 | Migrate 16 repo integration test assertions to `assert.ErrorIs` | 0.5 day | Mechanical cleanup |

---

### S5e-01: Root Cause Investigation

**What we know:**
- Cert signed by our CA on Linux, verified by our Go code on Linux: ✅ PASSES
- Same cert in Mdm-Signature header, verified by NanoMDM on Linux: ❌ FAILS
- CA cert is identical in both (md5sum verified)
- openssl-signed certs verify fine in NanoMDM
- Error is `crypto/rsa: verification error` — RSA signature check fails

**Investigation plan:**

1. **Capture the exact cert bytes** NanoMDM receives from the Mdm-Signature header. Compare byte-for-byte with the cert our code extracts from the same header. If they differ, the pkcs7 parsing is the issue.

2. **Test with NanoMDM's own pkcs7 library**: Write a test that uses `smallstep/pkcs7 v0.2.1` (same version as NanoMDM) to parse the Mdm-Signature and extract the cert. Verify the cert against our CA. This isolates whether the issue is in pkcs7 parsing or in x509 verification.

3. **Test with NanoMDM's certverify package directly**: Import `github.com/micromdm/nanomdm/certverify` and call `NewPoolVerifier` with our CA PEM, then `Verify` with the cert from the Mdm-Signature. This replicates NanoMDM's exact code path.

4. **Check if the issue is in the PKCS#7 SignedData structure**: mdmb creates the Mdm-Signature using `go.mozilla.org/pkcs7.NewSignedData()`. NanoMDM parses it with `smallstep/pkcs7.Parse()`. The cert embedded in the SignedData might be serialized differently by the two libraries.

5. **Compare cert DER bytes**: Extract the cert from the PKCS#7 structure using both libraries and compare the raw DER bytes. If they differ, one library is modifying the cert during parse/serialize.

**Possible root causes:**
- `smallstep/pkcs7` re-encodes the cert during parsing, producing different DER bytes
- The PKCS#7 SignedData structure from `go.mozilla.org/pkcs7` uses a format that `smallstep/pkcs7` doesn't handle correctly
- NanoMDM's `VerifyMdmSignature` function modifies the cert before passing it to the verifier
- The cert has extensions or encoding that triggers a Go x509 bug in specific contexts

### S5e-02: Fix or Workaround

Depends on root cause. Options:
- **If pkcs7 library mismatch**: Build NanoMDM with `go.mozilla.org/pkcs7` instead of `smallstep/pkcs7`
- **If cert encoding issue**: Normalize the cert in our SCEP response
- **If NanoMDM bug**: Patch NanoMDM's certverify or contribute upstream fix
- **If fundamental incompatibility**: Document as known limitation, verify with real Apple device in F-01

### S5e-03: Repo Test Assertion Cleanup

16 integration test assertions in `internal/repository/` still use:
```go
assert.Contains(t, err.Error(), "not found")
```

Should be:
```go
assert.ErrorIs(t, err, apperrors.ErrNotFound)
```

Files:
- `app_integration_test.go` (4)
- `group_integration_test.go` (4)
- `new_repos_test.go` (1)
- `policy_assignment_integration_test.go` (1)
- `policy_version_integration_test.go` (1)
- `sprint12_gaps_integration_test.go` (3)
- `sprint5_integration_test.go` (2)

Mechanical change — no behavior difference, just consistency with the sentinel error pattern from S5c-07.

---

## Definition of Done

- [ ] Root cause of NanoMDM cert verification failure identified and documented
- [ ] Either: fix applied and mdmb check-in works through NanoMDM, OR: root cause documented with clear production impact assessment
- [ ] All 16 repo integration test assertions use `assert.ErrorIs`
- [ ] All tests pass in Docker (`make dev-test`)

---

*Created: 2026-04-23 — Cert verification issue identified during Sprint 5c mdmb integration*
