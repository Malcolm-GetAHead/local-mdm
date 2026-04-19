package android

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/androidmanagement/v1"
)

// AppManager handles app deployment via Android Management API policies.
type AppManager struct {
	service *androidmanagement.Service
}

// NewAppManager creates an AppManager.
func NewAppManager(service *androidmanagement.Service) *AppManager {
	return &AppManager{service: service}
}

// DeployApp adds an app to a policy's application list as FORCE_INSTALLED.
func (m *AppManager) DeployApp(ctx context.Context, policyName, packageName, installType string) error {
	if installType == "" {
		installType = "FORCE_INSTALLED"
	}

	policy, err := m.service.Enterprises.Policies.Get(policyName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get policy %s: %w", policyName, err)
	}

	for _, app := range policy.Applications {
		if app.PackageName == packageName {
			app.InstallType = installType
			_, err := m.service.Enterprises.Policies.Patch(policyName, policy).Context(ctx).Do()
			return err
		}
	}

	policy.Applications = append(policy.Applications, &androidmanagement.ApplicationPolicy{
		PackageName:             packageName,
		InstallType:             installType,
		DefaultPermissionPolicy: "GRANT",
	})

	_, err = m.service.Enterprises.Policies.Patch(policyName, policy).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to deploy app %s: %w", packageName, err)
	}
	return nil
}

// RemoveApp removes an app from a policy's application list.
func (m *AppManager) RemoveApp(ctx context.Context, policyName, packageName string) error {
	policy, err := m.service.Enterprises.Policies.Get(policyName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get policy %s: %w", policyName, err)
	}

	var filtered []*androidmanagement.ApplicationPolicy
	for _, app := range policy.Applications {
		if app.PackageName != packageName {
			filtered = append(filtered, app)
		}
	}
	policy.Applications = filtered

	_, err = m.service.Enterprises.Policies.Patch(policyName, policy).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to remove app %s: %w", packageName, err)
	}
	return nil
}

// SetManagedConfig sets managed configuration for an app in a policy.
func (m *AppManager) SetManagedConfig(ctx context.Context, policyName, packageName string, config map[string]interface{}) error {
	policy, err := m.service.Enterprises.Policies.Get(policyName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get policy %s: %w", policyName, err)
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	for _, app := range policy.Applications {
		if app.PackageName == packageName {
			app.ManagedConfiguration = configJSON
			break
		}
	}

	_, err = m.service.Enterprises.Policies.Patch(policyName, policy).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to set managed config for %s: %w", packageName, err)
	}
	return nil
}
