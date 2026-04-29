# VM Setup Guide for Real Device Testing

Step-by-step guide for creating macOS and Windows VMs to test Local MDM enrollment and management.

## Prerequisites

- **Host**: Apple Silicon Mac (M1/M2/M3/M4)
- **UTM**: `brew install --cask utm` (free, native ARM virtualization)
- **Disk space**: ~40 GB per VM
- **Docker stack running**: `make docker-up && sleep 45 && make migrate-up && make seed`

## Network Architecture

```
┌─────────────────────────────────────────────────────┐
│  macOS Host (e.g. 192.168.1.102)                    │
│                                                     │
│  ┌──────────────┐  ┌──────────┐  ┌──────────────┐  │
│  │ localmdm     │  │ nanomdm  │  │ nginx-tls    │  │
│  │ :8080 (HTTP) │  │ :9000    │  │ :443 / :8443 │  │
│  └──────────────┘  └──────────┘  └──────────────┘  │
│  ┌──────────────┐  ┌──────────┐                     │
│  │ keycloak     │  │ postgres │                     │
│  │ :8180        │  │ :5432    │                     │
│  └──────────────┘  └──────────┘                     │
└──────────────────┬──────────────────┬───────────────┘
                   │ bridge network   │
          ┌────────┴───┐      ┌───────┴────┐
          │ macOS VM   │      │ Windows VM │
          │ 192.168.64.4│      │ 192.168.65.2│
          └────────────┘      └────────────┘
```

VMs access the MDM server via the host's LAN IP. The host IP must be reachable from both VMs.

---

## macOS VM

### Create the VM

1. Download a macOS IPSW (macOS 26 Tahoe or later) from Apple or `mist` CLI
2. UTM → **Create New VM** → **Virtualize** → **macOS**
3. Select the IPSW file
4. Settings: **2 CPU cores, 4 GB RAM, 30 GB disk**
5. Name: `LocalMDM-macOS-Test`
6. Complete macOS installation, create user `testuser` / `testuser`

### Configure for MDM Testing

SSH into the VM or use the UTM console:

```bash
# Enable SSH (passwordless from host after key copy)
sudo systemsetup -setremotelogin on

# Get the VM's IP
ifconfig | grep "inet " | grep -v 127.0.0.1
```

From the host, copy your SSH key:
```bash
ssh-copy-id testuser@<vm-ip>
```

### Trust the MDM CA Certificate

The project CA cert must be trusted before enrollment. Copy it from the host:

```bash
# From the host
scp internal/api/certs/ca.crt testuser@<vm-ip>:/tmp/ca.crt

# On the VM
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain /tmp/ca.crt
```

### Enroll in MDM

1. Open **Safari** on the VM and navigate to: `http://<host-ip>:8080/enrollment/macos/profile`
2. This downloads an enrollment `.mobileconfig` profile
3. Open **System Settings → Privacy & Security → Profiles** (or the notification)
4. Install the profile — this triggers SCEP certificate enrollment and MDM check-in
5. The device will appear in the dashboard at `http://<host-ip>:8080/dashboard/`

### What Happens After Enrollment

On each check-in (reboot or manual trigger), the server auto-queues 8 commands:
- SecurityInfo, DeviceInformation (35 queries), ProfileList
- InstalledApplicationList, CertificateList, ManagedApplicationList
- AvailableOSUpdates, OSUpdateStatus

Results populate `platform_data` and drive compliance evaluation. Without APNs, the server cannot push commands — reboot the VM to trigger a check-in.

### Create Template Snapshot

Once enrollment is verified working:
1. Shut down the VM
2. In UTM, right-click → **Clone**
3. Rename the original to `LocalMDM-macOS-Template`
4. Use the clone (`LocalMDM-macOS-Test`) for testing
5. Restore from template: `./scripts/restore_vms.sh`

---

## Windows VM

### Create the VM

1. Download Windows 11 ARM64 ISO from Microsoft
2. UTM → **Create New VM** → **Virtualize** → **Windows**
3. Select the ISO, check **Install drivers and SPICE tools**
4. Settings: **2 CPU cores, 4 GB RAM, 40 GB disk**
5. Name: `LocalMDM-Windows-Test`
6. Complete Windows installation, create user `testuser` / `testuser`

