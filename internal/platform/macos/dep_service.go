package macos

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/micromdm/nanodep/client"
	"github.com/micromdm/nanodep/godep"
	depsync "github.com/micromdm/nanodep/sync"
)

// DEPService manages Apple DEP integration — profiles, device sync, and assignment.
type DEPService struct {
	storage  DEPStorageInterface
	logger   *slog.Logger
}

// DEPStorageInterface defines the storage operations needed by DEPService.
type DEPStorageInterface interface {
	RetrieveAuthTokens(ctx context.Context, name string) (*client.OAuth1Tokens, error)
	StoreAuthTokens(ctx context.Context, name string, tokens *client.OAuth1Tokens) error
	RetrieveConfig(ctx context.Context, name string) (*client.Config, error)
	StoreConfig(ctx context.Context, name string, cfg *client.Config) error
	RetrieveCursor(ctx context.Context, name string) (string, error)
	StoreCursor(ctx context.Context, name string, cursor string) error
	RetrieveAssignerProfile(ctx context.Context, name string) (string, time.Time, error)
	StoreAssignerProfile(ctx context.Context, name string, profileUUID string) error
	GenerateTokenPKI(ctx context.Context, name string, cn string, validityDays int) ([]byte, error)
	RetrieveCurrentTokenPKI(ctx context.Context, name string) ([]byte, []byte, error)
	RetrieveStagingTokenPKI(ctx context.Context, name string) ([]byte, []byte, error)
	UpstageTokenPKI(ctx context.Context, name string) error
	StoreSyncedDevice(ctx context.Context, depName, serialNumber string, deviceData map[string]interface{}) error
	ListDEPDevices(ctx context.Context, depName string, limit, offset int) ([]DEPDevice, int, error)
}

// NewDEPService creates a new DEP service.
func NewDEPService(storage DEPStorageInterface, logger *slog.Logger) *DEPService {
	return &DEPService{storage: storage, logger: logger}
}

// DEPProfile represents a DEP enrollment profile to be sent to Apple.
type DEPProfile struct {
	ProfileName           string   `json:"profile_name"`
	URL                   string   `json:"url"`
	AllowPairing          bool     `json:"allow_pairing"`
	IsSupervised          bool     `json:"is_supervised"`
	IsMultiUser           bool     `json:"is_multi_user"`
	IsMandatory           bool     `json:"is_mandatory"`
	AwaitDeviceConfigured bool     `json:"await_device_configured"`
	IsMDMRemovable        bool     `json:"is_mdm_removable"`
	AutoAdvanceSetup      bool     `json:"auto_advance_setup"`
	SupportPhoneNumber    string   `json:"support_phone_number,omitempty"`
	SupportEmailAddress   string   `json:"support_email_address,omitempty"`
	OrgMagic              string   `json:"org_magic,omitempty"`
	Department            string   `json:"department,omitempty"`
	SkipSetupItems        []string `json:"skip_setup_items,omitempty"`
}

// DefaultDEPProfile returns a sensible default DEP profile for the given MDM server URL.
func DefaultDEPProfile(serverURL, orgName string) *DEPProfile {
	return &DEPProfile{
		ProfileName:           orgName + " MDM",
		URL:                   serverURL + "/mdm",
		AllowPairing:          true,
		IsSupervised:          true,
		IsMandatory:           true,
		AwaitDeviceConfigured: true,
		IsMDMRemovable:        false,
		SkipSetupItems: []string{
			"AppleID", "Biometric", "Diagnostics", "DisplayTone",
			"Location", "Payment", "Privacy", "Restore", "Siri", "TOS",
		},
	}
}

// StoreConfig stores the DEP configuration (base URL) for a DEP name.
func (s *DEPService) StoreConfig(ctx context.Context, name, baseURL string) error {
	return s.storage.StoreConfig(ctx, name, &client.Config{BaseURL: baseURL})
}

