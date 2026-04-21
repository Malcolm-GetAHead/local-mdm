# F-07: Advanced MDM Features

**Priority**: Low  
**Effort**: 5-7 days (increased from 3-5 — user management, API tokens, app updates added from Sprint 2a audit)  
**Score Impact**: +0.05 points  
**Status**: Out of scope for v1.0

---

## Gap Analysis

### Current State
- Core MDM features (enrollment, policies, commands)
- Basic policy assignment
- Device groups (static, from S4-02)
- Manual policy deployment

### Missing
- Dynamic device groups (moved from S4-02 — filter rules, auto-population, periodic re-evaluation)
- Geofencing (marked as out of scope)
- Conditional access policies
- Scheduled policy deployment
- Policy simulation/dry-run mode
- Bulk operations UI
- Custom device attributes/tags
- Webhook system for external integrations
- User management CRUD (Sprint 2a audit — CLI and dashboard assume it exists)
- API token authentication (Sprint 2a audit — CLI assumes it exists)
- App update management (Sprint 2a audit — install/remove covered in S3, updates not planned)

### Impact
Without advanced features:
- Less flexible policy management
- Manual group management overhead
- No location-based policies
- No time-based policy deployment
- Difficult to test policies before deployment

---

## Proposed Solution

### 1. Geofencing

**Use Cases**:
- Enforce stricter policies when device leaves office
- Disable certain features outside approved locations
- Alert when device enters/exits geofence
- Compliance based on location

**Implementation**:
```go
// internal/geofence/geofence.go
type Geofence struct {
    ID          uuid.UUID
    Name        string
    Type        string  // circle, polygon
    Center      Location
    Radius      float64  // meters
    Vertices    []Location  // for polygon
    Policies    []uuid.UUID  // policies to apply inside fence
    Actions     []Action  // actions to trigger on enter/exit
}

type Location struct {
    Latitude  float64
    Longitude float64
}

type Action struct {
    Type    string  // alert, apply_policy, remove_policy
    Trigger string  // enter, exit
    Target  uuid.UUID
}

func (g *Geofence) Contains(location Location) bool {
    if g.Type == "circle" {
        return distance(g.Center, location) <= g.Radius
    }
    // Polygon containment check
    return pointInPolygon(location, g.Vertices)
}
```

**Device Location Tracking**:
```go
// internal/service/location.go
func (s *LocationService) UpdateDeviceLocation(deviceID uuid.UUID, location Location) error {
    // Store location
    s.repo.UpdateLocation(deviceID, location)
    
    // Check geofences
    geofences := s.geofenceRepo.GetAll()
    for _, fence := range geofences {
        wasInside := s.geofenceRepo.WasInside(deviceID, fence.ID)
        isInside := fence.Contains(location)
        
        if !wasInside && isInside {
            // Device entered geofence
            s.handleGeofenceEnter(deviceID, fence)
        } else if wasInside && !isInside {
            // Device exited geofence
            s.handleGeofenceExit(deviceID, fence)
        }
    }
    
    return nil
}
```

**Privacy Considerations**:
- Location tracking opt-in required
- Location data encrypted at rest
- Configurable location update frequency
- Location history retention policy
- User can view their location history

### 2. Conditional Access Policies

**Conditions**:
- Device compliance status
- Device location (geofence)
- Time of day / day of week
- Network (corporate WiFi, VPN, public)
- Device risk level
- User group membership

**Implementation**:
```go
// internal/policy/conditional.go
type ConditionalPolicy struct {
    ID         uuid.UUID
    Name       string
    Conditions []Condition
    Actions    []Action
    Priority   int
}

type Condition struct {
    Type     string  // compliance, location, time, network, risk
    Operator string  // equals, not_equals, greater_than, in, not_in
    Value    interface{}
}

type Action struct {
    Type   string  // allow, deny, require_mfa, apply_policy
    Target uuid.UUID
}

func (p *ConditionalPolicy) Evaluate(device *Device, context *Context) bool {
    for _, condition := range p.Conditions {
        if !condition.Evaluate(device, context) {
            return false
        }
    }
    return true
}
```

**Example Policies**:
```yaml
# Require VPN outside office
- name: "VPN Required Outside Office"
  conditions:
    - type: location
      operator: not_in
      value: office_geofence
  actions:
    - type: apply_policy
      target: vpn_policy_id

# Restrict access during off-hours
- name: "Off-Hours Restrictions"
  conditions:
    - type: time
      operator: not_between
      value: ["09:00", "17:00"]
  actions:
    - type: apply_policy
      target: restricted_policy_id

# High-risk device restrictions
- name: "High Risk Device"
  conditions:
    - type: risk_level
      operator: greater_than
      value: 70
  actions:
    - type: deny
    - type: alert
      target: security_team
```

