package api

import (
	"bytes"
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatBytes(t *testing.T) {
	tmpl := template.Must(template.New("test").Funcs(templateFuncs).Parse(`{{formatBytes .}}`))

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"4.2 GB", float64(4509715660), "4.2 GB"},
		{"1.0 GB", float64(1 << 30), "1.0 GB"},
		{"512.0 MB", float64(512 * 1024 * 1024), "512.0 MB"},
		{"1.5 MB", float64(1572864), "1.5 MB"},
		{"10.0 KB", float64(10240), "10.0 KB"},
		{"500 B", float64(500), "500 B"},
		{"zero", float64(0), "0 B"},
		{"int type", int(1048576), "1.0 MB"},
		{"int64 type", int64(1073741824), "1.0 GB"},
		{"string fallback", "not a number", "—"},
		{"nil fallback", nil, "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tmpl.Execute(&buf, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
