# S5-08: CLI Tools for Administration

**Sprint**: 5 — UI & Polish
**Parallel**: ✅ Yes
**Effort**: 2-3 days

## Objective

Provide command-line tools for administrative tasks, automation, and troubleshooting.

## Tasks

### 1. Main CLI Binary
- Cobra-based CLI framework
- Subcommands for different operations
- Configuration file support
- API authentication (API tokens)
- Files: `cmd/localmdm-cli/main.go`, `cmd/localmdm-cli/cmd/`

### 2. Device Management Commands
- `localmdm-cli devices list` - List all devices
- `localmdm-cli devices get <id>` - Get device details
- `localmdm-cli devices lock <id>` - Lock device
- `localmdm-cli devices wipe <id>` - Wipe device
- `localmdm-cli devices unenroll <id>` - Unenroll device
- Files: `cmd/localmdm-cli/cmd/devices.go`

### 3. Policy Management Commands
- `localmdm-cli policies list` - List policies
- `localmdm-cli policies get <id>` - Get policy details
- `localmdm-cli policies create -f policy.json` - Create policy from file
- `localmdm-cli policies update <id> -f policy.json` - Update policy
- `localmdm-cli policies delete <id>` - Delete policy
- `localmdm-cli policies assign <policy-id> <device-id>` - Assign policy
- Files: `cmd/localmdm-cli/cmd/policies.go`

### 4. Enrollment Commands
- `localmdm-cli enroll macos --enterprise <id>` - Generate macOS enrollment profile
- `localmdm-cli enroll windows --enterprise <id>` - Get Windows enrollment URL
- `localmdm-cli enroll android --enterprise <id>` - Generate Android QR code
- Files: `cmd/localmdm-cli/cmd/enroll.go`

### 5. Certificate Commands
- `localmdm-cli certs list` - List certificates
- `localmdm-cli certs revoke <serial>` - Revoke certificate
- `localmdm-cli certs upload-apns -f cert.pem -k key.pem` - Upload APNs cert
- Files: `cmd/localmdm-cli/cmd/certs.go`

### 6. Admin Commands
- `localmdm-cli users list` - List admin users
- `localmdm-cli users create` - Create admin user
- `localmdm-cli tokens create --name "CI Token"` - Create API token
- `localmdm-cli tokens list` - List API tokens
- `localmdm-cli tokens revoke <id>` - Revoke API token
- Files: `cmd/localmdm-cli/cmd/admin.go`

### 7. Utility Commands
- `localmdm-cli health` - Check server health
- `localmdm-cli version` - Show CLI and server versions
- `localmdm-cli config init` - Initialize config file
- Files: `cmd/localmdm-cli/cmd/utils.go`

## Configuration

```yaml
# ~/.localmdm-cli.yaml
server:
  url: "https://mdm.example.com"
  api_token: "your-api-token-here"
  
output:
  format: "table"  # table, json, yaml
  
defaults:
  enterprise_id: "ent-123e4567-e89b-12d3-a456-426614174000"
```

## Usage Examples

### List Devices
```bash
$ localmdm-cli devices list
ID                                    NAME              PLATFORM  STATUS    LAST SEEN
dev-123e4567-e89b-12d3-a456-42661417  John's MacBook    macos     enrolled  2 minutes ago
dev-223e4567-e89b-12d3-a456-42661417  Jane's Laptop     windows   enrolled  5 minutes ago
dev-323e4567-e89b-12d3-a456-42661417  Bob's Phone       android   enrolled  1 hour ago
```

### Lock Device
```bash
$ localmdm-cli devices lock dev-123e4567-e89b-12d3-a456-426614174000
✓ Lock command sent to device dev-123e4567-e89b-12d3-a456-426614174000
```

### Create Policy from File
```bash
$ localmdm-cli policies create -f wifi-policy.json
✓ Policy created: pol-123e4567-e89b-12d3-a456-426614174000
```

### Generate macOS Enrollment Profile
```bash
$ localmdm-cli enroll macos --enterprise ent-123 --output enrollment.mobileconfig
✓ Enrollment profile saved to enrollment.mobileconfig
```

### Bulk Operations (with jq)
```bash
# Lock all non-compliant devices
$ localmdm-cli devices list --format json | \
  jq -r '.[] | select(.compliance_status == "non_compliant") | .id' | \
  xargs -I {} localmdm-cli devices lock {}
```

## Output Formats

### Table (default)
```
ID          NAME            STATUS
dev-123     John's MacBook  enrolled
dev-456     Jane's Laptop   enrolled
```

### JSON
```json
[
  {
    "id": "dev-123",
    "name": "John's MacBook",
    "status": "enrolled"
  }
]
```

### YAML
```yaml
- id: dev-123
  name: John's MacBook
  status: enrolled
```

## Installation

```bash
# Download binary
curl -L https://github.com/malcolm-getahead/local-mdm/releases/latest/download/localmdm-cli-linux-amd64 -o localmdm-cli
chmod +x localmdm-cli
sudo mv localmdm-cli /usr/local/bin/

# Or build from source
make build-cli
```

## Acceptance Criteria

- [ ] CLI binary builds for Linux, macOS, Windows
- [ ] All device management commands work
- [ ] All policy management commands work
- [ ] Enrollment commands generate correct artifacts
- [ ] Output formats (table, JSON, YAML) work
- [ ] Configuration file loaded correctly
- [ ] API authentication via tokens works
- [ ] Help text and examples documented

## Future Enhancements

- Interactive mode (TUI with bubbletea)
- Shell completion (bash, zsh, fish)
- Bulk import from CSV
- Watch mode (tail device events)
- Plugin system for custom commands