### 3. Dynamic Device Groups

> Moved from S4-02 (Sprint 4). S4-02 implements static groups with manual membership. This section adds automatic membership based on filter rules.

**Prerequisite**: S4-02 static groups must be complete. Dynamic groups extend the same `device_groups` table and `group_memberships` junction table with a `rules` JSONB column and a `type` column (static vs dynamic).

**What changes from static groups**:
- Static: admin manually adds device IDs to a group
- Dynamic: admin defines filter rules, system evaluates them against all devices and auto-populates membership
- Policy assignment and deployment logic is identical — it doesn't care how a device got into a group

**Group Rules**:
```go
// internal/groups/dynamic.go
type DynamicGroup struct {
    ID          uuid.UUID
    Name        string
    Description string
    Rules       []Rule
    UpdateFreq  time.Duration  // how often to re-evaluate
}

type Rule struct {
    Field    string  // platform, os_version, compliance_status, location, tag
    Operator string  // equals, contains, greater_than, less_than, in
    Value    interface{}
}

func (g *DynamicGroup) Evaluate(device *Device) bool {
    for _, rule := range g.Rules {
        if !rule.Evaluate(device) {
            return false
        }
    }
    return true
}
```

**Example Groups**:
```yaml
# All non-compliant Windows devices
- name: "Non-Compliant Windows"
  rules:
    - field: platform
      operator: equals
      value: windows
    - field: compliance_status
      operator: equals
      value: non_compliant

# iOS devices with old OS
- name: "Outdated iOS"
  rules:
    - field: platform
      operator: equals
      value: ios
    - field: os_version
      operator: less_than
      value: "16.0"

# Devices in engineering department
- name: "Engineering Devices"
  rules:
    - field: tag
      operator: contains
      value: engineering
```

**Auto-Update**:
- Groups re-evaluated every 15 minutes
- Devices automatically added/removed
- Policies automatically applied/removed
- Audit log of group membership changes

### 4. Scheduled Policy Deployment

**Use Cases**:
- Deploy updates during maintenance window
- Apply restrictions during business hours
- Gradual rollout (canary deployment)

**Implementation**:
```go
// internal/policy/scheduler.go
type ScheduledDeployment struct {
    ID          uuid.UUID
    PolicyID    uuid.UUID
    TargetType  string  // device, group, all
    TargetID    uuid.UUID
    Schedule    Schedule
    Status      string  // pending, in_progress, completed, failed
}

type Schedule struct {
    Type       string  // immediate, scheduled, recurring, gradual
    StartTime  time.Time
    EndTime    time.Time
    Recurrence string  // daily, weekly, monthly
    Gradual    *GradualRollout
}

type GradualRollout struct {
    Percentage int  // deploy to X% of devices
    Interval   time.Duration  // wait between batches
    MaxBatch   int  // max devices per batch
}
```

**Example Schedules**:
```yaml
# Deploy during maintenance window
- policy: security_update
  target: all_devices
  schedule:
    type: scheduled
    start_time: "2026-03-15T02:00:00Z"
    end_time: "2026-03-15T04:00:00Z"

# Gradual rollout
- policy: new_vpn_config
  target: all_devices
  schedule:
    type: gradual
    start_time: "2026-03-15T09:00:00Z"
    gradual:
      percentage: 10  # 10% at a time
      interval: 1h    # wait 1 hour between batches
      max_batch: 100  # max 100 devices per batch

# Recurring policy (business hours only)
- policy: work_hours_restrictions
  target: all_devices
  schedule:
    type: recurring
    recurrence: daily
    start_time: "09:00"
    end_time: "17:00"
```

### 5. Policy Dry-Run Mode

**Use Cases**:
- Test policy before deployment
- Preview policy impact
- Identify conflicts with existing policies

