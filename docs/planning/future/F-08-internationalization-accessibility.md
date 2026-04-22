# F-08: Internationalization & Accessibility

**Priority**: Low  
**Effort**: 1-2 days  
**Score Impact**: +0.03 points  
**Status**: Post-launch

---

## Gap Analysis

### Current State
- Web dashboard uses Go HTML templates + HTMX + Tailwind CSS (Sprint 5b)
- Accessibility mentioned but not fully defined
- English-only interface

### Missing
- i18n framework for Go templates
- Translations (Spanish, French, German, etc.)
- WCAG 2.1 AA compliance testing
- Screen reader testing
- Keyboard navigation testing
- Color contrast validation
- RTL language support

### Impact
Without i18n and accessibility:
- Limited to English-speaking users
- Excludes users with disabilities
- Legal compliance issues (ADA, Section 508)
- Reduced market reach

---

## Proposed Solution

### 1. Internationalization (i18n)

**Framework**: Go `text/template` with translation maps (or `nicksnyder/go-i18n` library)

**Setup**:
```go
// internal/i18n/i18n.go
package i18n

import "embed"

//go:embed locales/*.json
var localeFS embed.FS

type Localizer struct {
    translations map[string]map[string]string // lang -> key -> value
    defaultLang  string
}

func New(defaultLang string) (*Localizer, error) {
    l := &Localizer{
        translations: make(map[string]map[string]string),
        defaultLang:  defaultLang,
    }
    // Load embedded locale files
    return l, l.loadLocales()
}

func (l *Localizer) T(lang, key string) string {
    if t, ok := l.translations[lang][key]; ok {
        return t
    }
    if t, ok := l.translations[l.defaultLang][key]; ok {
        return t
    }
    return key
}
```

**Translation Files**:
```json
// internal/i18n/locales/en.json
{
  "nav.dashboard": "Dashboard",
  "nav.devices": "Devices",
  "nav.policies": "Policies",
  "nav.users": "Users",
  "devices.title": "Devices",
  "devices.enrolled": "Enrolled",
  "devices.actions.lock": "Lock Device",
  "devices.actions.wipe": "Wipe Device",
  "common.save": "Save",
  "common.cancel": "Cancel",
  "common.delete": "Delete"
}
```

```json
// internal/i18n/locales/es.json
{
  "nav.dashboard": "Panel",
  "nav.devices": "Dispositivos",
  "nav.policies": "Políticas",
  "nav.users": "Usuarios",
  "devices.title": "Dispositivos",
  "devices.enrolled": "Inscrito",
  "devices.actions.lock": "Bloquear Dispositivo",
  "devices.actions.wipe": "Borrar Dispositivo",
  "common.save": "Guardar",
  "common.cancel": "Cancelar",
  "common.delete": "Eliminar"
}
```

**Usage in Go Templates**:
```html
<!-- templates/devices/list.html -->
<h1>{{ T .Lang "devices.title" }}</h1>
<table>
  <thead>
    <tr>
      <th>{{ T .Lang "devices.name" }}</th>
      <th>{{ T .Lang "devices.status" }}</th>
    </tr>
  </thead>
</table>
<button hx-post="/api/v1/devices/{{ .Device.ID }}/lock">
  {{ T .Lang "devices.actions.lock" }}
</button>
```

**Language Selection** (via cookie or Accept-Language header):
```go
// internal/api/middleware.go
func languageMiddleware(localizer *i18n.Localizer) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            lang := r.URL.Query().Get("lang")
            if lang == "" {
                if c, err := r.Cookie("lang"); err == nil {
                    lang = c.Value
                }
            }
            if lang == "" {
                lang = parseAcceptLanguage(r.Header.Get("Accept-Language"))
            }
            ctx := context.WithValue(r.Context(), langKey, lang)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 2. Right-to-Left (RTL) Support

**Languages**: Arabic, Hebrew

**Tailwind CSS** (built-in RTL support):
```html
<html lang="ar" dir="rtl">
  <!-- Tailwind's rtl: variant handles layout flipping -->
  <div class="ml-4 rtl:mr-4 rtl:ml-0">Content</div>
</html>
```

**Template Logic**:
```html
{{ if isRTL .Lang }}
<html lang="{{ .Lang }}" dir="rtl">
{{ else }}
<html lang="{{ .Lang }}" dir="ltr">
{{ end }}
```

### 3. WCAG 2.1 AA Compliance

**Requirements** (same regardless of framework):

**1.1 Text Alternatives**:
```html
<img src="/static/device.png" alt="Windows laptop">
<button aria-label="Lock device">
  <svg aria-hidden="true"><!-- lock icon --></svg>
