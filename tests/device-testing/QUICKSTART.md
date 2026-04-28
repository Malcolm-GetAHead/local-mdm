# Quick Start: Automated Device Testing

## 🚀 Get Testing in 3 Steps

### Step 1: One-Time VM Setup (30 minutes)

```bash
# Install UTM (free, native macOS VM manager)
brew install --cask utm

# Run the setup wizard
cd tests/device-testing
./scripts/setup_vms.sh
```

This will guide you through:
- ✅ Creating macOS VM (from IPSW) and Windows VM (from ISO) in UTM
- ✅ Enabling SSH on macOS VM
- ✅ Enabling WinRM on Windows VM
- ✅ Recording VM IP addresses

### Step 2: Install Dependencies (2 minutes)

```bash
pip3 install -r requirements.txt
python3 -c "import requests; print('✓ Dependencies OK')"
```

### Step 3: Run Tests! (5 minutes)

```bash
# Start MDM server (in another terminal)
make docker-up && sleep 45 && make migrate-up && make seed && make run

# Run all tests
./scripts/run_all_tests.sh

# Or run specific platform
./scripts/run_all_tests.sh macos
./scripts/run_all_tests.sh windows
./scripts/run_all_tests.sh android
```

## 🔄 After Testing

Restore VMs to clean state:
```bash
./scripts/restore_vms.sh
```

This deletes the test VMs and re-clones from templates. To set up templates:
1. Configure a VM fully (OS installed, SSH/WinRM enabled)
2. In UTM, right-click the VM → Clone
3. Rename the original to `LocalMDM-macOS-Test-Template` (or Windows equivalent)
4. Use the clone as `LocalMDM-macOS-Test` for testing

## 📚 Full Documentation

See [README.md](README.md) for complete documentation.
