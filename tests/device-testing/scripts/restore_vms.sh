#!/bin/bash
# Restore VMs to clean state before testing
# UTM doesn't support snapshots — we use clone + delete pattern
# or APFS instant clone (cp -c) on the .utm bundle

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/../.vm_config"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "✗ VM config not found. Run ./scripts/setup_vms.sh first"
    exit 1
fi

source "$CONFIG_FILE"

echo "=== Restoring VMs to Clean State ==="
echo ""

# Stop VMs if running
echo "Stopping VMs..."
utmctl stop "$MACOS_VM_NAME" 2>/dev/null || true
utmctl stop "$WINDOWS_VM_NAME" 2>/dev/null || true
sleep 3

# UTM stores VMs in ~/Library/Containers/com.utmapp.UTM/Data/Documents/
# or ~/Library/Group Containers/WDNLXAD4W8/Library/Containers/com.utmapp.UTM/Data/Documents/
UTM_DIR="$HOME/Library/Containers/com.utmapp.UTM/Data/Documents"
if [ ! -d "$UTM_DIR" ]; then
    UTM_DIR=$(find "$HOME/Library" -name "*.utm" -path "*/UTM/*" -maxdepth 6 2>/dev/null | head -1 | xargs dirname)
fi

if [ -z "$UTM_DIR" ] || [ ! -d "$UTM_DIR" ]; then
    echo "✗ Cannot find UTM VM directory"
    echo "  Manually delete test VMs in UTM and clone from templates"
    exit 1
fi

echo "UTM directory: $UTM_DIR"

# For each VM: delete the test clone, re-clone from template
for VM_NAME in "$MACOS_VM_NAME" "$WINDOWS_VM_NAME"; do
    TEMPLATE="${VM_NAME}-Template"
    
    if utmctl list 2>/dev/null | grep -q "$TEMPLATE"; then
        echo "Deleting $VM_NAME..."
        utmctl delete "$VM_NAME" 2>/dev/null || true
        sleep 1
        
        echo "Cloning from $TEMPLATE..."
        utmctl clone "$TEMPLATE" --name "$VM_NAME" 2>/dev/null
        echo "✓ $VM_NAME restored from template"
    else
        echo "⚠ Template '$TEMPLATE' not found — skipping $VM_NAME"
        echo "  To create a template: in UTM, right-click the configured VM → Clone"
        echo "  Rename the original to '${VM_NAME}-Template' and the clone to '$VM_NAME'"
    fi
done

echo ""
echo "✓ VMs restored"
echo ""
echo "Start VMs:"
echo "  utmctl start \"$MACOS_VM_NAME\""
echo "  utmctl start \"$WINDOWS_VM_NAME\""
