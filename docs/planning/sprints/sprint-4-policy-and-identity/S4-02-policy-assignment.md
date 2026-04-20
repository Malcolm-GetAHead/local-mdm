# S4-02: Policy Assignment & Static Device Groups

**Sprint**: 4 — Policy & Identity  
**Parallel**: ⚠️ Needs S4-01 policy model  
**Effort**: 3-4 days

## Tasks

### 1. Static Device Groups
- Create/update/delete device groups per enterprise
- Static membership — admin manually adds/removes devices
- Group metadata: name, description, enterprise_id
- Files: `internal/service/groups.go`, `internal/repository/group.go`

### 2. Policy Assignment
- Assign policy to: individual device, device group, or all devices
- Priority ordering when multiple policies apply
- Conflict resolution (most restrictive wins, or priority-based)
- Uses `policy_assignments` table (migration 000006) with target_type/target_id/priority
- Files: `internal/service/policy.go` (assignment methods on PolicyService)

### 3. Policy Deployment Trigger
- On assignment: translate + push to all affected devices
- On device enrollment: evaluate and push applicable policies
- On group membership change: re-evaluate affected devices
- Service calls platform translators (Sprint 3) and command dispatcher (async)
- Files: `internal/service/policy.go` (deployment methods on PolicyService)

### 4. API Handlers
- Handlers are thin — parse request, call GroupService/PolicyService, format response
- Files: `internal/api/handlers.go` (new handler methods on Server)

## Acceptance Criteria

- [ ] Device group created with static members
- [ ] Devices added to and removed from groups
- [ ] Policy assigned to group, all member devices receive it
- [ ] New device added to group receives existing group policies
- [ ] Policy conflict resolved correctly (priority-based)

## Out of Scope (Deferred to F-07)

The following dynamic group features were originally in this task but have been moved to [F-07: Advanced Features](../../future/F-07-advanced-features.md) to keep Sprint 4 focused:

- **Dynamic group membership** — filter rules that auto-populate groups based on device attributes (platform, OS version, compliance status, tags)
- **Auto-update scheduler** — periodic re-evaluation of dynamic group membership
- **Rule engine** — parsing and evaluating filter expressions against device data

Static groups cover the core use case (admin assigns devices to groups, assigns policies to groups). Dynamic groups add automation on top of that foundation and can be built later without changing the policy assignment or deployment logic.
