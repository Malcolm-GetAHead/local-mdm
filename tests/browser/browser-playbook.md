# Local MDM — Browser Test Playbook

## Login

### Keycloak Login
- [ ] Visit `/dashboard/` — page contains "Sign in"
- [ ] Click "Sign in with Keycloak"
- [ ] Fill: username=`admin`, password=`admin`
- [ ] Click "Sign In"
- [ ] Verify redirected to `/dashboard/` — page contains "Dashboard"

### Noscript Tag
- [ ] Visit `/dashboard/` — page contains "JavaScript Required"

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
- [ ] Verify "MacBook" is visible

### Filter by Platform
- [ ] Select "macOS" from the "platform" dropdown
- [ ] Wait 0.5s
- [ ] Verify table row appears with text "MacBook"

### Search Devices
- [ ] Fill: Search=`Surface`
- [ ] Wait 0.5s
- [ ] Verify table row appears with text "Surface"

### Device Detail
- [ ] Click "View" on first device row
- [ ] Verify "Serial Number" is visible
- [ ] Verify "OS Version" is visible
- [ ] Verify "Actions" is visible

## Policy Management

### List Policies
- [ ] Navigate to "Policies"
- [ ] Verify table header row is visible
- [ ] Verify "Corporate Security Baseline" is visible

### Create Policy
- [ ] Click "Create Policy"
- [ ] Verify form appears with heading "Create Policy"
- [ ] Fill: Name=`Test Policy`, Description=`Automated test`
- [ ] Click "Create Policy"
- [ ] Verify redirected to `/dashboard/policies` — page contains "Test Policy"

### Edit Policy
- [ ] Click "Edit" on "Test Policy"
- [ ] Verify Name field contains "Test Policy"

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
- [ ] Verify "device.lock" is visible

### Filter by Action
- [ ] Fill: action=`policy`
- [ ] Wait 0.5s
- [ ] Verify "policy.create" is visible
