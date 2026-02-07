# F-08: Internationalization & Accessibility

**Priority**: Low  
**Effort**: 1-2 days  
**Score Impact**: +0.03 points  
**Status**: Post-launch

---

## Gap Analysis

### Current State
- Web dashboard (S5-01)
- Accessibility mentioned but not fully defined
- English-only interface

### Missing
- i18n framework for web dashboard
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

**Framework**: react-i18next

**Setup**:
```javascript
// web/src/i18n.js
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import Backend from 'i18next-http-backend';
import LanguageDetector from 'i18next-browser-languagedetector';

i18n
  .use(Backend)
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    fallbackLng: 'en',
    supportedLngs: ['en', 'es', 'fr', 'de', 'ja', 'zh'],
    backend: {
      loadPath: '/locales/{{lng}}/{{ns}}.json'
    },
    interpolation: {
      escapeValue: false
    }
  });

export default i18n;
```

**Translation Files**:
```json
// web/public/locales/en/common.json
{
  "nav": {
    "dashboard": "Dashboard",
    "devices": "Devices",
    "policies": "Policies",
    "users": "Users",
    "settings": "Settings"
  },
  "devices": {
    "title": "Devices",
    "enrolled": "Enrolled",
    "compliant": "Compliant",
    "actions": {
      "lock": "Lock Device",
      "wipe": "Wipe Device",
      "unenroll": "Unenroll"
    }
  },
  "common": {
    "save": "Save",
    "cancel": "Cancel",
    "delete": "Delete",
    "confirm": "Confirm"
  }
}
```

```json
// web/public/locales/es/common.json
{
  "nav": {
    "dashboard": "Panel",
    "devices": "Dispositivos",
    "policies": "Políticas",
    "users": "Usuarios",
    "settings": "Configuración"
  },
  "devices": {
    "title": "Dispositivos",
    "enrolled": "Inscrito",
    "compliant": "Conforme",
    "actions": {
      "lock": "Bloquear Dispositivo",
      "wipe": "Borrar Dispositivo",
      "unenroll": "Dar de Baja"
    }
  },
  "common": {
    "save": "Guardar",
    "cancel": "Cancelar",
    "delete": "Eliminar",
    "confirm": "Confirmar"
  }
}
```

**Usage in Components**:
```javascript
// web/src/components/DeviceList.jsx
import { useTranslation } from 'react-i18next';

function DeviceList() {
  const { t } = useTranslation();
  
  return (
    <div>
      <h1>{t('devices.title')}</h1>
      <button>{t('devices.actions.lock')}</button>
    </div>
  );
}
```

**Language Selector**:
```javascript
// web/src/components/LanguageSelector.jsx
import { useTranslation } from 'react-i18next';

function LanguageSelector() {
  const { i18n } = useTranslation();
  
  const languages = [
    { code: 'en', name: 'English' },
    { code: 'es', name: 'Español' },
    { code: 'fr', name: 'Français' },
    { code: 'de', name: 'Deutsch' },
    { code: 'ja', name: '日本語' },
    { code: 'zh', name: '中文' }
  ];
  
  return (
    <select 
      value={i18n.language} 
      onChange={(e) => i18n.changeLanguage(e.target.value)}
    >
      {languages.map(lang => (
        <option key={lang.code} value={lang.code}>
          {lang.name}
        </option>
      ))}
    </select>
  );
}
```

**Date/Time Formatting**:
```javascript
// Use Intl API for locale-aware formatting
const date = new Date();
const formatter = new Intl.DateTimeFormat(i18n.language, {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit'
});

console.log(formatter.format(date));
// en: "February 6, 2026, 10:30 AM"
// es: "6 de febrero de 2026, 10:30"
// fr: "6 février 2026, 10:30"
```

**Number Formatting**:
```javascript
const number = 1234567.89;
const formatter = new Intl.NumberFormat(i18n.language, {
  style: 'currency',
  currency: 'USD'
});

console.log(formatter.format(number));
// en: "$1,234,567.89"
// es: "1.234.567,89 US$"
// fr: "1 234 567,89 $US"
```

### 2. Right-to-Left (RTL) Support

**Languages**: Arabic, Hebrew

**CSS**:
```css
/* web/src/styles/rtl.css */
[dir="rtl"] {
  direction: rtl;
  text-align: right;
}

[dir="rtl"] .sidebar {
  left: auto;
  right: 0;
}

[dir="rtl"] .icon {
  transform: scaleX(-1);
}
```

**React**:
```javascript
// web/src/App.jsx
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

function App() {
  const { i18n } = useTranslation();
  
  useEffect(() => {
    const rtlLanguages = ['ar', 'he'];
    const dir = rtlLanguages.includes(i18n.language) ? 'rtl' : 'ltr';
    document.documentElement.setAttribute('dir', dir);
  }, [i18n.language]);
  
  return <div>...</div>;
}
```

### 3. WCAG 2.1 AA Compliance

**Requirements**:

**1.1 Text Alternatives**:
- All images have alt text
- Icons have aria-labels
- Decorative images have empty alt

```jsx
<img src="device.png" alt="Windows laptop" />
<button aria-label="Lock device">
  <LockIcon aria-hidden="true" />
</button>
```

**1.3 Adaptable**:
- Semantic HTML (header, nav, main, footer)
- Proper heading hierarchy (h1 → h2 → h3)
- Form labels associated with inputs

```jsx
<label htmlFor="device-name">Device Name</label>
<input id="device-name" type="text" />
```

**1.4 Distinguishable**:
- Color contrast ratio ≥ 4.5:1 for normal text
- Color contrast ratio ≥ 3:1 for large text
- Don't rely on color alone to convey information