### Configure for MDM Testing

Open **PowerShell as Administrator** in the VM:

```powershell
# Enable OpenSSH Server
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Start-Service sshd
Set-Service -Name sshd -StartupType Automatic

# Get the VM's IP
ipconfig
```

### Trust the MDM CA Certificate

Copy the CA cert to the VM and import it:

```powershell
# From the host (via SCP or shared folder)
scp internal/api/certs/ca.crt testuser@<vm-ip>:C:\Users\testuser\ca.crt

# On the VM (PowerShell as Admin)
certutil -addstore Root C:\Users\testuser\ca.crt
```

### Add DNS Entry for Enrollment Discovery

Windows MDM enrollment uses DNS-based discovery. Add a hosts entry:

```powershell
# On the VM (PowerShell as Admin)
Add-Content C:\Windows\System32\drivers\etc\hosts "<host-ip> enterpriseenrollment.localmdm.local"
```

### Enroll in MDM

1. Open **Settings → Accounts → Access work or school**
2. Click **Enroll only in device management**
3. Enter email: `admin@localmdm.local` (or `<enterprise-uuid>@localmdm.local` for a specific enterprise)
4. The device discovers the MDM server via the hosts entry, completes WSTEP enrollment
5. OMA-DM sync begins on the configured schedule

The email local part is used for enterprise assignment:
- UUID format (e.g. `00000000-0000-0000-0000-000000000001@localmdm.local`) → uses that enterprise ID
- Non-UUID (e.g. `admin@localmdm.local`) → falls back to `default_enterprise_id` config, then hardcoded Acme Corp

### Monitor Enrollment

On the VM, check Event Viewer:
```
Applications and Services Logs → Microsoft → Windows → DeviceManagement-Enterprise-Diagnostics-Provider → Admin
```

On the host, watch server logs:
```bash
docker compose logs -f localmdm
```

### Current Limitations

- **OMA-DM device info queries not yet implemented** — sync handler acknowledges sessions but doesn't query BitLocker/firewall/OS version. Windows compliance shows "unknown".
- **No WNS push** — device polls on schedule. Force sync via Settings → Access work or school → Info → Sync.
- **Programmatic enrollment blocked** — `RegisterDeviceWithManagement` API fails with COM threading error. Settings UI is the working path.

### Create Template Snapshot

Same process as macOS — clone in UTM, rename original to `LocalMDM-Windows-Template`.

---

## Restoring VMs

Before each test session, restore from templates to get clean enrollment state:

```bash
./scripts/restore_vms.sh
```

This stops running VMs, deletes the test clones, and re-clones from templates.

**Important**: `make dev-test` destroys real device data in the shared database. Always run tests *before* enrolling real devices, or restore VMs after running tests.

---

## Troubleshooting

### VM can't reach the MDM server
```bash
# Check host IP is reachable from VM
ping <host-ip>

# Check Docker stack is running
curl http://<host-ip>:8080/health

# Check nginx TLS proxy (Windows needs HTTPS)
curl -k https://<host-ip>:8443/health
```

### macOS enrollment profile fails to install
- Verify CA cert is trusted: **System Settings → Privacy & Security → Certificates**
- Check the enrollment profile URL is correct and server is running
- Check NanoMDM logs: `docker compose logs -f nanomdm`

### Windows enrollment fails at discovery
- Verify hosts entry: `ping enterpriseenrollment.localmdm.local`
- Verify CA cert is trusted: `certutil -store Root | findstr "Local MDM"`
- Check Event Viewer for detailed error codes
- Verify HTTPS works: open `https://<host-ip>:8443/health` in Edge

### Container rebuilds break enrolled devices
The CA cert/key must persist across rebuilds. They're volume-mounted from `./internal/api/certs/`. If you delete these files or rebuild without the mount, all enrolled devices lose trust and must re-enroll from a clean VM snapshot.

---

*Last updated: 2026-04-29*
