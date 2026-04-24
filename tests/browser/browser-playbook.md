# Local MDM — Browser Test Playbook
<!-- Run `make seed` before testing to reset data. Tests mutate device statuses. -->

## Login

### Keycloak Login
- [ ] Visit `/dashboard/` — page contains "Sign in"
- [ ] Fill: username=`admin`, password=`admin123`
- [ ] Click "Sign In"
- [ ] Wait 1s
- [ ] Verify "Total Devices" is visible

## Dashboard

### Overview Stats
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Verify "Enrolled" is visible
- [ ] Verify "Non-Compliant" is visible
- [ ] Verify "Active Policies" is visible

## Device Management

### List Devices
- [ ] Navigate to "Devices"
- [ ] Verify table header row is visible

### Sort Devices
- [ ] Visit `/dashboard/devices?sort=platform&dir=asc` — page contains "Devices"
- [ ] Verify table header row is visible

### Filter by Platform
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Select "macOS" from the "platform" dropdown
- [ ] Wait 0.5s
- [ ] Verify table row appears with text "MacBook"

### Search Devices
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Fill: Search=`Surface`
- [ ] Wait 1s
- [ ] Verify table row appears with text "Surface"

### Device Detail
- [ ] Navigate to "Devices"
- [ ] Click "Alice MacBook Pro"
- [ ] Verify "Serial Number" is visible
- [ ] Verify "Lock Device" is visible
- [ ] Verify "Unenroll Device" is visible

## Policy Management

### List Policies
- [ ] Navigate to "Policies"
- [ ] Verify table header row is visible
- [ ] Verify "Corporate Security Baseline" is visible

### Create Policy
- [ ] Click "Create Policy"
- [ ] Verify "Security" is visible
- [ ] Fill: Name=`Test Policy`, Description=`Automated test`
- [ ] Click "Create Policy"
- [ ] Verify redirected to `/dashboard/policies` — page contains "Test Policy"

### Edit Policy
- [ ] Click "Test Policy"
- [ ] Verify "Edit Policy" is visible
- [ ] Verify "Save Changes" is visible

### Assign Policy
- [ ] Navigate to "Policies"
- [ ] Click "Assign" on "Test Policy"
- [ ] Verify "Assign to Group" is visible

## Groups

### List Groups
- [ ] Navigate to "Groups"
- [ ] Verify table header row is visible
- [ ] Verify "Engineering" is visible
- [ ] Verify "Create Group" is visible

### Create Group
- [ ] Click "Create Group"
- [ ] Wait 0.5s
- [ ] Fill: Name=`PW Test Group`, Description=`Playwright test`
- [ ] Click "Save Group"
- [ ] Wait 1s
- [ ] Verify "PW Test Group" is visible

### Group Detail
- [ ] Navigate to "Groups"
- [ ] Click "Engineering"
- [ ] Verify "Engineering" is visible
- [ ] Verify "Members" is visible

## Compliance

### Compliance Dashboard
- [ ] Navigate to "Compliance"
- [ ] Verify "Compliant" is visible
- [ ] Verify "Non-Compliant" is visible
- [ ] Verify table header row is visible

### Filter by Status
- [ ] Visit `/dashboard/compliance?status_filter=non_compliant` — page contains "Non-Compliant"
- [ ] Verify "Clear Filter" is visible

## Audit Log

### View Audit Log
- [ ] Navigate to "Audit Log"
- [ ] Verify table header row is visible

### Filter by Action
- [ ] Fill: action=`policy`
- [ ] Wait 1s
- [ ] Verify "policy.create" is visible

## Logout

### Keycloak Logout
- [ ] Click "Logout"
- [ ] Wait 1s
- [ ] Verify "log out" is visible
