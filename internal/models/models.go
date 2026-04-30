package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JSONB is a custom type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// Enterprise represents an organization/tenant
type Enterprise struct {
	BaseModel
	Name     string `json:"name" db:"name"`
	Slug     string `json:"slug" db:"slug"`
	Settings JSONB  `json:"settings" db:"settings"`
}

// User represents an admin user
type User struct {
	BaseModel
	EnterpriseID uuid.UUID  `json:"enterprise_id" db:"enterprise_id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	FullName     string     `json:"full_name" db:"full_name"`
	Role         string     `json:"role" db:"role"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
}

// Device represents an enrolled device
type Device struct {
	BaseModel
	EnterpriseID   uuid.UUID  `json:"enterprise_id" db:"enterprise_id"`
	Platform       string     `json:"platform" db:"platform"`
	DeviceID       string     `json:"device_id" db:"device_id"`
	SerialNumber   string     `json:"serial_number" db:"serial_number"`
	Name           string     `json:"name" db:"name"`
	Model          string     `json:"model" db:"model"`
	OSVersion      string     `json:"os_version" db:"os_version"`
	EnrollmentDate time.Time  `json:"enrollment_date" db:"enrollment_date"`
	LastSeen       *time.Time `json:"last_seen,omitempty" db:"last_seen"`
	Status         string     `json:"status" db:"status"`
	PlatformData   JSONB      `json:"platform_data" db:"platform_data"`
}

// Policy represents a management policy
type Policy struct {
	BaseModel
	EnterpriseID uuid.UUID `json:"enterprise_id" db:"enterprise_id"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	Platform     string    `json:"platform" db:"platform"`
	PolicyType   string    `json:"policy_type" db:"policy_type"`
	PolicyConfig JSONB     `json:"policy_config" db:"policy_config"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	IsTemplate   bool      `json:"is_template" db:"is_template"`
}

// DevicePolicy represents the junction between devices and policies
type DevicePolicy struct {
	BaseModel
	DeviceID     uuid.UUID  `json:"device_id" db:"device_id"`
	PolicyID     uuid.UUID  `json:"policy_id" db:"policy_id"`
	AppliedAt    time.Time  `json:"applied_at" db:"applied_at"`
	Status       string     `json:"status" db:"status"`
	ErrorMessage string     `json:"error_message,omitempty" db:"error_message"`
}

