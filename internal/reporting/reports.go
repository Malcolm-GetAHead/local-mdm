package reporting

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Service generates reports from the database.
type Service struct {
	db *sql.DB
}

// NewService creates a reporting service using the writer pool.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// DeviceRow represents a row in the device inventory report.
type DeviceRow struct {
	ID           uuid.UUID `json:"id"`
	Platform     string    `json:"platform"`
	Name         string    `json:"name"`
	SerialNumber string    `json:"serial_number"`
	OSVersion    string    `json:"os_version"`
	Status       string    `json:"status"`
	LastSeen     *time.Time `json:"last_seen"`
	EnrolledAt   time.Time  `json:"enrolled_at"`
}

// DeviceInventory returns device inventory filtered by platform.
func (s *Service) DeviceInventory(ctx context.Context, enterpriseID uuid.UUID, platform string) ([]DeviceRow, error) {
	query := `SELECT id, platform, COALESCE(name,''), COALESCE(serial_number,''), COALESCE(os_version,''), status, last_seen, enrollment_date
		FROM devices WHERE enterprise_id = $1 AND deleted_at IS NULL`
	args := []interface{}{enterpriseID}
	if platform != "" {
		query += " AND platform = $2"
		args = append(args, platform)
	}
	query += " ORDER BY name ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DeviceRow
	for rows.Next() {
		var d DeviceRow
		if err := rows.Scan(&d.ID, &d.Platform, &d.Name, &d.SerialNumber, &d.OSVersion, &d.Status, &d.LastSeen, &d.EnrolledAt); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// ComplianceRow represents a row in the compliance report.
type ComplianceRow struct {
	DeviceID    uuid.UUID      `json:"device_id"`
	DeviceName  string         `json:"device_name"`
	Platform    string         `json:"platform"`
	PolicyName  string         `json:"policy_name"`
	Status      string         `json:"status"`
	EvaluatedAt time.Time      `json:"evaluated_at"`
	Details     models.JSONB   `json:"details"`
}

// ComplianceReport returns compliance status per device/policy.
func (s *Service) ComplianceReport(ctx context.Context, enterpriseID uuid.UUID) ([]ComplianceRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cr.device_id, COALESCE(d.name,''), d.platform, COALESCE(p.name,''), cr.status, cr.evaluated_at, cr.details
		 FROM compliance_results cr
		 JOIN devices d ON cr.device_id = d.id
		 JOIN policies p ON cr.policy_id = p.id
		 WHERE d.enterprise_id = $1 AND d.deleted_at IS NULL
		 ORDER BY d.name, p.name`, enterpriseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ComplianceRow
	for rows.Next() {
		var c ComplianceRow
		if err := rows.Scan(&c.DeviceID, &c.DeviceName, &c.Platform, &c.PolicyName, &c.Status, &c.EvaluatedAt, &c.Details); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// EnrollmentRow represents a row in the enrollment report.
type EnrollmentRow struct {
	Date     string `json:"date"`
	Platform string `json:"platform"`
	Count    int    `json:"count"`
}

// EnrollmentReport returns enrollment counts grouped by day and platform.
func (s *Service) EnrollmentReport(ctx context.Context, enterpriseID uuid.UUID, days int) ([]EnrollmentRow, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT DATE(enrollment_date) as d, platform, COUNT(*)
		 FROM devices WHERE enterprise_id = $1 AND deleted_at IS NULL
		 AND enrollment_date >= NOW() - $2::interval
		 GROUP BY d, platform ORDER BY d DESC, platform`,
		enterpriseID, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []EnrollmentRow
	for rows.Next() {
		var e EnrollmentRow
		if err := rows.Scan(&e.Date, &e.Platform, &e.Count); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
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