// GenerateTokenPKI generates a keypair for the Apple portal token exchange.
func (s *DEPService) GenerateTokenPKI(ctx context.Context, name string) ([]byte, error) {
	cn := fmt.Sprintf("LocalMDM DEP - %s", name)
	certPEM, err := s.storage.GenerateTokenPKI(ctx, name, cn, 365)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token PKI: %w", err)
	}
	s.logger.Info("generated DEP token PKI", "name", name)
	return certPEM, nil
}

// SetAssignerProfile sets the profile UUID to auto-assign to new DEP devices.
func (s *DEPService) SetAssignerProfile(ctx context.Context, name, profileUUID string) error {
	if err := s.storage.StoreAssignerProfile(ctx, name, profileUUID); err != nil {
		return fmt.Errorf("failed to set assigner profile: %w", err)
	}
	s.logger.Info("set DEP assigner profile", "name", name, "profile_uuid", profileUUID)
	return nil
}

// GetAssignerProfile retrieves the current assigner profile UUID.
func (s *DEPService) GetAssignerProfile(ctx context.Context, name string) (string, time.Time, error) {
	return s.storage.RetrieveAssignerProfile(ctx, name)
}

// ListDevices lists synced DEP devices for a DEP name.
func (s *DEPService) ListDevices(ctx context.Context, name string, limit, offset int) ([]DEPDevice, int, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.storage.ListDEPDevices(ctx, name, limit, offset)
}

// SyncDevicesCallback is a callback for the DEP syncer that stores synced devices
// and auto-assigns profiles.
func (s *DEPService) SyncDevicesCallback(ctx context.Context, isFetch bool, resp *godep.FetchDeviceResponseJson) error {
	if resp == nil {
		return nil
	}

	s.logger.Info("DEP sync callback",
		"devices", len(resp.Devices),
		"more_to_follow", resp.MoreToFollow,
		"is_fetch", isFetch,
	)

	for _, device := range resp.Devices {
		data := deviceToMap(device)
		if err := s.storage.StoreSyncedDevice(ctx, "", device.SerialNumber, data); err != nil {
			s.logger.Error("failed to store synced device", "serial", device.SerialNumber, "error", err)
			continue
		}
	}

	return nil
}

// SyncDevicesCallbackForName returns a callback bound to a specific DEP name.
func (s *DEPService) SyncDevicesCallbackForName(depName string) func(context.Context, bool, *godep.FetchDeviceResponseJson) error {
	return func(ctx context.Context, isFetch bool, resp *godep.FetchDeviceResponseJson) error {
		if resp == nil {
			return nil
		}

		s.logger.Info("DEP sync callback",
			"dep_name", depName,
			"devices", len(resp.Devices),
			"more_to_follow", resp.MoreToFollow,
		)

		for _, device := range resp.Devices {
			data := deviceToMap(device)
			if err := s.storage.StoreSyncedDevice(ctx, depName, device.SerialNumber, data); err != nil {
				s.logger.Error("failed to store synced device",
					"serial", device.SerialNumber, "dep_name", depName, "error", err)
			}
		}

		return nil
	}
}

func deviceToMap(d godep.DeviceJson) map[string]interface{} {
	b, err := json.Marshal(d)
	if err != nil {
		return map[string]interface{}{"serial_number": d.SerialNumber}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]interface{}{"serial_number": d.SerialNumber}
	}
	return m
}

// StartDEPSync starts a background goroutine that periodically syncs devices
// from Apple DEP. Returns a cancel function to stop the sync loop.
// If tokens are not configured, the sync will fail gracefully and log errors.
func (s *DEPService) StartDEPSync(depName string, interval time.Duration) context.CancelFunc {
	if interval <= 0 {
		interval = 30 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		depClient := godep.NewClient(s.storage, godep.WithUserAgent("local-mdm"))
		callback := s.SyncDevicesCallbackForName(depName)

		syncer := depsync.NewSyncer(
			depClient,
			depName,
			s.storage,
			depsync.WithDuration(interval),
			depsync.WithCallback(callback),
		)

		s.logger.Info("DEP sync loop starting",
			"dep_name", depName,
			"interval", interval,
		)

		if err := syncer.Run(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("DEP sync loop exited with error",
				"dep_name", depName,
				"error", err,
			)
		}
	}()

	return cancel
}
