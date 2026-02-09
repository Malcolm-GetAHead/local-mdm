package android

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/api/androidmanagement/v1"
	"google.golang.org/api/option"
)

// Client wraps Google Android Management API client
type Client struct {
	service   *androidmanagement.Service
	projectID string
	logger    *slog.Logger
}

// NewClient creates a new Android Management API client
func NewClient(ctx context.Context, projectID, serviceAccountJSON string, logger *slog.Logger) (*Client, error) {
	service, err := androidmanagement.NewService(ctx, option.WithCredentialsFile(serviceAccountJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create android management service: %w", err)
	}

	return &Client{
		service:   service,
		projectID: projectID,
		logger:    logger,
	}, nil
}

// CreateEnterprise creates or retrieves an enterprise
func (c *Client) CreateEnterprise(ctx context.Context, enterpriseName, signupURL string) (*androidmanagement.Enterprise, error) {
	// In production, this would handle the signup flow
	// For now, we'll create a basic enterprise structure
	
	enterprise := &androidmanagement.Enterprise{
		Name:        enterpriseName,
		EnabledNotificationTypes: []string{
			"ENROLLMENT",
			"COMPLIANCE_REPORT",
			"STATUS_REPORT",
		},
	}

	c.logger.Info("enterprise created", "name", enterpriseName)
	return enterprise, nil
}

// GetEnterprise retrieves an enterprise by name
func (c *Client) GetEnterprise(ctx context.Context, name string) (*androidmanagement.Enterprise, error) {
	enterprise, err := c.service.Enterprises.Get(name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get enterprise: %w", err)
	}
	return enterprise, nil
}

// CreateEnrollmentToken creates an enrollment token
func (c *Client) CreateEnrollmentToken(ctx context.Context, enterpriseName string, policyName string) (*androidmanagement.EnrollmentToken, error) {
	token := &androidmanagement.EnrollmentToken{
		PolicyName: policyName,
		Duration:   "2592000s", // 30 days
	}

	result, err := c.service.Enterprises.EnrollmentTokens.Create(enterpriseName, token).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create enrollment token: %w", err)
	}

	c.logger.Info("enrollment token created",
		"enterprise", enterpriseName,
		"token_name", result.Name,
	)

	return result, nil
}

// ListDevices lists all devices for an enterprise
func (c *Client) ListDevices(ctx context.Context, enterpriseName string) ([]*androidmanagement.Device, error) {
	var devices []*androidmanagement.Device
	
	call := c.service.Enterprises.Devices.List(enterpriseName).Context(ctx)
	
	err := call.Pages(ctx, func(page *androidmanagement.ListDevicesResponse) error {
		devices = append(devices, page.Devices...)
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	return devices, nil
}

// GetDevice retrieves a specific device
func (c *Client) GetDevice(ctx context.Context, deviceName string) (*androidmanagement.Device, error) {
	device, err := c.service.Enterprises.Devices.Get(deviceName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	return device, nil
}

// CreatePolicy creates a device policy
func (c *Client) CreatePolicy(ctx context.Context, enterpriseName, policyID string, policy *androidmanagement.Policy) (*androidmanagement.Policy, error) {
	policyName := fmt.Sprintf("%s/policies/%s", enterpriseName, policyID)
	
	result, err := c.service.Enterprises.Policies.Patch(policyName, policy).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	c.logger.Info("policy created",
		"enterprise", enterpriseName,
		"policy_id", policyID,
	)

	return result, nil
}

// GetPolicy retrieves a policy
func (c *Client) GetPolicy(ctx context.Context, policyName string) (*androidmanagement.Policy, error) {
	policy, err := c.service.Enterprises.Policies.Get(policyName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}
	return policy, nil
}