</button>
```

**1.3 Adaptable**:
```html
<!-- Semantic HTML with proper heading hierarchy -->
<header><nav>...</nav></header>
<main>
  <h1>Devices</h1>
  <section>
    <h2>Enrolled Devices</h2>
  </section>
</main>
<footer>...</footer>
```

**1.4 Distinguishable**:
- Color contrast ratio ≥ 4.5:1 for normal text
- Color contrast ratio ≥ 3:1 for large text
- Tailwind CSS classes should be audited for contrast compliance

**2.1 Keyboard Accessible**:
```html
<!-- All interactive elements focusable, visible focus ring -->
<button class="focus:ring-2 focus:ring-blue-500 focus:outline-none">
  Lock Device
</button>
```

**2.4 Navigable**:
```html
<a href="#main-content" class="sr-only focus:not-sr-only">
  Skip to main content
</a>
<main id="main-content">...</main>
```

**4.1 Compatible — HTMX considerations**:
```html
<!-- HTMX swaps should announce changes to screen readers -->
<div id="device-list" aria-live="polite" hx-get="/devices" hx-trigger="load">
  Loading...
</div>

<!-- Status messages after actions -->
<div role="alert" aria-live="assertive" id="flash-messages">
  {{ if .Flash }}{{ .Flash }}{{ end }}
</div>
```

### 4. Automated Testing

**Tools** (no React/Jest — use CLI tools against rendered HTML):
- **Pa11y** — automated WCAG testing against URLs
- **axe CLI** — accessibility testing from command line
- **Lighthouse CI** — Chrome-based auditing

**CI/CD**:
```yaml
# .github/workflows/accessibility.yml
name: Accessibility Tests
on: [push, pull_request]
jobs:
  a11y:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: make build && make run &
      - run: sleep 5
      - run: npx pa11y-ci --sitemap http://localhost:8080/sitemap.xml
      - run: npx @axe-core/cli http://localhost:8080/dashboard
```

---

## Implementation Tasks

### Task 1: i18n Framework (0.5 days)
- Implement Go `Localizer` with embedded locale files
- Add `T` template function for translations
- Add language detection middleware (cookie, Accept-Language)
- Extract all template strings to locale JSON files

### Task 2: Translations (1 day)
- Translate to Spanish, French, German
- Test all translations render correctly
- Add language selector to dashboard header

### Task 3: WCAG Compliance (0.5 days)
- Audit templates for semantic HTML, ARIA labels, contrast
- Add skip-to-content link
- Ensure HTMX swaps use `aria-live` regions
- Test with VoiceOver (macOS) and NVDA (Windows)

### Task 4: Automated Testing (0.5 days)
- Set up Pa11y CI against running server
- Fix violations
- Add to CI pipeline

---

## Acceptance Criteria

- [ ] i18n framework integrated with Go templates
- [ ] At least 3 languages supported (English, Spanish, French)
- [ ] RTL support for Arabic/Hebrew (Tailwind `rtl:` variant)
- [ ] WCAG 2.1 AA compliant (Pa11y tests pass)
- [ ] Screen reader tested (VoiceOver, NVDA)
- [ ] Keyboard navigation works for all features
- [ ] HTMX dynamic content announced to screen readers
- [ ] Automated accessibility tests in CI/CD

---

## Supported Languages (Priority Order)

1. **English** (en) - Default
2. **Spanish** (es) - 460M speakers
3. **French** (fr) - 280M speakers
4. **German** (de) - 130M speakers
5. **Japanese** (ja) - 125M speakers
6. **Chinese Simplified** (zh) - 1.1B speakers
7. **Portuguese** (pt) - 220M speakers
8. **Arabic** (ar) - 310M speakers (RTL)
9. **Russian** (ru) - 260M speakers
10. **Italian** (it) - 85M speakers

---

## Future Enhancements

- Voice control support
- High contrast mode
- Dyslexia-friendly fonts
- Text-to-speech for notifications
- Customizable UI (font size, spacing)
- Translation memory for consistency

---

## References

- [WCAG 2.1 Guidelines](https://www.w3.org/WAI/WCAG21/quickref/)
- [Pa11y Documentation](https://pa11y.org/)
- [Tailwind CSS RTL](https://tailwindcss.com/blog/tailwindcss-v3-3#rtl-and-ltr-modifiers)
- [HTMX Accessibility](https://htmx.org/essays/a11y/)
- [Sprint 5c: Web Dashboard](../sprints/sprint-5c-web-dashboard/OVERVIEW.md)
