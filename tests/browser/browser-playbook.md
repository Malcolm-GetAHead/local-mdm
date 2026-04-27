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

### Sidebar Navigation
- [ ] Navigate to "Devices"
- [ ] Verify "Local MDM" is visible
- [ ] Verify "Logout" is visible
- [ ] Verify "Devices" is visible
- [ ] Navigate to "Policies"
- [ ] Verify "Local MDM" is visible
- [ ] Verify "Logout" is visible
- [ ] Verify "Policies" is visible
- [ ] Navigate to "Groups"
- [ ] Verify "Local MDM" is visible
- [ ] Verify "Logout" is visible
- [ ] Verify "Groups" is visible
- [ ] Navigate to "Audit Log"
- [ ] Verify "Local MDM" is visible
- [ ] Verify "Logout" is visible
- [ ] Verify "Audit Log" is visible
- [ ] Navigate to "Dashboard"
- [ ] Verify "Local MDM" is visible
- [ ] Verify "Logout" is visible
- [ ] Verify "Total Devices" is visible

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

### Theme Switcher
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Click "Change theme"
- [ ] Wait 0.3s
- [ ] Verify "Violet" is visible
- [ ] Click "Violet"
- [ ] Wait 0.3s
- [ ] Click "Change theme"
- [ ] Wait 0.3s
- [ ] Verify "Ocean" is visible
- [ ] Click "Default"
- [ ] Wait 0.3s
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
- [ ] Click "Platform Details"
- [ ] Verify "Architecture" is visible

### Device Tab Switching
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Click "Alice MacBook Pro"
- [ ] Verify "Compliance" is visible
- [ ] Click "Policies"
- [ ] Verify "Assigned Via" is visible
- [ ] Verify "Compliance" is visible
- [ ] Click "Compliance"
- [ ] Wait 0.5s
- [ ] Click "Commands"
- [ ] Verify "Command" is visible
- [ ] Click "Platform Details"
- [ ] Verify "Architecture" is visible
- [ ] Verify "FileVault" is visible

### Device Delete
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Click "Dev Mac Mini"
- [ ] Click "Delete"
- [ ] Wait 1s
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Verify "Dev Mac Mini" is not visible

### Sub-page Navigation (hx-boost)
- [ ] Visit `/dashboard/devices` — page contains "Devices"
- [ ] Click "Alice MacBook Pro"
- [ ] Wait 0.5s
- [ ] Verify "Serial Number" is visible
- [ ] Verify "Local MDM" is visible
- [ ] Verify "Logout" is visible

## Policy Management

### List Policies
- [ ] Visit `/dashboard/policies` — page contains "Policies"
- [ ] Verify table header row is visible
- [ ] Verify "Corporate Security Baseline" is visible

### Create Policy with Settings
- [ ] Click "Create Policy"
- [ ] Verify "Security" is visible
- [ ] Fill: Name=`Test Policy`, Description=`Automated test`
- [ ] Click "Require Encryption"
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

### Policy Assign and Unassign
- [ ] Select "Engineering" from the "group-select" dropdown
- [ ] Click "Assign to Group"
- [ ] Wait 1s
- [ ] Verify "Current Assignments" is visible
- [ ] Click "Remove" on "Engineering"
- [ ] Wait 1s

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
- [ ] Visit `/dashboard/compliance` — page contains "Compliance"
- [ ] Click "Non-Compliant"
- [ ] Wait 0.5s
- [ ] Verify table header row is visible
- [ ] Click "Non-Compliant"
- [ ] Wait 0.5s
- [ ] Verify table header row is visible

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

### Audit Log Expand Detail
- [ ] Visit `/dashboard/audit` — page contains "Audit Log"
- [ ] Wait 0.5s
- [ ] Click "▶"
- [ ] Wait 0.5s
- [ ] Verify "▼" is visible

## Mobile (375px)

### Hamburger Menu
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Click "Toggle menu"
- [ ] Wait 0.5s
- [ ] Navigate to "Devices"
- [ ] Wait 0.5s
- [ ] Verify table header row is visible

## API Documentation

### Swagger UI Loads
- [ ] Visit `/docs`
- [ ] Wait 2s
- [ ] Verify "Local MDM API" is visible

## Logout

### Keycloak Logout
- [ ] Visit `/dashboard/` — page contains "Total Devices"
- [ ] Click "Logout"
- [ ] Wait 1s
- [ ] Verify "log out" is visible
