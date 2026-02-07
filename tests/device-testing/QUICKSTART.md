# Quick Start: Automated Device Testing

## 🚀 Get Testing in 3 Steps

### Step 1: One-Time VM Setup (30 minutes)

```bash
# Install VMware Fusion Pro (FREE for personal use)
brew install --cask vmware-fusion

# Run the setup wizard
cd tests/device-testing
./scripts/setup_vms.sh
```

This will guide you through:
- ✅ Enabling SSH on macOS VM
- ✅ Enabling WinRM on Windows VM  
- ✅ Creating VM snapshots via vmrun
- ✅ Testing connectivity

### Step 2: Install Dependencies (2 minutes)

```bash
# Install Python packages
pip3 install -r requirements.txt

# Verify installation
python3 -c "import requests, qrcode; print('✓ Dependencies OK')"
```

### Step 3: Run Tests! (5 minutes)

```bash
# Start MDM server (in another terminal)
make run

# Run all tests
./scripts/run_all_tests.sh

# Or run specific platform
./scripts/run_all_tests.sh macos
./scripts/run_all_tests.sh android
```

## 📊 View Results

```bash
# Open HTML report
open results/report_*.html

# View screenshots
open results/screenshots/

# Check logs
cat results/test_run_*.log
```

## 🔄 After Testing

Restore VMs to clean state:
```bash
# Via CLI
vmrun revertToSnapshot ~/Virtual\ Machines.localized/LocalMDM-macOS-Test.vmwarevm/LocalMDM-macOS-Test.vmx ready-for-testing

# Or via GUI
# VMware Fusion → VM → Snapshots → Restore "ready-for-testing"
```

## ❓ Troubleshooting

**MDM server not running?**
```bash
make run
curl http://localhost:8080/health
```

**Can't connect to macOS VM?**
```bash
ssh localmdm-macos
# If fails, re-run: ./scripts/setup_vms.sh
```

**Android emulator not starting?**
```bash
emulator -list-avds
emulator -avd LocalMDM-Test
```

## 📚 Full Documentation

See [README.md](README.md) for complete documentation.

## ✅ What Gets Tested

- ✅ Device enrollment (all platforms)
- ✅ Profile/policy installation
- ✅ MDM commands (lock, wipe)
- ✅ Device inventory reporting
- ✅ Certificate issuance
- ✅ Compliance checking

## 🎯 Next Steps

1. Run tests regularly during development
2. Add tests to CI/CD pipeline
3. Create additional test scenarios
4. Test with real devices (optional)

**Happy Testing! 🚀**
