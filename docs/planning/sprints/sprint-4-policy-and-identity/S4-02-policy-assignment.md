# S4-02: Policy Assignment & Groups

**Sprint**: 4 — Policy & Identity
**Parallel**: ⚠️ Needs S4-01 policy model
**Effort**: 3-4 days

## Tasks

### 1. Device Groups
- Create/update/delete device groups per enterprise
- Static membership (manually add/remove devices)
- Dynamic membership (filter rules: platform, OS version, compliance status)
- Files: `internal/service/groups.go`, `internal/repository/group.go`

### 2. Policy Assignment
- Assign policy to: individual device, device group, or all devices
- Priority ordering when multiple policies apply
- Conflict resolution (most restrictive wins, or priority-based)
- Files: `internal/policy/assignment.go`

### 3. Policy Deployment Trigger
- On assignment: translate + push to all affected devices
- On device enrollment: evaluate and push applicable policies
- On group membership change: re-evaluate affected devices
- Files: `internal/policy/deploy.go`

### 4. API Handlers
- `GET/POST /api/v1/groups` — group CRUD
- `POST /api/v1/groups/{id}/devices` — add devices
- `DELETE /api/v1/groups/{id}/devices/{device_id}` — remove device
- `POST /api/v1/policies/{id}/assign` — assign to device/group
- Files: `internal/api/handlers/groups.go`

## Acceptance Criteria

- [ ] Device group created with static members
- [ ] Dynamic group auto-populates based on filter rules
- [ ] Policy assigned to group, all member devices receive it
- [ ] New device enrolling into a group receives existing policies
- [ ] Policy conflict resolved correctly
