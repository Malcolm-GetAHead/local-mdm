# Local MDM — Browser Test Playbook

## Login

### Direct Auth Setup
- [ ] Visit `http://localhost:8080/health` — page contains "healthy"

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
