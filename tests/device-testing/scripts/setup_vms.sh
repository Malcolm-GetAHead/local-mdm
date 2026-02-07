#!/bin/bash
# One-time VM setup guide for automated testing with VMware Fusion
# Run this after creating VMs in VMware Fusion

set -e

echo "=== Local MDM VM Setup Guide (VMware Fusion) ==="
echo ""
echo "This script will guide you through setting up VMs for automated testing."
echo "You only need to do this ONCE."
echo ""

# Check if VMware Fusion is installed
if [ ! -d "/Applications/VMware Fusion.app" ]; then
    echo "✗ VMware Fusion not found"
    echo ""
    echo "Install VMware Fusion Pro (FREE for personal use):"
    echo "  brew install --cask vmware-fusion"
    echo "  OR download from: https://support.broadcom.com/group/ecx/productdownloads?subfamily=VMware+Fusion"
    echo ""
    exit 1
fi

echo "✓ VMware Fusion installed"
echo ""

# Check if VMs exist
echo "Step 1: Verify VMs are created in VMware Fusion"
echo "----------------------------------------"
echo "Required VMs:"
echo "  - LocalMDM-macOS-Test (macOS 26)"
echo "  - LocalMDM-Windows-Test (Windows 11 ARM)"
echo ""
read -p "Have you created both VMs in VMware Fusion? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Please create VMs first using VMware Fusion GUI, then run this script again."
    exit 1
fi

# Get host IP
HOST_IP=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}' | head -1)
echo ""
echo "Step 2: Network Configuration"
echo "----------------------------------------"
echo "Your Mac's IP address: $HOST_IP"
echo "VMs will access MDM server at: http://$HOST_IP:8080"
echo ""

# Find VM paths
MACOS_VM=$(find ~/Virtual\ Machines.localized -name "LocalMDM-macOS-Test.vmx" 2>/dev/null | head -1)
WINDOWS_VM=$(find ~/Virtual\ Machines.localized -name "LocalMDM-Windows-Test.vmx" 2>/dev/null | head -1)

if [ -z "$MACOS_VM" ]; then
    echo "⚠ macOS VM not found in ~/Virtual Machines.localized/"
    read -p "Enter full path to LocalMDM-macOS-Test.vmx: " MACOS_VM
fi

if [ -z "$WINDOWS_VM" ]; then
    echo "⚠ Windows VM not found in ~/Virtual Machines.localized/"
    read -p "Enter full path to LocalMDM-Windows-Test.vmx: " WINDOWS_VM
fi

echo "macOS VM: $MACOS_VM" > tests/device-testing/.vm_config
echo "Windows VM: $WINDOWS_VM" >> tests/device-testing/.vm_config

# macOS VM setup
echo ""
echo "Step 3: macOS VM Setup"
echo "----------------------------------------"
echo "Start the macOS VM in VMware Fusion, then:"
echo ""
echo "1. Complete macOS setup (create user: testuser, password: test1234)"
echo "2. Open Terminal in VM and run:"
echo ""
echo "   # Enable SSH"
echo "   sudo systemsetup -setremotelogin on"
echo ""
echo "3. Get VM's IP address:"
echo "   ifconfig | grep 'inet '"
echo ""
read -p "Enter macOS VM IP address: " MACOS_IP
echo "macOS VM IP: $MACOS_IP" >> tests/device-testing/.vm_config

# Test SSH connection
echo ""
echo "Testing SSH connection to macOS VM..."
if ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no testuser@$MACOS_IP "echo 'SSH works'" 2>/dev/null; then
    echo "✓ SSH connection successful"
else
    echo "✗ SSH connection failed. Please verify:"
    echo "  - VM is running"
    echo "  - SSH is enabled"
    echo "  - IP address is correct"
    exit 1
fi

# Windows VM setup
echo ""
echo "Step 4: Windows VM Setup"
echo "----------------------------------------"
echo "Start the Windows VM in VMware Fusion, then:"
echo ""
echo "1. Complete Windows setup (create user: testuser, password: Test1234!)"
echo "2. Open PowerShell as Administrator and run:"
echo ""
echo "   # Enable WinRM"
echo "   Enable-PSRemoting -Force"
echo "   Set-Item WSMan:\localhost\Client\TrustedHosts -Value '$HOST_IP' -Force"
echo ""
echo "3. Get VM's IP address:"
echo "   ipconfig"
echo ""
read -p "Enter Windows VM IP address: " WINDOWS_IP
echo "Windows VM IP: $WINDOWS_IP" >> tests/device-testing/.vm_config

# Create SSH config
echo ""
echo "Step 5: Creating SSH config"
echo "----------------------------------------"
cat > ~/.ssh/config.localmdm << EOF
Host localmdm-macos
    HostName $MACOS_IP
    User testuser
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null

Host localmdm-windows
    HostName $WINDOWS_IP
    User testuser
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
EOF

echo "✓ SSH config created at ~/.ssh/config.localmdm"
echo ""
echo "Add this to your ~/.ssh/config:"
echo "  Include ~/.ssh/config.localmdm"
echo ""

# Create snapshots
echo "Step 6: Create VM Snapshots"
echo "----------------------------------------"
echo "Creating snapshots via vmrun..."
echo ""

if vmrun snapshot "$MACOS_VM" ready-for-testing 2>/dev/null; then
    echo "✓ macOS VM snapshot created"
else
    echo "⚠ Failed to create macOS snapshot via CLI"
    echo "  Create manually: VM → Snapshots → Take Snapshot → 'ready-for-testing'"
fi

if vmrun snapshot "$WINDOWS_VM" ready-for-testing 2>/dev/null; then
    echo "✓ Windows VM snapshot created"
else
    echo "⚠ Failed to create Windows snapshot via CLI"
    echo "  Create manually: VM → Snapshots → Take Snapshot → 'ready-for-testing'"
fi

# Summary
echo ""
echo "=== Setup Complete! ==="
echo "----------------------------------------"
echo "Configuration saved to: tests/device-testing/.vm_config"
echo ""
echo "Next steps:"
echo "1. Start Local MDM server: make run"
echo "2. Run tests: ./tests/device-testing/scripts/run_all_tests.sh"
echo ""
echo "VM Access:"
echo "  macOS:   ssh localmdm-macos"
echo "  Windows: ssh localmdm-windows (if SSH installed)"
echo ""
echo "To restore VMs to clean state:"
echo "  vmrun revertToSnapshot \"$MACOS_VM\" ready-for-testing"
echo "  vmrun revertToSnapshot \"$WINDOWS_VM\" ready-for-testing"
