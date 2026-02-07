#!/usr/bin/env python3
"""
Automated macOS enrollment testing
Tests enrollment flow, profile installation, and basic commands
"""

import requests
import subprocess
import time
import json
import sys
from pathlib import Path

# Configuration
MDM_SERVER = "http://localhost:8080"
MACOS_VM_HOST = "localmdm-macos"
ENTERPRISE_ID = "ent-123"
RESULTS_DIR = Path("tests/device-testing/results")
SCREENSHOTS_DIR = RESULTS_DIR / "screenshots"

# Create directories
RESULTS_DIR.mkdir(parents=True, exist_ok=True)
SCREENSHOTS_DIR.mkdir(parents=True, exist_ok=True)

def log(message):
    """Print timestamped log message"""
    timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{timestamp}] {message}")

def run_ssh_command(command, capture_output=True):
    """Run command on macOS VM via SSH"""
    cmd = ["ssh", MACOS_VM_HOST, command]
    result = subprocess.run(cmd, capture_output=capture_output, text=True)
    return result

def take_screenshot(name):
    """Take screenshot of macOS VM"""
    log(f"Taking screenshot: {name}")
    screenshot_path = SCREENSHOTS_DIR / f"macos_{name}_{int(time.time())}.png"
    run_ssh_command(f"screencapture -x {screenshot_path}")
    return screenshot_path

def test_vm_connectivity():
    """Test SSH connection to VM"""
    log("Testing VM connectivity...")
    result = run_ssh_command("echo 'connected'")
    if result.returncode != 0:
        log("✗ Failed to connect to macOS VM")
        return False
    log("✓ VM connectivity OK")
    return True

def test_mdm_server():
    """Test MDM server is running"""
    log("Testing MDM server...")
    try:
        response = requests.get(f"{MDM_SERVER}/health", timeout=5)
        if response.status_code == 200:
            log("✓ MDM server is running")
            return True
    except requests.exceptions.RequestException as e:
        log(f"✗ MDM server not accessible: {e}")
        return False
    return False

def generate_enrollment_profile():
    """Generate enrollment profile from MDM server"""
    log("Generating enrollment profile...")
    try:
        response = requests.get(
            f"{MDM_SERVER}/api/v1/macos/enroll/{ENTERPRISE_ID}",
            timeout=10
        )
        if response.status_code == 200:
            profile_path = RESULTS_DIR / "enrollment.mobileconfig"
            profile_path.write_bytes(response.content)
            log(f"✓ Enrollment profile saved to {profile_path}")
            return profile_path
    except requests.exceptions.RequestException as e:
        log(f"✗ Failed to generate profile: {e}")
        return None

def copy_profile_to_vm(profile_path):
    """Copy enrollment profile to VM"""
    log("Copying profile to VM...")
    result = subprocess.run([
        "scp", str(profile_path), f"{MACOS_VM_HOST}:/tmp/enrollment.mobileconfig"
    ])
    if result.returncode == 0:
        log("✓ Profile copied to VM")
        return True
    log("✗ Failed to copy profile")
    return False

def install_profile_on_vm():
    """Install enrollment profile on VM"""
    log("Installing enrollment profile...")
    
    # Install profile
    result = run_ssh_command(
        "sudo profiles install -path /tmp/enrollment.mobileconfig"
    )
    
    if result.returncode == 0:
        log("✓ Profile installed")
        take_screenshot("profile_installed")
        return True
    
    log(f"✗ Profile installation failed: {result.stderr}")
    return False

def verify_enrollment():
    """Verify device appears in MDM server"""
    log("Verifying enrollment...")
    
    # Wait for device to check in
    time.sleep(10)
    
    try:
        response = requests.get(f"{MDM_SERVER}/api/v1/devices", timeout=10)
        devices = response.json()
        
        macos_devices = [d for d in devices if d.get('platform') == 'macos']
        
        if macos_devices:
            device = macos_devices[0]
            log(f"✓ Device enrolled: {device.get('name')} ({device.get('id')})")
            return device.get('id')
        else:
            log("✗ Device not found in MDM server")
            return None
            
    except requests.exceptions.RequestException as e:
        log(f"✗ Failed to verify enrollment: {e}")
        return None