```css
/* Good contrast */
.button {
  background: #0066cc;  /* Blue */
  color: #ffffff;       /* White */
  /* Contrast ratio: 7.5:1 ✓ */
}

/* Bad contrast */
.button-bad {
  background: #ffcc00;  /* Yellow */
  color: #ffffff;       /* White */
  /* Contrast ratio: 1.4:1 ✗ */
}
```

**2.1 Keyboard Accessible**:
- All functionality available via keyboard
- Visible focus indicators
- Logical tab order

```css
button:focus,
input:focus {
  outline: 2px solid #0066cc;
  outline-offset: 2px;
}
```

**2.4 Navigable**:
- Skip to main content link
- Page titles describe content
- Link text is descriptive

```jsx
<a href="#main-content" className="skip-link">
  Skip to main content
</a>

<main id="main-content">
  {/* Content */}
</main>
```

**3.1 Readable**:
- Language of page specified
- Language of parts specified

```html
<html lang="en">
  <p lang="es">Hola mundo</p>
</html>
```

**4.1 Compatible**:
- Valid HTML
- ARIA attributes used correctly
- Status messages announced

```jsx
<div role="alert" aria-live="polite">
  Device locked successfully
</div>
```

### 4. Screen Reader Testing

**Tools**:
- NVDA (Windows, free)
- JAWS (Windows, commercial)
- VoiceOver (macOS, built-in)
- TalkBack (Android, built-in)

**Testing Checklist**:
- [ ] All interactive elements announced
- [ ] Form labels read correctly
- [ ] Error messages announced
- [ ] Loading states announced
- [ ] Modal dialogs trap focus
- [ ] Tables have proper headers
- [ ] Lists use proper markup

**ARIA Live Regions**:
```jsx
// Announce status changes
<div role="status" aria-live="polite" aria-atomic="true">
  {statusMessage}
</div>

// Announce errors
<div role="alert" aria-live="assertive">
  {errorMessage}
</div>
```

### 5. Keyboard Navigation

**Requirements**:
- Tab through all interactive elements
- Shift+Tab to go backwards
- Enter/Space to activate buttons
- Arrow keys for menus and lists
- Escape to close modals

**Implementation**:
```jsx
// Modal with keyboard trap
function Modal({ isOpen, onClose, children }) {
  const modalRef = useRef();
  
  useEffect(() => {
    if (!isOpen) return;
    
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        onClose();
      }
      
      // Trap focus inside modal
      if (e.key === 'Tab') {
        const focusableElements = modalRef.current.querySelectorAll(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        const firstElement = focusableElements[0];
        const lastElement = focusableElements[focusableElements.length - 1];
        
        if (e.shiftKey && document.activeElement === firstElement) {
          e.preventDefault();
          lastElement.focus();
        } else if (!e.shiftKey && document.activeElement === lastElement) {
          e.preventDefault();
          firstElement.focus();
        }
      }
    };
    
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);
  
  if (!isOpen) return null;
  
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div 
        ref={modalRef}
        className="modal" 
        role="dialog" 
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
      >
        {children}
      </div>
    </div>
  );
}
```

### 6. Automated Testing

**Tools**:
- axe-core (accessibility testing)
- Pa11y (automated testing)
- Lighthouse (Chrome DevTools)

**Integration**:
```javascript
// web/src/tests/accessibility.test.js
import { render } from '@testing-library/react';
import { axe, toHaveNoViolations } from 'jest-axe';
import DeviceList from '../components/DeviceList';

expect.extend(toHaveNoViolations);

test('DeviceList has no accessibility violations', async () => {
  const { container } = render(<DeviceList />);
  const results = await axe(container);
  expect(results).toHaveNoViolations();
});
```

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
      - uses: actions/setup-node@v3
      - run: npm install
      - run: npm run build
      - run: npm run test:a11y
      - name: Run Pa11y
        run: |
          npm install -g pa11y-ci
          pa11y-ci --sitemap http://localhost:3000/sitemap.xml
```

---

## Implementation Tasks

### Task 1: i18n Framework (0.5 days)
- Install react-i18next
- Set up translation files
- Extract all strings to translation files
- Add language selector

### Task 2: Translations (1 day)
- Translate to Spanish
- Translate to French
- Translate to German
- Test all translations

### Task 3: WCAG Compliance (0.5 days)
- Audit current accessibility
- Fix color contrast issues
- Add ARIA labels
- Improve keyboard navigation
- Test with screen readers

### Task 4: Automated Testing (0.5 days)
- Set up axe-core
- Add accessibility tests
- Configure CI/CD
- Fix violations

---

## Acceptance Criteria

- [ ] i18n framework integrated
- [ ] At least 3 languages supported (English, Spanish, French)
- [ ] RTL support for Arabic/Hebrew
- [ ] WCAG 2.1 AA compliant (automated tests pass)
- [ ] Screen reader tested (NVDA, VoiceOver)
- [ ] Keyboard navigation works for all features
- [ ] Color contrast meets requirements
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

## Translation Management

**Options**:
- **Crowdin** - Translation management platform
- **Lokalise** - Localization platform
- **POEditor** - Translation management
- **Manual** - JSON files in git

**Workflow**:
1. Developer adds new strings in English
2. Strings exported to translation platform
3. Translators translate strings
4. Translations imported back to codebase
5. Automated tests verify translations

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
- [react-i18next Documentation](https://react.i18next.com/)
- [axe-core Documentation](https://github.com/dequelabs/axe-core)
- [S5-01: Web Dashboard](../sprint-5-ui-and-polish/S5-01-web-dashboard.md)
