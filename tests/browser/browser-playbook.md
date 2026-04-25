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

### Needs Attention Panel
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Verify "Needs Attention" is visible

### Dark Mode Toggle
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Click "Toggle dark mode"
- [ ] Wait 0.5s
- [ ] Click "Toggle dark mode"
- [ ] Wait 0.5s
- [ ] Verify "Total Devices" is visible

## Device Management

### List Devices
- [ ] Navigate to "Devices"
- [ ] Verify table header row is visible

### Sort Devices
- [ ] Visit `/dashboard/devices?sort=platform&dir=asc` — page contains "Devices"
- [ ] Verify table header row is visible

### Pagination
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Verify "Page 1 of" is visible
- [ ] Visit `/dashboard/devices?page=2` — page contains "Devices"
- [ ] Verify "Page 2 of" is visible

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
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Click "Alice MacBook Pro"
- [ ] Verify "Serial Number" is visible
- [ ] Verify "Lock" is visible
- [ ] Verify "Unenroll" is visible
- [ ] Verify "Compliance" is visible

### Device Delete
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Click "Dev Mac Mini"
- [ ] Click "Delete"
- [ ] Wait 1s
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Verify "Dev Mac Mini" is not visible

## Policy Management

### List Policies
- [ ] Visit `/dashboard/policies` — page contains "Policies"
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

### Policy Full CRUD
- [ ] Visit `/dashboard/policies` — page contains "Policies"
- [ ] Click "Create Policy"
- [ ] Fill: Name=`CRUD Test Policy`, Description=`Will be deleted`
- [ ] Click "Create Policy"
- [ ] Verify redirected to `/dashboard/policies` — page contains "CRUD Test Policy"
- [ ] Click "CRUD Test Policy"
- [ ] Verify "Edit Policy" is visible
- [ ] Fill: Name=`CRUD Test Policy Updated`
- [ ] Click "Save Changes"
- [ ] Wait 1s
- [ ] Visit `/dashboard/policies` — page contains "CRUD Test Policy Updated"

## Groups

### List Groups
- [ ] Visit `/dashboard/groups` — page contains "Groups"
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
- [ ] Visit `/dashboard/groups` — page contains "Groups"
- [ ] Click "PW Test Group"
- [ ] Verify "PW Test Group" is visible
- [ ] Verify "Members" is visible

### Group Inline Edit
- [ ] Visit `/dashboard/groups` — page contains "Groups"
- [ ] Click "PW Test Group"
- [ ] Click "Edit"
- [ ] Wait 0.5s
- [ ] Fill: Name=`PW Test Group Edited`, Description=`Updated by Playwright`
- [ ] Click "Save"
- [ ] Wait 1s
- [ ] Verify "PW Test Group Edited" is visible

### Group Full CRUD
- [ ] Visit `/dashboard/groups` — page contains "Groups"
- [ ] Click "Create Group"
- [ ] Wait 0.5s
- [ ] Fill: Name=`CRUD Test Group`, Description=`Will be deleted`
- [ ] Click "Save Group"
- [ ] Wait 1s
- [ ] Verify "CRUD Test Group" is visible
- [ ] Visit `/dashboard/groups` — page contains "CRUD Test Group"
- [ ] Click "Delete" on "CRUD Test Group"
- [ ] Wait 1s
- [ ] Verify "CRUD Test Group" is not visible

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
- [ ] Visit `/dashboard/audit` — page contains "Audit Log"
- [ ] Verify table header row is visible

### Filter by Action
- [ ] Visit `/dashboard/audit` — page contains "Audit Log"
- [ ] Wait 0.5s
- [ ] Fill: action=`policy`
- [ ] Wait 1s
- [ ] Verify "policy.create" is visible

## Mobile (375px)

### Hamburger Menu
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Click "Toggle menu"
- [ ] Wait 0.5s
- [ ] Navigate to "Devices"
- [ ] Wait 0.5s
- [ ] Verify table header row is visible

## Logout

### Keycloak Logout
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Click "Logout"
- [ ] Wait 1s
- [ ] Verify "log out" is visible
