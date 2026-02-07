#!/usr/bin/env python3
"""
Automated Android enrollment testing
Tests QR code enrollment, policy deployment, and commands
"""

import requests
import subprocess
import time
import json
import sys
from pathlib import Path
import qrcode
import io

# Configuration
MDM_SERVER = "http://10.0.2.2:8080"  # Android emulator host access
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

def run_adb_command(command, capture_output=True):
    """Run ADB command"""
    cmd = ["adb", "shell"] + command.split() if isinstance(command, str) else ["adb"] + command
    result = subprocess.run(cmd, capture_output=capture_output, text=True)
    return result

def check_emulator():
    """Check if Android emulator is running"""
    log("Checking for Android emulator...")
    result = subprocess.run(["adb", "devices"], capture_output=True, text=True)
    
    devices = [line for line in result.stdout.split('\n') if '\tdevice' in line]
    
    if devices:
        log(f"✓ Found {len(devices)} Android device(s)")
        return True
    else:
        log("✗ No Android devices found")
        return False

def start_emulator():
    """Start Android emulator"""
    log("Starting Android emulator...")
    
    # Check if emulator exists
    result = subprocess.run(
        ["emulator", "-list-avds"],
        capture_output=True,
        text=True
    )
    
    avds = result.stdout.strip().split('\n')
    if not avds or avds[0] == '':
        log("✗ No Android emulators found. Create one first:")
        log("  avdmanager create avd -n LocalMDM-Test -k 'system-images;android-33;google_apis;arm64-v8a'")
        return False
    
    avd_name = avds[0]
    log(f"Starting emulator: {avd_name}")
    
    # Start emulator in background
    subprocess.Popen(
        ["emulator", "-avd", avd_name, "-no-snapshot-load"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL
    )
    
    # Wait for device
    log("Waiting for emulator to boot...")
    subprocess.run(["adb", "wait-for-device"], timeout=120)
    
    # Wait for boot complete
    for i in range(60):
        result = run_adb_command("getprop sys.boot_completed")
        if result.stdout.strip() == "1":
            log("✓ Emulator booted successfully")
            time.sleep(5)  # Extra time for system to settle
            return True
        time.sleep(2)
    
    log("✗ Emulator boot timeout")
    return False

def take_screenshot(name):
    """Take screenshot of Android device"""
    log(f"Taking screenshot: {name}")
    screenshot_path = SCREENSHOTS_DIR / f"android_{name}_{int(time.time())}.png"
    subprocess.run([
        "adb", "exec-out", "screencap", "-p"
    ], stdout=open(screenshot_path, 'wb'))
    return screenshot_path

def test_mdm_server():
    """Test MDM server is accessible from emulator"""
    log("Testing MDM server accessibility...")
    
    # Test from emulator
    result = run_adb_command(f"curl -s -o /dev/null -w '%{{http_code}}' {MDM_SERVER}/health")
    
    if "200" in result.stdout:
        log("✓ MDM server accessible from emulator")
        return True
    else:
        log("✗ MDM server not accessible from emulator")
        log(f"  Make sure server is running on host")
        return False

def generate_enrollment_token():
    """Generate Android enrollment token"""
    log("Generating enrollment token...")
    
    try:
        response = requests.post(
            f"http://localhost:8080/api/v1/android/enrollment-token",
            json={"enterprise_id": ENTERPRISE_ID},
            timeout=10
        )
        
        if response.status_code == 200:
            data = response.json()
            token = data.get('token')
            log(f"✓ Enrollment token generated: {token[:20]}...")
            return token
        else:
            log(f"✗ Failed to generate token: {response.status_code}")
            return None
            
    except requests.exceptions.RequestException as e:
        log(f"✗ Failed to generate token: {e}")
        return None

def generate_qr_code(token):
    """Generate QR code for enrollment"""
    log("Generating QR code...")
    
    # Create QR code
    qr = qrcode.QRCode(version=1, box_size=10, border=5)
    qr.add_data(token)
    qr.make(fit=True)
    
    img = qr.make_image(fill_color="black", back_color="white")
    
    # Save QR code
    qr_path = RESULTS_DIR / "enrollment_qr.png"
    img.save(qr_path)
    log(f"✓ QR code saved to {qr_path}")
    
    return qr_path

def simulate_qr_enrollment(token):
    """Simulate QR code enrollment via ADB"""
    log("Simulating QR code enrollment...")
    
    # Open enrollment URL directly (simulates QR scan)
    enrollment_url = f"{MDM_SERVER}/android/enroll?token={token}"
    
    result = run_adb_command(f"am start -a android.intent.action.VIEW -d '{enrollment_url}'")
    
    if result.returncode == 0:
        log("✓ Enrollment intent sent")
        take_screenshot("enrollment_started")
        time.sleep(10)
        return True
    else:
        log("✗ Failed to send enrollment intent")
        return False

def verify_enrollment():
    """Verify device appears in MDM server"""
    log("Verifying enrollment...")
    
    # Wait for enrollment to complete
    time.sleep(15)
    
    try:
        response = requests.get("http://localhost:8080/api/v1/devices", timeout=10)
        devices = response.json()
        
        android_devices = [d for d in devices if d.get('platform') == 'android']
        
        if android_devices:
            device = android_devices[0]
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
            f"http://localhost:8080/api/v1/devices/{device_id}/lock",
            timeout=10
        )
        
        if response.status_code == 200:
            log("✓ Lock command sent")
            time.sleep(5)
            take_screenshot("device_locked")
            return True
        else:
            log(f"✗ Lock command failed: {response.status_code}")
            return False
            
    except requests.exceptions.RequestException as e:
        log(f"✗ Failed to send lock command: {e}")
        return False

