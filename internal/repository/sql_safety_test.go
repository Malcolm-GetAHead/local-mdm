package repository

import (
	"testing"
)

func TestValidateOrderColumn(t *testing.T) {
	tests := []struct {
		name      string
		column    string
		whitelist map[string]string
		wantCol   string
		wantOK    bool
	}{
		{
			name:      "valid column",
			column:    "name",
			whitelist: map[string]string{"name": "name", "created_at": "created_at"},
			wantCol:   "name",
			wantOK:    true,
		},
		{
			name:      "invalid column",
			column:    "DROP TABLE",
			whitelist: map[string]string{"name": "name", "created_at": "created_at"},
			wantCol:   "",
			wantOK:    false,
		},
		{
			name:      "sql injection attempt",
			column:    "name; DROP TABLE devices; --",
			whitelist: map[string]string{"name": "name", "created_at": "created_at"},
			wantCol:   "",
			wantOK:    false,
		},
		{
			name:      "empty column",
			column:    "",
			whitelist: map[string]string{"name": "name", "created_at": "created_at"},
			wantCol:   "",
			wantOK:    false,
		},
		{
			name:      "case sensitive - uppercase rejected",
			column:    "NAME",
			whitelist: map[string]string{"name": "name", "created_at": "created_at"},
			wantCol:   "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCol, gotOK := ValidateOrderColumn(tt.column, tt.whitelist)
			if gotCol != tt.wantCol {
				t.Errorf("ValidateOrderColumn() column = %v, want %v", gotCol, tt.wantCol)
			}
			if gotOK != tt.wantOK {
				t.Errorf("ValidateOrderColumn() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestDeviceOrderColumns(t *testing.T) {
	expectedColumns := []string{
		"name", "created_at", "updated_at", "status",
		"platform", "serial_number", "enrollment_date", "last_seen",
	}

	for _, col := range expectedColumns {
		if _, ok := DeviceOrderColumns[col]; !ok {
			t.Errorf("DeviceOrderColumns missing expected column: %s", col)
		}
	}

	// Verify whitelist rejects SQL injection
	_, ok := ValidateOrderColumn("name; DROP TABLE devices", DeviceOrderColumns)
	if ok {
		t.Error("DeviceOrderColumns should reject SQL injection attempt")
	}
}

func TestEnterpriseOrderColumns(t *testing.T) {
	expectedColumns := []string{"name", "created_at", "updated_at"}

	for _, col := range expectedColumns {
		if _, ok := EnterpriseOrderColumns[col]; !ok {
			t.Errorf("EnterpriseOrderColumns missing expected column: %s", col)
		}
	}

	// Verify whitelist rejects SQL injection
	_, ok := ValidateOrderColumn("name; DROP TABLE enterprises", EnterpriseOrderColumns)
	if ok {
		t.Error("EnterpriseOrderColumns should reject SQL injection attempt")
	}
}

func TestPolicyOrderColumns(t *testing.T) {
	expectedColumns := []string{"name", "created_at", "updated_at", "priority"}

	for _, col := range expectedColumns {
		if _, ok := PolicyOrderColumns[col]; !ok {
			t.Errorf("PolicyOrderColumns missing expected column: %s", col)
		}
	}

	// Verify whitelist rejects SQL injection
	_, ok := ValidateOrderColumn("name; DROP TABLE policies", PolicyOrderColumns)
	if ok {
		t.Error("PolicyOrderColumns should reject SQL injection attempt")
	}
}

func TestDefaultOrderColumn(t *testing.T) {
	got := DefaultOrderColumn()
	want := "created_at"
	if got != want {
		t.Errorf("DefaultOrderColumn() = %v, want %v", got, want)
	}
}

func TestSQLInjectionPrevention(t *testing.T) {
	// Test various SQL injection attempts
	injectionAttempts := []string{
		"name; DROP TABLE devices; --",
		"name' OR '1'='1",
		"name UNION SELECT * FROM users",
		"name; DELETE FROM devices WHERE 1=1; --",
		"name/**/OR/**/1=1",
		"name' AND 1=1 --",
		"1; UPDATE devices SET status='compromised'",
	}

	for _, attempt := range injectionAttempts {
		t.Run("injection_"+attempt, func(t *testing.T) {
			_, ok := ValidateOrderColumn(attempt, DeviceOrderColumns)
			if ok {
				t.Errorf("ValidateOrderColumn should reject SQL injection: %s", attempt)
			}
		})
	}
}

func TestWhitelistCompleteness(t *testing.T) {
	// Verify all whitelists have created_at as default
	whitelists := []struct {
		name      string
		whitelist map[string]string
	}{
		{"DeviceOrderColumns", DeviceOrderColumns},
		{"EnterpriseOrderColumns", EnterpriseOrderColumns},
		{"PolicyOrderColumns", PolicyOrderColumns},
	}

	for _, wl := range whitelists {
		t.Run(wl.name, func(t *testing.T) {
			if _, ok := wl.whitelist["created_at"]; !ok {
				t.Errorf("%s missing 'created_at' column", wl.name)
			}
			if _, ok := wl.whitelist["updated_at"]; !ok {
				t.Errorf("%s missing 'updated_at' column", wl.name)
			}
		})
	}
}

func TestValidateOrderColumnEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		column    string
		whitelist map[string]string
		wantOK    bool
	}{
		{
			name:      "column with leading whitespace",
			column:    " name",
			whitelist: DeviceOrderColumns,
			wantOK:    false, // Should reject - no trimming
		},
		{
			name:      "column with trailing whitespace",
			column:    "name ",
			whitelist: DeviceOrderColumns,
			wantOK:    false, // Should reject - no trimming
		},
		{
			name:      "column with unicode characters",
			column:    "名前", // Japanese for "name"
			whitelist: DeviceOrderColumns,
			wantOK:    false,
		},
		{
			name:      "very long column name",
			column:    string(make([]byte, 1000)),
			whitelist: DeviceOrderColumns,
			wantOK:    false,
		},
		{
			name:      "column with tab character",
			column:    "name\t",
			whitelist: DeviceOrderColumns,
			wantOK:    false,
		},
		{
			name:      "column with newline",
			column:    "name\n",
			whitelist: DeviceOrderColumns,
			wantOK:    false,
		},
		{
			name:      "null byte in column",
			column:    "name\x00",
			whitelist: DeviceOrderColumns,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotOK := ValidateOrderColumn(tt.column, tt.whitelist)
			if gotOK != tt.wantOK {
				t.Errorf("ValidateOrderColumn() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}