**Implementation**:
```go
// internal/policy/dryrun.go
type DryRunResult struct {
    PolicyID       uuid.UUID
    AffectedDevices []DeviceImpact
    Conflicts      []Conflict
    Warnings       []Warning
}

type DeviceImpact struct {
    DeviceID      uuid.UUID
    DeviceName    string
    Changes       []Change
    WillSucceed   bool
    FailureReason string
}

type Change struct {
    Setting   string
    OldValue  interface{}
    NewValue  interface{}
}

type Conflict struct {
    PolicyID     uuid.UUID
    PolicyName   string
    ConflictType string  // override, incompatible
    Setting      string
}

func (s *PolicyService) DryRun(policyID uuid.UUID, targets []uuid.UUID) (*DryRunResult, error) {
    policy := s.repo.GetByID(policyID)
    result := &DryRunResult{PolicyID: policyID}
    
    for _, deviceID := range targets {
        device := s.deviceRepo.GetByID(deviceID)
        impact := s.evaluatePolicyImpact(policy, device)
        result.AffectedDevices = append(result.AffectedDevices, impact)
    }
    
    result.Conflicts = s.detectConflicts(policy, targets)
    result.Warnings = s.generateWarnings(policy, targets)
    
    return result, nil
}
```

### 6. Bulk Operations UI

**Operations**:
- Lock multiple devices
- Wipe multiple devices
- Apply policy to multiple devices
- Remove policy from multiple devices
- Update device tags
- Move devices to different group

**UI**:
```
Device List
┌─────────────────────────────────────────────────────┐
│ [✓] Select All  [Actions ▼]                         │
├─────────────────────────────────────────────────────┤
│ [✓] Device 1 - Windows 11 - Compliant              │
│ [✓] Device 2 - macOS 14 - Non-Compliant            │
│ [ ] Device 3 - Android 13 - Compliant              │
│ [✓] Device 4 - Windows 11 - Compliant              │
└─────────────────────────────────────────────────────┘

Actions Menu:
- Lock Selected Devices (3)
- Wipe Selected Devices (3)
- Apply Policy...
- Remove Policy...
- Add Tag...
- Move to Group...
```

**Confirmation**:
```
Confirm Bulk Action
┌─────────────────────────────────────────────────────┐
│ You are about to LOCK 3 devices:                    │
│                                                      │
│ • Device 1 (Windows 11)                             │
│ • Device 2 (macOS 14)                               │
│ • Device 4 (Windows 11)                             │
│                                                      │
│ This action cannot be undone.                       │
│                                                      │
│ Type "LOCK" to confirm: [____________]              │
│                                                      │
│ [Cancel]  [Confirm]                                 │
└─────────────────────────────────────────────────────┘
```

### 7. Custom Device Attributes

**Use Cases**:
- Tag devices by department, location, owner
- Custom metadata for reporting
- Dynamic group membership based on tags

**Implementation**:
```go
// internal/models/device.go
type Device struct {
    // ... existing fields
    Tags       map[string]string  // custom key-value pairs
    Attributes JSONB              // flexible custom data
}

// Example tags
device.Tags = map[string]string{
    "department": "engineering",
    "location":   "san-francisco",
    "owner":      "john@example.com",
    "cost_center": "CC-1234",
    "project":    "project-alpha",
}
```

**API**:
```
PUT /api/v1/devices/{id}/tags
{
  "department": "engineering",
  "location": "san-francisco"
}

GET /api/v1/devices?tag=department:engineering
```

### 8. Webhook System

**Events**:
- Device enrolled
- Device unenrolled
- Policy applied
- Policy failed
- Device non-compliant
- Command executed
- Certificate issued

**Implementation**:
```go
// internal/webhooks/webhooks.go
type Webhook struct {
    ID          uuid.UUID
    URL         string
    Events      []string
    Secret      string  // for HMAC signature
    Active      bool
    RetryPolicy RetryPolicy
}

type WebhookEvent struct {
    ID        uuid.UUID
    Type      string
    Timestamp time.Time
    Data      interface{}
    Signature string
}

func (w *Webhook) Send(event *WebhookEvent) error {
    payload, _ := json.Marshal(event)
    signature := hmac.SHA256(w.Secret, payload)
    
    req, _ := http.NewRequest("POST", w.URL, bytes.NewReader(payload))
    req.Header.Set("X-Webhook-Signature", signature)
    req.Header.Set("X-Webhook-Event", event.Type)
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil || resp.StatusCode >= 400 {
        return w.retry(event)
    }
    
    return nil
}
```

**Configuration**:
```yaml
webhooks:
  - url: https://example.com/webhooks/mdm
    events:
      - device.enrolled
      - device.unenrolled
      - policy.failed
    secret: webhook_secret_key
    retry:
      max_attempts: 3
      backoff: exponential
```

### 9. User Management CRUD

> **Gap identified in Sprint 2a audit.** The CLI (S5-08) and web dashboard (S5b) both assume user management API endpoints exist, but no sprint task builds them. The `api_tokens` table exists in the schema but has no server-side implementation.
>
> **Update (Sprint 5):** S5-11 now handles the base case — CRUD endpoints, role assignment, and audit logging. F-07 retains ownership of the **advanced features**: Keycloak user provisioning integration and cross-enterprise user management.