def get_device_info():
    """Get Android device information"""
    log("Getting device information...")
    
    info = {}
    
    # Get device model
    result = run_adb_command("getprop ro.product.model")
    info['model'] = result.stdout.strip()
    
    # Get Android version
    result = run_adb_command("getprop ro.build.version.release")
    info['android_version'] = result.stdout.strip()
    
    # Get serial number
    result = run_adb_command("getprop ro.serialno")
    info['serial'] = result.stdout.strip()
    
    log(f"✓ Device: {info['model']}, Android {info['android_version']}")
    return info

def main():
    """Run all Android enrollment tests"""
    log("=== Starting Android Enrollment Tests ===")
    
    results = {
        "emulator_running": False,
        "mdm_server": False,
        "token_generation": False,
        "qr_generation": False,
        "enrollment": False,
        "enrollment_verification": False,
        "device_lock": False
    }
    
    # Check/start emulator
    if not check_emulator():
        if not start_emulator():
            log("✗ Cannot proceed without emulator")
            sys.exit(1)
    results["emulator_running"] = True
    
    # Get device info
    device_info = get_device_info()
    
    # Test MDM server
    if not test_mdm_server():
        log("✗ Cannot proceed without MDM server")
        sys.exit(1)
    results["mdm_server"] = True
    
    # Generate enrollment token
    token = generate_enrollment_token()
    if not token:
        log("✗ Cannot proceed without enrollment token")
        sys.exit(1)
    results["token_generation"] = True
    
    # Generate QR code
    qr_path = generate_qr_code(token)
    if qr_path:
        results["qr_generation"] = True
    
    # Simulate enrollment
    if not simulate_qr_enrollment(token):
        log("✗ Enrollment simulation failed")
        sys.exit(1)
    results["enrollment"] = True
    
    # Verify enrollment
    device_id = verify_enrollment()
    if not device_id:
        log("✗ Enrollment verification failed")
        sys.exit(1)
    results["enrollment_verification"] = True
    
    # Test device lock
    results["device_lock"] = test_device_lock(device_id)
    
    # Summary
    log("\n=== Test Results ===")
    passed = sum(results.values())
    total = len(results)
    
    for test, result in results.items():
        status = "✓ PASS" if result else "✗ FAIL"
        log(f"{status}: {test}")
    
    log(f"\nPassed: {passed}/{total}")
    log(f"Device: {device_info['model']} (Android {device_info['android_version']})")
    
    # Save results
    results_data = {
        "results": results,
        "device_info": device_info,
        "timestamp": time.time()
    }
    results_file = RESULTS_DIR / f"android_test_results_{int(time.time())}.json"
    results_file.write_text(json.dumps(results_data, indent=2))
    log(f"Results saved to {results_file}")
    
    # Exit with appropriate code
    sys.exit(0 if passed == total else 1)

if __name__ == "__main__":
    main()
