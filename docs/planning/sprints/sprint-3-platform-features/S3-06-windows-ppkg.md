# S3-06: Windows Provisioning Packages (.ppkg)

**Sprint**: 3 — Platform Features
**Parallel**: ⚠️ Partial (depends on S2-03 enrollment working)
**Depends on**: S2-03 (Windows enrollment)
**Effort**: 2-3 days

## Objective

Generate Windows Provisioning Packages (.ppkg) for bulk enrollment and configuration deployment.

## Tasks

### 1. Provisioning Package Generation
- Generate .ppkg files using Windows ICD XML format
- Embed enrollment configuration (MDM server URL, certificates)
- Embed WiFi profiles
- Embed VPN profiles
- Files: `internal/platform/windows/ppkg.go`, `internal/platform/windows/ppkg_xml.go`

### 2. Package Signing
- Sign .ppkg with code signing certificate
- Validate certificate chain
- Files: `internal/platform/windows/ppkg_signing.go`

### 3. Package Templates
- Pre-built templates for common scenarios:
  - Basic enrollment only
  - Enrollment + WiFi
  - Enrollment + WiFi + VPN
  - Kiosk mode configuration
- Files: `internal/platform/windows/ppkg_templates.go`

### 4. API Endpoints
- `POST /api/v1/windows/ppkg` - Generate provisioning package
- `GET /api/v1/windows/ppkg/{id}` - Download .ppkg file
- `GET /api/v1/windows/ppkg/templates` - List available templates
- Files: `internal/api/handlers/windows_ppkg.go`

## Provisioning Package Structure

```xml
<?xml version="1.0" encoding="utf-8"?>
<WindowsCustomizations>
  <PackageConfig xmlns="urn:schemas-Microsoft-com:Windows-ICD-Package-Config.v1.0">
    <ID>{GUID}</ID>
    <Name>LocalMDM Enrollment</Name>
    <Version>1.0</Version>
    <OwnerType>OEM</OwnerType>
  </PackageConfig>
  <Settings xmlns="urn:schemas-microsoft-com:windows-provisioning">
    <Customizations>
      <Common>
        <!-- MDM Enrollment -->
        <Policies>
          <AllowMDMEnrollment>1</AllowMDMEnrollment>
        </Policies>
        <DMClient>
          <Provider>
            <ProviderID>LocalMDM</ProviderID>
            <DiscoveryServiceFullURL>https://mdm.example.com/EnrollmentServer/Discovery.svc</DiscoveryServiceFullURL>
          </Provider>
        </DMClient>
        <!-- WiFi Profile -->
        <Connections>
          <WiFi>
            <Profile>...</Profile>
          </WiFi>
        </Connections>
      </Common>
    </Customizations>
  </Settings>
</WindowsCustomizations>
```

## Usage Scenarios

### Scenario 1: USB Drive Deployment
1. Generate .ppkg via API
2. Copy to USB drive
3. Insert USB into new Windows device
4. Device auto-detects and applies package
5. Device enrolls in MDM

### Scenario 2: Network Share
1. Generate .ppkg via API
2. Place on network share
3. Run via Group Policy or script
4. Devices apply package and enroll

### Scenario 3: OOBE (Out-of-Box Experience)
1. Generate .ppkg with OOBE settings
2. Apply during Windows setup
3. Device enrolls before user login

## Configuration Options

```json
{
  "name": "Corporate Enrollment",
  "template": "enrollment_wifi",
  "settings": {
    "mdm": {
      "server_url": "https://mdm.example.com",
      "discovery_url": "https://mdm.example.com/EnrollmentServer/Discovery.svc"
    },
    "wifi": {
      "ssid": "CorpNet",
      "security": "wpa2-enterprise",
      "eap_method": "PEAP"
    },
    "vpn": {
      "name": "Corporate VPN",
      "server": "vpn.example.com",
      "type": "IKEv2"
    }
  }
}
```

## Acceptance Criteria

- [ ] .ppkg file generated with enrollment configuration
- [ ] Package can be applied to Windows device
- [ ] Device enrolls automatically after applying package
- [ ] WiFi profile included in package works
- [ ] Package signed with valid certificate
- [ ] Templates available for common scenarios

## References

- [Windows Provisioning Documentation](https://docs.microsoft.com/en-us/windows/configuration/provisioning-packages/provisioning-packages)
- [Windows ICD Reference](https://docs.microsoft.com/en-us/windows/configuration/wcd/wcd)
