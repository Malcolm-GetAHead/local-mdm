package api

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestIsHTMX(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	assert.False(t, isHTMX(r))

	r.Header.Set("HX-Request", "true")
	assert.True(t, isHTMX(r))
}

func TestDetectPolicyType(t *testing.T) {
	config := models.JSONB{"require_encryption": true, "min_password_length": 8}
	assert.Equal(t, "security", detectPolicyType(config))

	config = models.JSONB{"disable_camera": true}
	assert.Equal(t, "restrictions", detectPolicyType(config))
}

func TestParseSettingsFromForm(t *testing.T) {
	form := url.Values{
		"setting_require_encryption":  {"true"},
		"setting_min_password_length": {"8"},
		"setting_bogus_key":           {"value"},
		"name":                        {"ignored"},
	}
	r, _ := http.NewRequest("POST", "/", nil)
	r.Form = form

	config, invalid := parseSettingsFromForm(r)
	assert.Contains(t, invalid, "bogus_key")
	assert.Equal(t, true, config["require_encryption"])
	assert.Equal(t, 8, config["min_password_length"])
}

func TestPickBestRole(t *testing.T) {
	assert.Equal(t, "super_admin", pickBestRole([]string{"viewer", "super_admin", "admin"}))
	assert.Equal(t, "admin", pickBestRole([]string{"viewer", "admin"}))
	assert.Equal(t, "viewer", pickBestRole([]string{"viewer"}))
	assert.Equal(t, "viewer", pickBestRole([]string{}))
	assert.Equal(t, "viewer", pickBestRole([]string{"unknown_role"}))
}

func TestBuildChart(t *testing.T) {
	chart := buildChart("Test", []pieSlice{
		{"A", 50, "#ff0000"},
		{"B", 30, "#00ff00"},
		{"C", 0, "#0000ff"},
	})
	assert.Equal(t, "Test", chart.Title)
	assert.Contains(t, chart.Pie, "<svg")
	assert.Len(t, chart.Legend, 2) // C has 0 value, excluded
	assert.Equal(t, "A", chart.Legend[0].Label)
}

func TestBuildChartSingleSlice(t *testing.T) {
	chart := buildChart("Solo", []pieSlice{{"Only", 100, "#ff0000"}})
	assert.Contains(t, chart.Pie, "A") // full circle uses two arcs
	assert.Len(t, chart.Legend, 1)
}

func TestGenerateCSRF(t *testing.T) {
	key := []byte("test-secret-key")
	token1 := generateCSRF("user-123", key)
	token2 := generateCSRF("user-123", key)
	token3 := generateCSRF("user-456", key)

	assert.Equal(t, token1, token2, "same user should get same token")
	assert.NotEqual(t, token1, token3, "different users should get different tokens")
	assert.Len(t, token1, 32)
}

func TestSplitOnce(t *testing.T) {
	assert.Equal(t, []string{"a", "b.c"}, splitOnce("a.b.c", '.'))
	assert.Equal(t, []string{"abc"}, splitOnce("abc", '.'))
}
