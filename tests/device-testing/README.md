# Device Testing Framework

Automated testing framework for Local MDM device enrollment and management.

## Overview

This framework provides automated testing for:
- ✅ **macOS** enrollment via configuration profiles
- ✅ **Android** enrollment via QR codes
- ⏳ **Windows** enrollment (coming soon)

## Prerequisites

### One-Time Setup (You Do This Once)

1. **Install UTM** (free, native macOS VM manager):
   ```bash
   brew install --cask utm
   ```

2. **Create VMs in UTM**:
   - macOS VM: "LocalMDM-macOS-Test" from `images/macos-26.ipsw` (2 cores, 4GB RAM, 30GB disk)
   - Windows VM: "LocalMDM-Windows-Test" from `images/Win11_25H2_English_Arm64.iso` (2 cores, 4GB RAM, 40GB disk)

3. **Configure VMs for remote access**:
   ```bash
   ./scripts/setup_vms.sh
   ```
   This script will guide you through:
   - Enabling SSH on macOS VM
   - Enabling WinRM on Windows VM
   - Recording VM IP addresses

3. **Install Python dependencies**:
   ```bash
   pip3 install -r requirements.txt
   ```

4. **Create Android emulator** (if not exists):
   ```bash
   avdmanager create avd \
     -n LocalMDM-Test \
     -k "system-images;android-33;google_apis;arm64-v8a" \
     -d "pixel_7"
   ```

## Running Tests

### Run All Tests
```bash
./scripts/run_all_tests.sh
```

### Run Specific Platform
```bash
./scripts/run_all_tests.sh macos
./scripts/run_all_tests.sh android
./scripts/run_all_tests.sh windows
```

### Run Individual Test
```bash
python3 scripts/test_macos_enrollment.py
python3 scripts/test_android_enrollment.py
```

## What Gets Tested

### macOS Tests
1. ✅ VM connectivity (SSH)
2. ✅ MDM server accessibility
3. ✅ Enrollment profile generation
4. ✅ Profile installation
5. ✅ Device enrollment verification
6. ✅ Profile listing
7. ✅ Device lock command

### Android Tests
1. ✅ Emulator startup
2. ✅ MDM server accessibility
3. ✅ Enrollment token generation
4. ✅ QR code generation
5. ✅ Enrollment simulation
6. ✅ Device enrollment verification
7. ✅ Device lock command

### Windows Tests (Coming Soon)
1. ⏳ VM connectivity (WinRM)
2. ⏳ Discovery service
3. ⏳ Enrollment via Settings
4. ⏳ Certificate issuance
5. ⏳ OMA-DM sync
6. ⏳ Device lock command

## Test Results

Results are saved to `results/` directory:

```
results/
├── macos_test_results_<timestamp>.json
├── android_test_results_<timestamp>.json
├── test_run_<timestamp>.log
├── report_<timestamp>.html
└── screenshots/
    ├── macos_profile_installed_<timestamp>.png
    ├── macos_device_locked_<timestamp>.png
    ├── android_enrollment_started_<timestamp>.png
    └── android_device_locked_<timestamp>.png
```

### View HTML Report
```bash
open results/report_<timestamp>.html
```

## Restoring VMs

After tests, restore VMs to clean state:

```bash
./scripts/restore_vms.sh
```

This deletes test VMs and re-clones from templates. See [QUICKSTART.md](QUICKSTART.md) for template setup.

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/device-testing.yml
name: Device Testing

on: [push, pull_request]

jobs:
  test-android:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.9'
      
      - name: Install dependencies
        run: pip3 install -r tests/device-testing/requirements.txt
      
      - name: Start MDM server
        run: |
          make run &
          sleep 10
      
      - name: Run Android tests
        run: ./tests/device-testing/scripts/run_all_tests.sh android
      
      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: test-results
          path: tests/device-testing/results/
```

## Troubleshooting

### macOS VM: SSH Connection Failed
```bash
# On macOS VM, verify SSH is enabled:
sudo systemsetup -getremotelogin

# Enable if needed:
sudo systemsetup -setremotelogin on
```

### Android: Emulator Not Starting
```bash
# List available emulators:
emulator -list-avds

# Start manually:
emulator -avd LocalMDM-Test

# Check for errors:
adb logcat
```

### MDM Server Not Accessible
```bash
# Check server is running:
curl http://localhost:8080/health

# Check firewall:
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate

# Get your IP (for VMs):
ifconfig | grep "inet " | grep -v 127.0.0.1
```

### Tests Failing
```bash
# Check logs:
cat results/test_run_<timestamp>.log

# Check screenshots:
open results/screenshots/

# Run with verbose output:
python3 -v scripts/test_macos_enrollment.py
```

## Architecture

```
tests/device-testing/
├── scripts/
│   ├── setup_vms.sh              # One-time VM setup
│   ├── test_macos_enrollment.py  # macOS automated tests
│   ├── test_android_enrollment.py # Android automated tests
│   └── run_all_tests.sh          # Master test runner
├── fixtures/
│   ├── policies/                 # Test policy definitions
│   └── profiles/                 # Test configuration profiles
├── results/                      # Test results (gitignored)
│   ├── *.json                    # Test result data
│   ├── *.log                     # Test logs
│   ├── *.html                    # HTML reports
│   └── screenshots/              # Test screenshots
├── images/                       # VM images (gitignored)
│   ├── windows-11-arm.iso
│   └── macos-26.ipsw
├── requirements.txt              # Python dependencies
└── README.md                     # This file
```

## Development

### Adding New Tests

1. Create test script in `scripts/`:
   ```python
   # scripts/test_new_feature.py
   def test_new_feature():
       # Your test code
       pass
   ```

2. Add to `run_all_tests.sh`:
   ```bash
   run_test "New Feature" "$SCRIPT_DIR/test_new_feature.py"
   ```

3. Run and verify:
   ```bash
   ./scripts/run_all_tests.sh
   ```

### Test Utilities

Common functions available in test scripts:
- `log(message)` - Timestamped logging
- `run_ssh_command(cmd)` - Execute on macOS VM
- `run_adb_command(cmd)` - Execute on Android
- `take_screenshot(name)` - Capture device screen
- `test_mdm_server()` - Verify server accessibility

## Next Steps

1. ✅ Run `./scripts/setup_vms.sh` (one-time setup)
2. ✅ Start MDM server: `make run`
3. ✅ Run tests: `./scripts/run_all_tests.sh`
4. ✅ View results: `open results/report_<timestamp>.html`
5. ✅ Restore VMs from snapshots
6. ✅ Repeat!

## Support

For issues or questions:
1. Check logs in `results/`
2. Check screenshots in `results/screenshots/`
3. Review [DEVICE_TESTING_TOOLING.md](../future/DEVICE_TESTING_TOOLING.md)
4. Check VM connectivity with `./scripts/setup_vms.sh`
