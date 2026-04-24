# Local MDM — Browser Test Playbook

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
- [ ] Verify "Devices by Platform" is visible
- [ ] Verify "Recent Activity" is visible

## Device Management

### List Devices
- [ ] Navigate to "Devices"
- [ ] Verify table header row is visible

### Filter by Platform
- [ ] Select "macOS" from the "platform" dropdown
- [ ] Wait 0.5s
- [ ] Verify table row appears with text "MacBook"

### Filter by Status
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Select "Enrolled" from the "status" dropdown
- [ ] Wait 0.5s
- [ ] Verify table header row is visible

### Search Devices
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Fill: Search=`Surface`
- [ ] Wait 0.5s
- [ ] Verify table row appears with text "Surface"

### Device Detail
- [ ] Click "View" on first device row
- [ ] Verify "Serial Number" is visible
- [ ] Verify "OS Version" is visible
- [ ] Verify "Actions" is visible
- [ ] Verify "Lock Device" is visible
- [ ] Verify "Wipe Device" is visible
- [ ] Verify "Groups" is visible

## Policy Management

### List Policies
- [ ] Navigate to "Policies"
- [ ] Verify table header row is visible
- [ ] Verify "Corporate Security Baseline" is visible

### Create Policy
- [ ] Click "Create Policy"
- [ ] Verify "Create Policy" is visible
- [ ] Fill: Name=`Test Policy`, Description=`Automated test`
- [ ] Click "Create Policy"
- [ ] Verify redirected to `/dashboard/policies` — page contains "Test Policy"

### Edit Policy
- [ ] Click "Edit" on "Test Policy"
- [ ] Verify "Edit Policy" is visible
- [ ] Verify "Save Changes" is visible

## Groups

### List Groups
- [ ] Navigate to "Groups"
- [ ] Verify table header row is visible
- [ ] Verify "Engineering" is visible
- [ ] Verify "Sales" is visible
- [ ] Verify "Executives" is visible

### Group Detail
- [ ] Click "View" on "Engineering"
- [ ] Verify "Engineering" is visible
- [ ] Verify "Members" is visible
- [ ] Verify table header row is visible

## Compliance

### Compliance Dashboard
- [ ] Navigate to "Compliance"
- [ ] Verify "Compliant" is visible
- [ ] Verify "Non-Compliant" is visible
- [ ] Verify table header row is visible

## Audit Log

### View Audit Log
- [ ] Navigate to "Audit Log"
- [ ] Verify table header row is visible

### Filter by Action
- [ ] Fill: action=`policy`
- [ ] Wait 0.5s
- [ ] Verify "policy.create" is visible

### Filter by Date
- [ ] Visit `/dashboard/audit` — page contains "Audit Log"
- [ ] Verify table header row is visible

## Logout

### Keycloak Logout
- [ ] Click "Logout"
- [ ] Wait 1s
- [ ] Verify "log out" is visible
