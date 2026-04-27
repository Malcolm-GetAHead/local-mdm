package reporting

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Service generates reports by delegating to a ReportStore.
type Service struct {
	store ReportStore
}

// NewService creates a reporting service.
func NewService(store ReportStore) *Service {
	return &Service{store: store}
}

// DeviceRow represents a row in the device inventory report.
type DeviceRow struct {
	ID           uuid.UUID  `json:"id"`
	Platform     string     `json:"platform"`
	Name         string     `json:"name"`
	SerialNumber string     `json:"serial_number"`
	OSVersion    string     `json:"os_version"`
	Status       string     `json:"status"`
	LastSeen     *time.Time `json:"last_seen"`
	EnrolledAt   time.Time  `json:"enrolled_at"`
}

// ComplianceRow represents a row in the compliance report.
type ComplianceRow struct {
	DeviceID    uuid.UUID    `json:"device_id"`
	DeviceName  string       `json:"device_name"`
	Platform    string       `json:"platform"`
	PolicyName  string       `json:"policy_name"`
	Status      string       `json:"status"`
	EvaluatedAt time.Time    `json:"evaluated_at"`
	Details     models.JSONB `json:"details"`
}

// EnrollmentRow represents a row in the enrollment report.
type EnrollmentRow struct {
	Date     string `json:"date"`
	Platform string `json:"platform"`
	Count    int    `json:"count"`
}

// DeviceInventory returns device inventory filtered by platform.
func (s *Service) DeviceInventory(ctx context.Context, enterpriseID uuid.UUID, platform string) ([]DeviceRow, error) {
	return s.store.DeviceInventory(ctx, enterpriseID, platform)
}

// ComplianceReport returns compliance status per device/policy.
func (s *Service) ComplianceReport(ctx context.Context, enterpriseID uuid.UUID) ([]ComplianceRow, error) {
	return s.store.ComplianceReport(ctx, enterpriseID)
}

// EnrollmentReport returns enrollment counts grouped by day and platform.
func (s *Service) EnrollmentReport(ctx context.Context, enterpriseID uuid.UUID, days int) ([]EnrollmentRow, error) {
	if days <= 0 {
		days = 30
	}
	return s.store.EnrollmentReport(ctx, enterpriseID, days)
}

// WriteCSV writes report rows as CSV.
func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteJSON writes report data as JSON.
func WriteJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