**Endpoints needed**:
```
GET    /api/v1/users                  → List users (scoped to enterprise)
POST   /api/v1/users                  → Create user (admin+)
GET    /api/v1/users/{id}             → Get user
PUT    /api/v1/users/{id}             → Update user (roles, enterprise assignment)
DELETE /api/v1/users/{id}             → Deactivate user
```

**Includes**:
- User-to-enterprise association
- Role assignment (super_admin, admin, operator, viewer)
- Integration with Keycloak user provisioning
- Audit logging for all user mutations

### 10. API Token Authentication

> **Gap identified in Sprint 2a audit.** S5-08 (CLI) consumes API tokens but no task builds the server-side token infrastructure.
>
> **Update (Sprint 5):** S5-11 now handles the base case — token generation, hash-based storage, validation middleware, and revocation. F-07 retains ownership of **advanced features**: token scoping by resource/action, rate limiting per token, and token usage analytics.

**Endpoints needed**:
```
POST   /api/v1/tokens                 → Generate API token (returns token once)
GET    /api/v1/tokens                 → List tokens (metadata only, not secret)
DELETE /api/v1/tokens/{id}            → Revoke token
```

**Includes**:
- Token generation (cryptographically random, plaintext returned once at creation)
- Hash-based storage using pgcrypto `crypt()`/`gen_salt()` — token is never stored in plaintext, only verified on each request
- Token validation middleware (alternative to OIDC JWT)
- Scoping to enterprise and role
- Expiration support
- Uses existing `api_tokens` database table

### 11. App Update Management

> **Gap identified in Sprint 2a audit.** Sprint 3 covers app install/remove and inventory, but not update detection or deployment.

**Includes**:
- Detect outdated apps by comparing installed versions against catalog
- Push app updates to devices (via platform-specific mechanisms)
- Update scheduling (maintenance windows)
- Update status tracking (pending, downloading, installed, failed)
- Reporting on outdated app counts per device/enterprise

---

## Implementation Tasks

### Task 1: Geofencing (1 day)
- Implement geofence data model
- Add location tracking
- Create geofence evaluation logic
- Build geofence UI
- Test with sample locations

### Task 2: Conditional Access (0.5 days)
- Implement condition evaluation engine
- Add conditional policy UI
- Test various conditions
- Document condition types

### Task 3: Dynamic Groups (0.5 days)
- Implement group rule engine
- Add auto-update scheduler
- Build dynamic group UI
- Test group membership updates

### Task 4: Scheduled Deployment (0.5 days)
- Implement scheduler
- Add gradual rollout logic
- Build scheduling UI
- Test scheduled deployments

### Task 5: Policy Dry-Run (0.5 days)
- Implement dry-run evaluation
- Add conflict detection
- Build dry-run UI
- Test with various policies

### Task 6: User Management CRUD (1 day)
- Implement user CRUD handlers and repository
- User-enterprise association and role assignment
- Keycloak user provisioning integration
- Handler tests with mock repos

### Task 7: API Token Authentication (0.5 days)
- Token generation, hashed storage, validation middleware
- Token scoping (enterprise, role, expiration)
- Revocation endpoint
- Tests for token lifecycle

### Task 8: App Update Management (1 day)
- Version comparison logic (installed vs catalog)
- Update push via platform mechanisms
- Update scheduling and status tracking
- Reporting on outdated apps

---

## Acceptance Criteria

- [ ] Geofences can be created and devices tracked
- [ ] Conditional policies evaluate correctly
- [ ] Dynamic groups update automatically
- [ ] Scheduled deployments execute on time
- [ ] Policy dry-run shows accurate impact
- [ ] Bulk operations work for 100+ devices
- [ ] Custom tags can be added to devices
- [ ] Webhooks deliver events reliably
- [ ] User CRUD endpoints work with enterprise scoping and role assignment
- [ ] API tokens can be generated, validated, and revoked
- [ ] App updates detected and deployable to devices

---

## Future Enhancements

- Machine learning for anomaly detection
- Predictive compliance (predict which devices will become non-compliant)
- Automated remediation workflows
- Integration marketplace (Slack, Teams, ServiceNow)
- Mobile app for admins
- Self-service portal for end users

---

## References

- [S4-01: Unified Policy](../sprint-4-policy-and-identity/S4-01-unified-policy.md)
- [S4-02: Policy Assignment](../sprint-4-policy-and-identity/S4-02-policy-assignment.md)
