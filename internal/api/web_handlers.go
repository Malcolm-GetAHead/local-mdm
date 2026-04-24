package api

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/models"
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
	statusCounts := map[string]int{}
	for _, d := range devices {
		platformCounts[d.Platform]++
		statusCounts[d.Status]++
		if d.Status == "enrolled" {
			enrolled++
		}
	}

	var platList []map[string]interface{}
	for _, p := range []string{"android", "macos", "windows"} {
		if c, ok := platformCounts[p]; ok {
			platList = append(platList, map[string]interface{}{"Platform": p, "Count": c})
		}
	}

	policies, _, _ := s.policyRepo.List(ctx, sess.EnterpriseID, 1000, 0)
	activePolicies := 0
	for _, p := range policies {
		if p.IsActive && !p.IsTemplate {
			activePolicies++
		}
	}

	// Compliance counts
	compliant, nonCompliant, unknown := 0, 0, 0
	if compRows, err := s.reportService.ComplianceReport(ctx, sess.EnterpriseID); err == nil {
		for _, cr := range compRows {
			switch cr.Status {
			case "compliant":
				compliant++
			case "non_compliant":
				nonCompliant++
			default:
				unknown++
			}
		}
	}

	// Generate SVG charts
	platformChart := svgPieChart("Platforms", []pieSlice{
		{"macOS", platformCounts["macos"], "#2563eb"},
		{"Windows", platformCounts["windows"], "#7c3aed"},
		{"Android", platformCounts["android"], "#059669"},
	})
	statusChart := svgPieChart("Device Status", []pieSlice{
		{"Enrolled", statusCounts["enrolled"], "#16a34a"},
		{"Unenrolled", statusCounts["unenrolled"], "#6b7280"},
		{"Wiped", statusCounts["wiped"], "#dc2626"},
	})
	complianceChart := svgPieChart("Compliance", []pieSlice{
		{"Compliant", compliant, "#16a34a"},
		{"Non-Compliant", nonCompliant, "#dc2626"},
		{"Unknown", unknown, "#ca8a04"},
	})

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
		"Charts": map[string]template.HTML{
			"Platform":   template.HTML(platformChart),
			"Status":     template.HTML(statusChart),
			"Compliance": template.HTML(complianceChart),
		},
		"RecentAudit": auditLogs,
	})
}

// handleWebDeviceList shows the device list with filtering, sorting, and pagination.
func (s *Server) handleWebDeviceList(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	platform := r.URL.Query().Get("platform")
	status := r.URL.Query().Get("status")
	query := strings.ToLower(r.URL.Query().Get("q"))
	sortField := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	if sortField == "" {
		sortField = "name"
	}
	if sortDir == "" {
		sortDir = "asc"
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	allDevices, _, _ := s.deviceRepo.List(ctx, sess.EnterpriseID, 1000, 0)

	// Filter
	var filtered []*models.Device
	for _, d := range allDevices {
		if platform != "" && d.Platform != platform {
			continue
		}
		if status != "" && d.Status != status {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(d.Name), query) &&
			!strings.Contains(strings.ToLower(d.Model), query) &&
			!strings.Contains(strings.ToLower(d.SerialNumber), query) {
			continue
		}
		filtered = append(filtered, d)
	}

	// Sort
	sortDevices(filtered, sortField, sortDir)

	// Paginate
	total := len(filtered)
	totalPages := (total + perPage - 1) / perPage
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	data := map[string]interface{}{
		"ActiveNav":   "devices",
		"Devices":     filtered[start:end],
		"TotalPages":  totalPages,
		"CurrentPage": page,
		"TotalItems":  total,
		"Filter": map[string]string{
			"Platform": platform, "Status": status, "Query": r.URL.Query().Get("q"),
			"Sort": sortField, "Dir": sortDir,
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

	// Compliance
	compResults, _ := s.complianceService.GetDeviceCompliance(ctx, id)
	var compliance []map[string]interface{}
	for _, cr := range compResults {
		policyName := ""
		if p, err := s.policyRepo.GetByID(ctx, cr.PolicyID); err == nil {
			policyName = p.Name
		}
		compliance = append(compliance, map[string]interface{}{
			"PolicyName": policyName, "Status": cr.Status, "EvaluatedAt": cr.EvaluatedAt,
		})
	}

	// Groups
	groups, _ := s.groupService.GetDeviceGroups(ctx, id)

	// Commands
	commands, _, _ := s.cmdRepo.ListByDevice(ctx, id, 20, 0)

	// Assigned policies (via group memberships + direct assignments)
	var assignedPolicies []*models.Policy
	if groupList, err := s.groupService.GetDeviceGroups(ctx, id); err == nil {
		seen := map[uuid.UUID]bool{}
		for _, g := range groupList {
			assignments, _ := s.groupService.ListAssignments(ctx, "group", g.ID)
			for _, a := range assignments {
				if !seen[a.PolicyID] {
					if p, err := s.policyRepo.GetByID(ctx, a.PolicyID); err == nil {
						assignedPolicies = append(assignedPolicies, p)
						seen[a.PolicyID] = true
					}
				}
			}
		}
	}

	s.renderPage(w, r, "device_detail", map[string]interface{}{
		"ActiveNav":        "devices",
		"Device":           device,
		"Compliance":       compliance,
		"Groups":           groups,
		"Commands":         commands,
		"AssignedPolicies": assignedPolicies,
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
	} else {
		s.logAudit(r, "device.lock", "device", id, nil)
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
	} else {
		s.logAudit(r, "device.wipe", "device", id, nil)
	}

	s.renderFragment(w, s.webTemplates["device_detail"], "action_result", map[string]interface{}{
		"Success": success,
		"Message": msg,
	})
}

// handleWebDeviceUnenroll unenrolls a device via HTMX.
func (s *Server) handleWebDeviceUnenroll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	_, err = s.deviceService.Unenroll(ctx, id)
	msg := "Device unenrolled successfully"
	success := true
	if err != nil {
		msg = "Failed to unenroll device: " + err.Error()
		success = false
	} else {
		s.logAudit(r, "device.unenroll", "device", id, nil)
	}

	s.renderFragment(w, s.webTemplates["device_detail"], "action_result", map[string]interface{}{
		"Success": success,
		"Message": msg,
	})
}

// (end of file)

func sortDevices(devices []*models.Device, field, dir string) {
	sort.Slice(devices, func(i, j int) bool {
		var less bool
		switch field {
		case "name":
			less = strings.ToLower(devices[i].Name) < strings.ToLower(devices[j].Name)
		case "platform":
			less = devices[i].Platform < devices[j].Platform
		case "model":
			less = strings.ToLower(devices[i].Model) < strings.ToLower(devices[j].Model)
		case "os_version":
			less = devices[i].OSVersion < devices[j].OSVersion
		case "status":
			less = devices[i].Status < devices[j].Status
		case "last_seen":
			ti, tj := devices[i].LastSeen, devices[j].LastSeen
			if ti == nil && tj == nil {
				return false
			}
			if ti == nil {
				return true
			}
			if tj == nil {
				return false
			}
			less = ti.Before(*tj)
		default:
			less = strings.ToLower(devices[i].Name) < strings.ToLower(devices[j].Name)
		}
		if dir == "desc" {
			return !less
		}
		return less
	})
}