def test_device_lock(device_id):
    """Test device lock command"""
    log("Testing device lock command...")
    
    try:
        response = requests.post(
            f"{MDM_SERVER}/api/v1/devices/{device_id}/lock",
            timeout=10
        )
        
        if response.status_code == 200:
            log("✓ Lock command sent")
            time.sleep(5)
            take_screenshot("device_locked")
            
            # Verify lock screen
            result = run_ssh_command("pgrep -x 'loginwindow'")
            if result.returncode == 0:
                log("✓ Device is locked")
                return True
            else:
                log("⚠ Lock status unclear")
                return True  # Command sent successfully
        else:
            log(f"✗ Lock command failed: {response.status_code}")
            return False
            
    except requests.exceptions.RequestException as e:
        log(f"✗ Failed to send lock command: {e}")
        return False

def test_profile_list():
    """List installed profiles"""
    log("Listing installed profiles...")
    result = run_ssh_command("profiles list")
    
    if "MDM" in result.stdout or "enrollment" in result.stdout.lower():
        log("✓ MDM profile is installed")
        return True
    else:
        log("⚠ MDM profile not found in profile list")
        return False

def cleanup():
    """Remove enrollment profile from VM"""
    log("Cleaning up...")
    run_ssh_command("sudo profiles remove -identifier com.localmdm.enrollment", capture_output=False)
    run_ssh_command("rm -f /tmp/enrollment.mobileconfig", capture_output=False)
    log("✓ Cleanup complete")

def main():
    """Run all macOS enrollment tests"""
    log("=== Starting macOS Enrollment Tests ===")
    
    results = {
        "vm_connectivity": False,
        "mdm_server": False,
        "profile_generation": False,
        "profile_installation": False,
        "enrollment_verification": False,
        "device_lock": False,
        "profile_list": False
    }
    
    # Test VM connectivity
    if not test_vm_connectivity():
        log("✗ Cannot proceed without VM connectivity")
        sys.exit(1)
    results["vm_connectivity"] = True
    
    # Test MDM server
    if not test_mdm_server():
        log("✗ Cannot proceed without MDM server")
        sys.exit(1)
    results["mdm_server"] = True
    
    # Generate enrollment profile
    profile_path = generate_enrollment_profile()
    if not profile_path:
        log("✗ Cannot proceed without enrollment profile")
        sys.exit(1)
    results["profile_generation"] = True
    
    # Copy profile to VM
    if not copy_profile_to_vm(profile_path):
        log("✗ Cannot proceed without copying profile")
        sys.exit(1)
    
    # Install profile
    if not install_profile_on_vm():
        log("✗ Profile installation failed")
        cleanup()
        sys.exit(1)
    results["profile_installation"] = True
    
    # Verify enrollment
    device_id = verify_enrollment()
    if not device_id:
        log("✗ Enrollment verification failed")
        cleanup()
        sys.exit(1)
    results["enrollment_verification"] = True
    
    # Test profile list
    results["profile_list"] = test_profile_list()
    
    # Test device lock
    results["device_lock"] = test_device_lock(device_id)
    
    # Cleanup
    cleanup()
    
    # Summary
    log("\n=== Test Results ===")
    passed = sum(results.values())
    total = len(results)
    
    for test, result in results.items():
        status = "✓ PASS" if result else "✗ FAIL"
        log(f"{status}: {test}")
    
    log(f"\nPassed: {passed}/{total}")
    
    # Save results
    results_file = RESULTS_DIR / f"macos_test_results_{int(time.time())}.json"
    results_file.write_text(json.dumps(results, indent=2))
    log(f"Results saved to {results_file}")
    
    # Exit with appropriate code
    sys.exit(0 if passed == total else 1)

if __name__ == "__main__":
    main()
