package android

import (
	"context"
	"fmt"

	"google.golang.org/api/androidmanagement/v1"
)

// DeviceCommander issues commands to Android devices via the Management API.
type DeviceCommander struct {
	service *androidmanagement.Service
}

// NewDeviceCommander creates a DeviceCommander from an existing API service.
func NewDeviceCommander(service *androidmanagement.Service) *DeviceCommander {
	return &DeviceCommander{service: service}
}

// LockDevice sends a LOCK command to the device.
func (c *DeviceCommander) LockDevice(ctx context.Context, deviceName string) error {
	cmd := &androidmanagement.Command{Type: "LOCK"}
	_, err := c.service.Enterprises.Devices.IssueCommand(deviceName, cmd).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to lock device %s: %w", deviceName, err)
	}
	return nil
}

// WipeDevice sends a RESET_PASSWORD + wipe command.
// If workProfileOnly is true, only the work profile is removed.
func (c *DeviceCommander) WipeDevice(ctx context.Context, deviceName string, workProfileOnly bool) error {
	wipeType := "WIPE_DATA_FLAGS_UNSPECIFIED"
	if workProfileOnly {
		wipeType = "PRESERVE_RESET_PROTECTION_DATA"
	}
	cmd := &androidmanagement.Command{
		Type:           "RESET_PASSWORD",
		ResetPasswordFlags: []string{wipeType},
	}
	_, err := c.service.Enterprises.Devices.IssueCommand(deviceName, cmd).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to wipe device %s: %w", deviceName, err)
	}
	return nil
}

// RebootDevice sends a REBOOT command to the device.
func (c *DeviceCommander) RebootDevice(ctx context.Context, deviceName string) error {
	cmd := &androidmanagement.Command{Type: "REBOOT"}
	_, err := c.service.Enterprises.Devices.IssueCommand(deviceName, cmd).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to reboot device %s: %w", deviceName, err)
	}
	return nil
}
