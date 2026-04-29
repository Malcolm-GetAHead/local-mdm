#!/bin/bash
# Regenerate the SMBIOS UUID for a UTM VM clone so Windows gets a unique hardware ID.
# Usage: ./regenerate_vm_uuid.sh "VM Name"
# Run with the VM shut down.

set -e

VM_NAME="${1:?Usage: $0 \"VM Name\"}"
UTM_DIR="$HOME/Library/Containers/com.utmapp.UTM/Data/Documents"
PLIST="$UTM_DIR/$VM_NAME.utm/config.plist"

if [ ! -f "$PLIST" ]; then
    echo "✗ VM not found: $PLIST"
    exit 1
fi

OLD_UUID=$(plutil -extract UUID raw "$PLIST")
NEW_UUID=$(uuidgen)

echo "VM: $VM_NAME"
echo "Old UUID: $OLD_UUID"
echo "New UUID: $NEW_UUID"

plutil -replace UUID -string "$NEW_UUID" "$PLIST"
echo "✓ UUID updated. Boot the VM — Windows will see a new hardware ID."
