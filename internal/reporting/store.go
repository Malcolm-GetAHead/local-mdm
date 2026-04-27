package reporting

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// ReportStore abstracts report data access.
type ReportStore interface {
	DeviceInventory(ctx context.Context, enterpriseID uuid.UUID, platform string) ([]DeviceRow, error)
	ComplianceReport(ctx context.Context, enterpriseID uuid.UUID) ([]ComplianceRow, error)
	EnrollmentReport(ctx context.Context, enterpriseID uuid.UUID, days int) ([]EnrollmentRow, error)
}

// SQLReportStore implements ReportStore using a SQL database.
type SQLReportStore struct {
	db *sql.DB
}

// NewSQLReportStore creates a new SQL-backed report store.
func NewSQLReportStore(db *sql.DB) *SQLReportStore {
	return &SQLReportStore{db: db}
}

func (s *SQLReportStore) DeviceInventory(ctx context.Context, enterpriseID uuid.UUID, platform string) ([]DeviceRow, error) {
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

func (s *SQLReportStore) ComplianceReport(ctx context.Context, enterpriseID uuid.UUID) ([]ComplianceRow, error) {
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

func (s *SQLReportStore) EnrollmentReport(ctx context.Context, enterpriseID uuid.UUID, days int) ([]EnrollmentRow, error) {
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

// Ensure SQLReportStore implements ReportStore at compile time.
var _ ReportStore = (*SQLReportStore)(nil)