// Certificate represents a device or CA certificate
type Certificate struct {
	BaseModel
	DeviceID       *uuid.UUID `json:"device_id,omitempty" db:"device_id"`
	CertType       string     `json:"cert_type" db:"cert_type"`
	Subject        string     `json:"subject" db:"subject"`
	SerialNumber   string     `json:"serial_number" db:"serial_number"`
	CertData       string     `json:"cert_data" db:"cert_data"`
	PrivateKeyData string     `json:"-" db:"private_key_data"`
	IssuedAt       time.Time  `json:"issued_at" db:"issued_at"`
	ExpiresAt      time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// APIToken represents an API access token
type APIToken struct {
	BaseModel
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	Name       string     `json:"name" db:"name"`
	TokenHash  string     `json:"-" db:"token_hash"`
	Scopes     JSONB      `json:"scopes" db:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	EnterpriseID *uuid.UUID `json:"enterprise_id,omitempty" db:"enterprise_id"`
	UserID       *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Action       string     `json:"action" db:"action"`
	ResourceType string     `json:"resource_type" db:"resource_type"`
	ResourceID   *uuid.UUID `json:"resource_id,omitempty" db:"resource_id"`
	Details      JSONB      `json:"details" db:"details"`
	IPAddress    string     `json:"ip_address" db:"ip_address"`
	UserAgent    string     `json:"user_agent" db:"user_agent"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// Platform constants
const (
	PlatformWindows = "windows"
	PlatformMacOS   = "macos"
	PlatformAndroid = "android"
	PlatformAll     = "all"
)

// Device status constants
const (
	DeviceStatusPending    = "pending"
	DeviceStatusEnrolled   = "enrolled"
	DeviceStatusUnenrolled = "unenrolled"
	DeviceStatusWiped      = "wiped"
	DeviceStatusLost       = "lost"
)

// Policy type constants
const (
	PolicyTypeWiFi        = "wifi"
	PolicyTypeVPN         = "vpn"
	PolicyTypeSecurity    = "security"
	PolicyTypeApp         = "app"
	PolicyTypeRestriction = "restriction"
	PolicyTypeCompliance  = "compliance"
)

// User role constants
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleViewer     = "viewer"
)

// Certificate type constants
const (
	CertTypeCA     = "ca"
	CertTypeDevice = "device"
	CertTypeAPNS   = "apns"
)

// App represents an application in the enterprise catalog
type App struct {
	BaseModel
	EnterpriseID uuid.UUID `json:"enterprise_id" db:"enterprise_id"`
	Name         string    `json:"name" db:"name"`
	Platform     string    `json:"platform" db:"platform"`
	Identifier   string    `json:"identifier" db:"identifier"` // bundle ID, package name, or MSI product code
	Version      string    `json:"version" db:"version"`
	InstallType  string    `json:"install_type" db:"install_type"` // required, available
	AppConfig    JSONB     `json:"app_config" db:"app_config"`
}

// App install type constants
const (
	AppInstallRequired  = "required"
	AppInstallAvailable = "available"
)

// DeviceGroup represents a static group of devices
type DeviceGroup struct {
	BaseModel
	EnterpriseID uuid.UUID `json:"enterprise_id" db:"enterprise_id"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
}

// GroupMembership represents a device's membership in a group
type GroupMembership struct {
	GroupID  uuid.UUID `json:"group_id" db:"group_id"`
	DeviceID uuid.UUID `json:"device_id" db:"device_id"`
	AddedAt  time.Time `json:"added_at" db:"added_at"`
}

// PolicyAssignment represents a policy assigned to a target (device, group, or enterprise)
type PolicyAssignment struct {
	ID         uuid.UUID `json:"id" db:"id"`
	PolicyID   uuid.UUID `json:"policy_id" db:"policy_id"`
	TargetType string    `json:"target_type" db:"target_type"` // device, group, enterprise
	TargetID   uuid.UUID `json:"target_id" db:"target_id"`
	Priority   int       `json:"priority" db:"priority"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Policy assignment target types
const (
	TargetTypeDevice     = "device"
	TargetTypeGroup      = "group"
	TargetTypeEnterprise = "enterprise"
)

// ComplianceResult represents a device's compliance state for a policy
type ComplianceResult struct {
	ID          uuid.UUID `json:"id" db:"id"`
	DeviceID    uuid.UUID `json:"device_id" db:"device_id"`
	PolicyID    uuid.UUID `json:"policy_id" db:"policy_id"`
	Status      string    `json:"status" db:"status"`
	Details     JSONB     `json:"details" db:"details"`
	EvaluatedAt time.Time `json:"evaluated_at" db:"evaluated_at"`
}

// Compliance status constants
const (
	ComplianceStatusCompliant    = "compliant"
	ComplianceStatusNonCompliant = "non_compliant"
	ComplianceStatusUnknown      = "unknown"
	ComplianceStatusError        = "error"
)

// ComplianceSummary holds enterprise-wide compliance counts.
type ComplianceSummary struct {
	Compliant    int `json:"compliant"`
	NonCompliant int `json:"non_compliant"`
	Unknown      int `json:"unknown"`
	Error        int `json:"error"`
	Total        int `json:"total"`
}

// DeviceApp tracks an app's installation state on a device
type DeviceApp struct {
	DeviceID         uuid.UUID  `json:"device_id" db:"device_id"`
	AppID            uuid.UUID  `json:"app_id" db:"app_id"`
	InstalledVersion string     `json:"installed_version" db:"installed_version"`
	Status           string     `json:"status" db:"status"`
	InstalledAt      *time.Time `json:"installed_at,omitempty" db:"installed_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// Device app status constants
const (
	DeviceAppPending   = "pending"
	DeviceAppInstalled = "installed"
	DeviceAppFailed    = "failed"
	DeviceAppRemoved   = "removed"
)

// PolicyVersion is a snapshot of a policy at a point in time.
type PolicyVersion struct {
	ID           uuid.UUID `json:"id" db:"id"`
	PolicyID     uuid.UUID `json:"policy_id" db:"policy_id"`
	Version      int       `json:"version" db:"version"`
	PolicyConfig JSONB     `json:"policy_config" db:"policy_config"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	Platform     string    `json:"platform" db:"platform"`
	PolicyType   string    `json:"policy_type" db:"policy_type"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedBy    string    `json:"created_by" db:"created_by"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// DeviceCommand represents a pending or completed management command
type DeviceCommand struct {
	BaseModel
	DeviceID     uuid.UUID  `json:"device_id" db:"device_id"`
	EnterpriseID *uuid.UUID `json:"enterprise_id,omitempty" db:"enterprise_id"`
	CommandType  string     `json:"command_type" db:"command_type"`
	CommandData  JSONB      `json:"command_data" db:"command_data"`
	Status       string     `json:"status" db:"status"`
	SentAt       *time.Time `json:"sent_at,omitempty" db:"sent_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	BatchID      *uuid.UUID `json:"batch_id,omitempty" db:"batch_id"`
	ErrorMessage string     `json:"error_message,omitempty" db:"error_message"`
}

// Command type constants
type EnrollmentToken struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	EnterpriseID  uuid.UUID  `json:"enterprise_id" db:"enterprise_id"`
	Token         string     `json:"token" db:"token"`
	Description   string     `json:"description" db:"description"`
	MaxUses       *int       `json:"max_uses" db:"max_uses"`
	UsesRemaining *int       `json:"uses_remaining" db:"uses_remaining"`
	ExpiresAt     time.Time  `json:"expires_at" db:"expires_at"`
	CreatedBy     *uuid.UUID `json:"created_by" db:"created_by"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

const (
	CommandTypeDeviceInfo      = "device_info"
	CommandTypeLock            = "lock"
	CommandTypeWipe            = "wipe"
	CommandTypeRestart         = "restart"
	CommandTypeShutdown        = "shutdown"
	CommandTypeInstallProfile  = "install_profile"
	CommandTypeRemoveProfile   = "remove_profile"
	CommandTypeInstallApp      = "install_app"
	CommandTypeRemoveApp       = "remove_app"
	CommandTypeProfileList     = "profile_list"
	CommandTypeAppList         = "app_list"
	CommandTypeSecurityInfo    = "security_info"
	CommandTypeCertificateList = "certificate_list"
)

// Command status constants
const (
	CommandStatusPending   = "pending"
	CommandStatusSent      = "sent"
	CommandStatusCompleted = "completed"
	CommandStatusFailed    = "failed"
)
