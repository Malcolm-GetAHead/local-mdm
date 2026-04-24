package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
	"time"

	"github.com/google/uuid"
)

//go:embed templates/*.html templates/**/*.html
var templateFS embed.FS

// templateData is the base data passed to all templates.
type templateData struct {
	User        *sessionUser
	UserRole    string
	ActiveNav   string
	Error       string
	Success     string
	// Page-specific data embedded via composition
}

type sessionUser struct {
	ID           uuid.UUID
	Email        string
	Role         string
	EnterpriseID uuid.UUID
}

// Template helper functions
var templateFuncs = template.FuncMap{
	"timeAgo": func(t interface{}) string {
		var ts time.Time
		switch v := t.(type) {
		case time.Time:
			ts = v
		case *time.Time:
			if v == nil {
				return "Never"
			}
			ts = *v
		default:
			return ""
		}
		d := time.Since(ts)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			m := int(d.Minutes())
			if m == 1 {
				return "1 minute ago"
			}
			return fmt.Sprintf("%d minutes ago", m)
		case d < 24*time.Hour:
			h := int(d.Hours())
			if h == 1 {
				return "1 hour ago"
			}
			return fmt.Sprintf("%d hours ago", h)
		default:
			days := int(d.Hours() / 24)
			if days == 1 {
				return "1 day ago"
			}
			return fmt.Sprintf("%d days ago", days)
		}
	},
	"formatDate": func(t time.Time) string {
		return t.Format("Jan 2, 2006")
	},
	"formatDateTime": func(t time.Time) string {
		return t.Format("Jan 2, 2006 3:04 PM")
	},
	"jsonPretty": func(v interface{}) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "{}"
		}
		return string(b)
	},
	"jsonCompact": func(v interface{}) string {
		b, err := json.Marshal(v)
		if err != nil {
			return "{}"
		}
		s := string(b)
		if len(s) > 80 {
			return s[:80] + "…"
		}
		return s
	},
	"join": strings.Join,
	"pages": func(total, current int) []int {
		var p []int
		for i := 1; i <= total; i++ {
			p = append(p, i)
		}
		return p
	},
	"eq": func(a, b interface{}) bool {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	},
	"ne": func(a, b interface{}) bool {
		return fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b)
	},
	"dict": func(pairs ...interface{}) map[string]interface{} {
		m := make(map[string]interface{}, len(pairs)/2)
		for i := 0; i < len(pairs)-1; i += 2 {
			m[fmt.Sprintf("%v", pairs[i])] = pairs[i+1]
		}
		return m
	},
	"printf": fmt.Sprintf,
	"safeHTML": func(s string) template.HTML { return template.HTML(s) },
}

func (s *Server) loadTemplates() error {
	base, err := fs.ReadFile(templateFS, "templates/base.html")
	if err != nil {
		return fmt.Errorf("read base template: %w", err)
	}

	// Read all partials
	partials, err := fs.Glob(templateFS, "templates/partials/*.html")
	if err != nil {
		return fmt.Errorf("glob partials: %w", err)
	}
	var partialContent string
	for _, p := range partials {
		b, err := fs.ReadFile(templateFS, p)
		if err != nil {
			return fmt.Errorf("read partial %s: %w", p, err)
		}
		partialContent += string(b) + "\n"
	}

	// Parse base + partials as the root template
	baseTmpl, err := template.New("base").Funcs(templateFuncs).Parse(string(base) + "\n" + partialContent)
	if err != nil {
		return fmt.Errorf("parse base template: %w", err)
	}

	// Load each page by cloning base and parsing the page on top
	pages, err := fs.Glob(templateFS, "templates/pages/*.html")
	if err != nil {
		return fmt.Errorf("glob pages: %w", err)
	}

	s.webTemplates = make(map[string]*template.Template)
	for _, page := range pages {
		pageContent, err := fs.ReadFile(templateFS, page)
		if err != nil {
			return fmt.Errorf("read page %s: %w", page, err)
		}

		name := strings.TrimPrefix(page, "templates/pages/")
		name = strings.TrimSuffix(name, ".html")

		clone, err := baseTmpl.Clone()
		if err != nil {
			return fmt.Errorf("clone base for %s: %w", name, err)
		}

		tmpl, err := clone.Parse(string(pageContent))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}
		s.webTemplates[name] = tmpl
	}

	s.logger.Info("Dashboard templates loaded", "count", len(s.webTemplates))
	return nil
}
