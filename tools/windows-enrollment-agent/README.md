# Windows Enrollment

## Working Enrollment Path: Settings UI

Windows 11 devices enroll via the native Settings UI — no agent required.

### Prerequisites on the Windows device

1. Import the Local MDM CA certificate into the machine trust store:
   ```cmd
   certutil -addstore Root ca.crt
   ```

2. Add a hosts entry for auto-discovery:
   ```cmd
   echo 192.168.1.102 enterpriseenrollment.localmdm.local >> C:\Windows\System32\drivers\etc\hosts
   ```

### Enrollment steps

1. Settings → Accounts → Access work or school
2. Click **"Enroll only in device management"**
3. Enter email: `admin@localmdm.local`
4. Windows auto-discovers `enterpriseenrollment.localmdm.local`
5. Enter any username/password at the credential prompt (OnPremise auth)
6. Windows completes Discovery → Policy → Enrollment → "Setting up your device"

### Server-side requirements

The MDM server must have:
- **CRL Distribution Point** on the TLS server cert — Windows schannel requires revocation checking
- **Non-chunked HTTP responses** — MS-MDE2 spec requires Content-Length headers
- **Correct XML namespace** — `http://schemas.microsoft.com/windows/management/2012/01/enrollment` (no trailing slash)

## Go Enrollment Agent (experimental)

The `enroll.go` tool calls `RegisterDeviceWithManagement` via Go syscall, bypassing the .NET COM threading limitation. Currently returns `0x80180006` — the API accepts the discovery response but fails during the enrollment exchange for unknown reasons. The Settings UI flow uses a different code path and works.

### Build

```bash
cd tools/windows-enrollment-agent
GOOS=windows GOARCH=arm64 go build -o enroll.exe enroll.go
```

## What doesn't work

- **C# agent** — .NET CLR pre-initializes COM as MTA, causing `0x80010106` from `RegisterDeviceWithManagement`
- **RegisterDeviceWithManagement from Go** — gets past COM but fails with `0x80180006`
- **provtool.exe with raw XML** — expects `.ppkg` format (structured ZIP), not provisioning XML
- **Manual registry enrollment** — incomplete, doesn't show in Settings UI
