# S2-03: Windows — Discovery & Enrollment (MS-MDE2)

**Sprint**: 2 — Platform Core
**Parallel**: ✅ Can start immediately after Sprint 1
**Effort**: 5-6 days

## Objective

Implement the MS-MDE2 discovery and enrollment protocol so Windows 10/11 devices can enroll via Settings → Access work or school.

## Tasks

### 1. Discovery Service
- `GET/POST /EnrollmentServer/Discovery.svc` endpoint
- Parse discovery request XML
- Return discovery response with enrollment endpoint URLs
- Support both federated and on-premise auth types
- Files: `internal/platform/windows/discovery.go`, `internal/platform/windows/protocol/discovery.go`

### 2. Enrollment Policy Service
- `POST /EnrollmentServer/Policy.svc` endpoint
- Return enrollment policy (certificate template, key length, hash algorithm)
- Files: `internal/platform/windows/policy_service.go`

### 3. Enrollment Service (WSTEP)
- `POST /EnrollmentServer/Enrollment.svc` endpoint
- Parse WSTEP enrollment request (SOAP/XML)
- Extract CSR from request
- Sign CSR using CA from S1-03
- Return signed certificate + provisioning XML
- Provisioning XML includes OMA-DM server URL, cert store config
- Register device in DeviceRepository
- Files: `internal/platform/windows/enrollment.go`, `internal/platform/windows/protocol/wstep.go`

### 4. Terms of Use (optional)
- `GET /EnrollmentServer/TermsOfUse` endpoint
- Return HTML terms page
- Files: `internal/platform/windows/terms.go`

### 5. XML Helpers
- SOAP envelope parsing/generation
- WS-Security header handling
- Binary security token extraction
- Files: `internal/platform/windows/protocol/soap.go`, `internal/platform/windows/protocol/xml_helpers.go`

### 6. Routes
- `/EnrollmentServer/Discovery.svc`
- `/EnrollmentServer/Policy.svc`
- `/EnrollmentServer/Enrollment.svc`
- `/EnrollmentServer/TermsOfUse`

## Acceptance Criteria

- [ ] Windows device discovers MDM server via Settings → Access work or school
- [ ] Discovery response contains correct enrollment URLs
- [ ] Device submits CSR and receives signed certificate
- [ ] Device receives provisioning XML with OMA-DM server URL
- [ ] Device appears in `GET /api/v1/devices` with platform=windows
- [ ] Enrollment works with both Windows 10 and Windows 11
