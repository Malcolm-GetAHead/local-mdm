package repository

// SQL injection prevention - column whitelists for dynamic queries
// These maps define the ONLY columns that can be used in ORDER BY clauses
// to prevent SQL injection if dynamic sorting is added in the future.

// DeviceOrderColumns defines safe ORDER BY columns for device queries
var DeviceOrderColumns = map[string]string{
	"name":            "name",
	"created_at":      "created_at",
	"updated_at":      "updated_at",
	"status":          "status",
	"platform":        "platform",
	"serial_number":   "serial_number",
	"enrollment_date": "enrollment_date",
	"last_seen":       "last_seen",
}

// EnterpriseOrderColumns defines safe ORDER BY columns for enterprise queries
var EnterpriseOrderColumns = map[string]string{
	"name":       "name",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

// PolicyOrderColumns defines safe ORDER BY columns for policy queries
var PolicyOrderColumns = map[string]string{
	"name":       "name",
	"created_at": "created_at",
	"updated_at": "updated_at",
	"priority":   "priority",
}

// ValidateOrderColumn checks if a column name is in the whitelist
// Returns the validated column name and true if valid, or empty string and false if invalid
func ValidateOrderColumn(column string, whitelist map[string]string) (string, bool) {
	validated, ok := whitelist[column]
	return validated, ok
}

// DefaultOrderColumn returns the default ORDER BY column
func DefaultOrderColumn() string {
	return "created_at"
}
