#!/bin/bash
# Restore VMs to clean state before testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/../.vm_config"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "✗ VM config not found. Run ./scripts/setup_vms.sh first"
    exit 1
fi

# Load VM paths
MACOS_VM=$(grep "macOS VM:" "$CONFIG_FILE" | cut -d' ' -f3-)
WINDOWS_VM=$(grep "Windows VM:" "$CONFIG_FILE" | cut -d' ' -f3-)

echo "=== Restoring VMs to Clean State ==="
echo ""

# Stop VMs if running
echo "Stopping VMs..."
vmrun stop "$MACOS_VM" soft 2>/dev/null || true
vmrun stop "$WINDOWS_VM" soft 2>/dev/null || true
sleep 5

# Restore snapshots
echo "Restoring macOS VM..."
if vmrun revertToSnapshot "$MACOS_VM" ready-for-testing; then
    echo "✓ macOS VM restored"
else
    echo "✗ Failed to restore macOS VM"
    exit 1
fi

echo "Restoring Windows VM..."
if vmrun revertToSnapshot "$WINDOWS_VM" ready-for-testing; then
    echo "✓ Windows VM restored"
else
    echo "✗ Failed to restore Windows VM"
    exit 1
fi

echo ""
echo "✓ VMs restored to clean state"
echo ""
echo "Start VMs with:"
echo "  vmrun start \"$MACOS_VM\" nogui"
echo "  vmrun start \"$WINDOWS_VM\" nogui"
