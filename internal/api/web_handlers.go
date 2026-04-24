package api

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// renderPage renders a named template with the given data.
func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	tmpl, ok := s.webTemplates[name]
	if !ok {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}
	sess := getSession(r)
	if data == nil {
		data = make(map[string]interface{})
	}
	data["User"] = sessionToUser(sess)
	if sess != nil {
		data["UserRole"] = sess.Role
	}
	if nonce, ok := r.Context().Value(cspNonceKey).(string); ok {
		data["CSPNonce"] = nonce
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		s.logger.Error("Template render error", "template", name, "error", err)
	}
}

// renderFragment renders only a named sub-template (for HTMX partial responses).
func (s *Server) renderFragment(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("Fragment render error", "template", name, "error", err)
	}
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// handleDashboardHome shows the main dashboard with stats.
func (s *Server) handleDashboardHome(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	devices, total, _ := s.deviceRepo.List(ctx, sess.EnterpriseID, 1000, 0)

	enrolled := 0
	platformCounts := map[string]int{}
	for _, d := range devices {
		platformCounts[d.Platform]++
		if d.Status == "enrolled" {
			enrolled++
		}
	}

	var platList []map[string]interface{}
	for p, c := range platformCounts {
		platList = append(platList, map[string]interface{}{"Platform": p, "Count": c})
	}

	policies, _, _ := s.policyRepo.List(ctx, sess.EnterpriseID, 1000, 0)
	activePolicies := 0
	for _, p := range policies {
		if p.IsActive && !p.IsTemplate {
			activePolicies++
		}
	}

	// Compliance non-compliant count from report service
	nonCompliant := 0
	if compRows, err := s.reportService.ComplianceReport(ctx, sess.EnterpriseID); err == nil {
		for _, cr := range compRows {
			if cr.Status == "non_compliant" {
				nonCompliant++
			}
		}
	}

	auditLogs, _, _ := s.auditLogRepo.Search(ctx, sess.EnterpriseID, "", "", "", 5, 0)

	s.renderPage(w, r, "dashboard", map[string]interface{}{
		"ActiveNav": "dashboard",
		"Stats": map[string]interface{}{
			"TotalDevices":   total,
			"Enrolled":       enrolled,
			"NonCompliant":   nonCompliant,
			"ActivePolicies": activePolicies,
			"PlatformCounts": platList,
		},
		"RecentAudit": auditLogs,
	})
}

// handleWebDeviceList shows the device list with filtering and pagination.
func (s *Server) handleWebDeviceList(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	platform := r.URL.Query().Get("platform")
	status := r.URL.Query().Get("status")
	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 20
	offset := (page - 1) * perPage

	devices, total, _ := s.deviceRepo.List(ctx, sess.EnterpriseID, perPage, offset)

	// Client-side filter (the repo List doesn't support platform/status/search filters directly)
	// For a real implementation, add filter params to the repo. For now, fetch all and filter.
	if platform != "" || status != "" || query != "" {
		allDevices, allTotal, _ := s.deviceRepo.List(ctx, sess.EnterpriseID, 1000, 0)
		var filtered []interface{}
		for _, d := range allDevices {
			if platform != "" && d.Platform != platform {
				continue
			}
			if status != "" && d.Status != status {
				continue
			}
			if query != "" && !containsCI(d.Name, query) && !containsCI(d.Model, query) && !containsCI(d.SerialNumber, query) {
				continue
			}
			filtered = append(filtered, d)
		}
		_ = allTotal
		total = len(filtered)
		start := offset
		end := offset + perPage
		if start > len(filtered) {
			start = len(filtered)
		}
		if end > len(filtered) {
			end = len(filtered)
		}
		devices = nil // clear typed slice
		// Re-assign from filtered
		for _, f := range filtered[start:end] {
			if d, ok := f.(*interface{}); ok {
				_ = d
			}
		}
		// Simpler: just use allDevices filtered
		devices = nil
		for i, d := range allDevices {
			if platform != "" && d.Platform != platform {
				continue
			}
			if status != "" && d.Status != status {
				continue
			}
			if query != "" && !containsCI(d.Name, query) && !containsCI(d.Model, query) && !containsCI(d.SerialNumber, query) {
				continue
			}
			if i >= offset && len(devices) < perPage {
				devices = append(devices, d)
			}
		}
	}

	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"ActiveNav":   "devices",
		"Devices":     devices,
		"TotalPages":  totalPages,
		"CurrentPage": page,
		"Filter": map[string]string{
			"Platform": platform,
			"Status":   status,
			"Query":    query,
		},
	}

	if isHTMX(r) {
		s.renderFragment(w, s.webTemplates["devices"], "device_table_body", data)
		return
	}
	s.renderPage(w, r, "devices", data)
}

// handleWebDeviceDetail shows a single device's details.
func (s *Server) handleWebDeviceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	// Get compliance results for this device
	compResults, _ := s.complianceService.GetDeviceCompliance(ctx, id)
	var compliance []map[string]interface{}
	for _, cr := range compResults {
		policyName := ""
		if p, err := s.policyRepo.GetByID(ctx, cr.PolicyID); err == nil {
			policyName = p.Name
		}
		compliance = append(compliance, map[string]interface{}{
			"PolicyName":  policyName,
			"Status":      cr.Status,
			"EvaluatedAt": cr.EvaluatedAt,
		})
	}

	// Get groups this device belongs to
	groups, _ := s.groupService.GetDeviceGroups(ctx, id)

	s.renderPage(w, r, "device_detail", map[string]interface{}{
		"ActiveNav":  "devices",
		"Device":     device,
		"Compliance": compliance,
		"Groups":     groups,
	})
}

// handleWebDeviceLock sends a lock command via HTMX.
func (s *Server) handleWebDeviceLock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	_, err = s.deviceService.Lock(ctx, id)
	msg := "Lock command sent successfully"
	success := true
	if err != nil {
		msg = "Failed to send lock command: " + err.Error()
		success = false
	}

	s.renderFragment(w, s.webTemplates["device_detail"], "action_result", map[string]interface{}{
		"Success": success,
		"Message": msg,
	})
}

// handleWebDeviceWipe sends a wipe command via HTMX.
func (s *Server) handleWebDeviceWipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	_, err = s.deviceService.Wipe(ctx, id)
	msg := "Wipe command sent successfully"
	success := true
	if err != nil {
		msg = "Failed to send wipe command: " + err.Error()
		success = false
	}

	s.renderFragment(w, s.webTemplates["device_detail"], "action_result", map[string]interface{}{
		"Success": success,
		"Message": msg,
	})
}

func containsCI(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) >= len(substr) &&
			(http.CanonicalHeaderKey(s) != "" || true) &&
			containsLower(s, substr))
}

func containsLower(s, sub string) bool {
	s = toLower(s)
	sub = toLower(sub)
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
