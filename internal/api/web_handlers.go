package api

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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
	if csrf, ok := r.Context().Value(csrfTokenKey).(string); ok {
		data["CSRFToken"] = csrf
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

	// Compliance counts (single query, reused for chart + needs-attention)
	compRows, _ := s.reportService.ComplianceReport(ctx, sess.EnterpriseID)
	compliant, nonCompliant, unknown := 0, 0, 0
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

	// Generate SVG charts
	platformChart := buildChart("Platforms", []pieSlice{
		{"macOS", platformCounts["macos"], "#2563eb"},
		{"Windows", platformCounts["windows"], "#7c3aed"},
		{"Android", platformCounts["android"], "#059669"},
	})
	statusChart := buildChart("Device Status", []pieSlice{
		{"Enrolled", statusCounts["enrolled"], "#16a34a"},
		{"Unenrolled", statusCounts["unenrolled"], "#6b7280"},
		{"Wiped", statusCounts["wiped"], "#dc2626"},
	})
	complianceChart := buildChart("Compliance", []pieSlice{
		{"Compliant", compliant, "#16a34a"},
		{"Non-Compliant", nonCompliant, "#dc2626"},
		{"Unknown", unknown, "#ca8a04"},
	})

	auditLogs, _, _ := s.auditLogRepo.Search(ctx, sess.EnterpriseID, "", "", "", 5, 0)

	// Devices needing attention: non-compliant or not seen in 7 days
	type attentionItem struct {
		ID     string
		Name   string
		Reason string
	}
	var needsAttention []attentionItem
	seen := map[string]bool{}
	for _, cr := range compRows {
		if cr.Status == "non_compliant" && !seen[cr.DeviceID.String()] {
			needsAttention = append(needsAttention, attentionItem{cr.DeviceID.String(), cr.DeviceName, "non-compliant"})
			seen[cr.DeviceID.String()] = true
		}
	}
	for _, d := range devices {
		if d.LastSeen != nil && time.Since(*d.LastSeen) > 7*24*time.Hour && !seen[d.ID.String()] {
			needsAttention = append(needsAttention, attentionItem{d.ID.String(), d.Name, "not seen 7d+"})
			seen[d.ID.String()] = true
		}
	}
	if len(needsAttention) > 10 {
		needsAttention = needsAttention[:10]
	}

	s.renderPage(w, r, "dashboard", map[string]interface{}{
		"ActiveNav": "dashboard",
		"Stats": map[string]interface{}{
			"TotalDevices":   total,
			"Enrolled":       enrolled,
			"NonCompliant":   nonCompliant,
			"ActivePolicies": activePolicies,
			"PlatformCounts": platList,
		},
		"Charts": []chartData{platformChart, statusChart, complianceChart},
		"RecentAudit":    auditLogs,
		"NeedsAttention": needsAttention,
	})
}

// handleWebDeviceList shows the device list with filtering, sorting, and pagination.
func (s *Server) handleWebDeviceList(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	platform := r.URL.Query().Get("platform")
	status := r.URL.Query().Get("status")
	query := r.URL.Query().Get("q")
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

	devices, total, _ := s.deviceRepo.ListFiltered(ctx, sess.EnterpriseID, platform, status, query, sortField, sortDir, perPage, (page-1)*perPage)

	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"ActiveNav":   "devices",
		"Devices":     devices,
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
	compliance := buildComplianceRows(ctx, compResults, s)
	var lastEvaluated interface{}
	if len(compResults) > 0 {
		lastEvaluated = compResults[0].EvaluatedAt
	}

	// Groups
	groups, _ := s.groupService.GetDeviceGroups(ctx, id)

	// Commands
	commands, _, _ := s.cmdRepo.ListByDevice(ctx, id, 20, 0)

	// Assigned policies — all effective (direct + group + enterprise)
	effectiveAssignments, _ := s.groupService.GetEffectivePolicies(ctx, id, device.EnterpriseID)
	var assignedPolicies []map[string]interface{}
	for _, a := range effectiveAssignments {
		if p, err := s.policyRepo.GetByID(ctx, a.PolicyID); err == nil {
			assignedPolicies = append(assignedPolicies, map[string]interface{}{
				"ID": p.ID, "Name": p.Name, "Platform": p.Platform, "Via": a.TargetType,
			})
		}
	}

	s.renderPage(w, r, "device_detail", map[string]interface{}{
		"ActiveNav":        "devices",
		"Device":           device,
		"Compliance":       compliance,
		"LastEvaluated":    lastEvaluated,
		"Groups":           groups,
		"Commands":         commands,
		"AssignedPolicies": assignedPolicies,
		"PlatformDetails":  buildPlatformDetails(device.PlatformData),
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

// handleWebDeviceEvaluate triggers compliance evaluation for a device.
// handleWebDeviceDelete deletes a device record.
func (s *Server) handleWebDeviceDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := uuid.Parse(mux.Vars(r)["id"])
	s.deviceService.Delete(ctx, id)
	s.logAudit(r, "device.delete", "device", id, nil)
	w.Header().Set("HX-Redirect", "/dashboard/devices")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWebDeviceEvaluate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := uuid.Parse(mux.Vars(r)["id"])

	err := s.complianceService.EvaluateDeviceByID(ctx, id)
	if err != nil {
		s.logger.Error("compliance evaluation failed", "error", err, "device_id", id)
	}

	// Return updated compliance table
	compResults, _ := s.complianceService.GetDeviceCompliance(ctx, id)
	compliance := buildComplianceRows(ctx, compResults, s)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.webTemplates["device_detail"].ExecuteTemplate(w, "compliance_tab", map[string]interface{}{"Compliance": compliance})
}

func buildComplianceRows(ctx context.Context, results []*models.ComplianceResult, s *Server) []map[string]interface{} {
	var rows []map[string]interface{}
	for _, cr := range results {
		policyName := ""
		var configKeys []string
		if p, err := s.policyRepo.GetByID(ctx, cr.PolicyID); err == nil {
			policyName = p.Name
			for k := range p.PolicyConfig {
				configKeys = append(configKeys, k)
			}
		}

		// Build violation set for quick lookup
		violationSet := map[string]bool{}
		if v, ok := cr.Details["violations"]; ok {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if str, ok := item.(string); ok {
						violationSet[str] = true
					}
				}
			}
		}

		if cr.Status == "unknown" {
			// Single row for unknown
			rows = append(rows, map[string]interface{}{
				"Setting": "all settings", "PolicyName": policyName,
				"Status": "unknown", "EvaluatedAt": cr.EvaluatedAt,
			})
			continue
		}

		if len(configKeys) == 0 {
			rows = append(rows, map[string]interface{}{
				"Setting": "all settings", "PolicyName": policyName,
				"Status": cr.Status, "EvaluatedAt": cr.EvaluatedAt,
			})
			continue
		}

		// One row per config key
		sort.Strings(configKeys)
		for _, k := range configKeys {
			status := "pass"
			// Check if this key produced a violation
			for viol := range violationSet {
				if violationMatchesKey(viol, k) {
					status = "fail"
					break
				}
			}
			label := strings.ReplaceAll(k, "_", " ")
			// Look up friendly label from catalog
			for _, cs := range policySettingsCatalog {
				if cs.Key == k {
					label = cs.Label
					break
				}
			}
			rows = append(rows, map[string]interface{}{
				"Setting": label, "PolicyName": policyName,
				"Status": status, "EvaluatedAt": cr.EvaluatedAt,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		pi := rows[i]["PolicyName"].(string)
		pj := rows[j]["PolicyName"].(string)
		if pi != pj {
			return pi < pj
		}
		return rows[i]["Setting"].(string) < rows[j]["Setting"].(string)
	})
	return rows
}

// violationMatchesKey maps a violation string to the config key that produced it.
// Uses keyword matching: the violation must contain a keyword derived from the config key.
var violationKeywords = map[string][]string{
	"require_password":    {"password"},
	"min_password_length": {"password length"},
	"require_encryption":  {"encryption"},
	"require_firewall":    {"firewall"},
	"allow_camera":        {"camera"},
}

func violationMatchesKey(violation, configKey string) bool {
	vLower := strings.ToLower(violation)
	if keywords, ok := violationKeywords[configKey]; ok {
		for _, kw := range keywords {
			if strings.Contains(vLower, kw) {
				return true
			}
		}
		return false
	}
	// Fallback: check if violation contains the key with underscores replaced
	return strings.Contains(vLower, strings.ReplaceAll(configKey, "_", " "))
}

type platformDetailItem struct {
	Label   string
	Value   string
	Type    string // "text" or "bool"
	BoolVal bool
}

type platformDetailGroup struct {
	Category string
	Items    []platformDetailItem
}

var platformKeyCategories = map[string]string{
	"serial": "Hardware", "architecture": "Hardware", "storage_total_gb": "Hardware",
	"storage_free_gb": "Hardware", "product_name": "Hardware",
	"hostname": "Network", "ip_address": "Network", "mac_address": "Network",
	"FileVaultEnabled": "Security", "firewall_enabled": "Security", "password_present": "Security",
	"password_length": "Security", "bitlocker_enabled": "Security", "bitlocker_status": "Security",
	"encryption_enabled": "Security", "supervised": "Security",
	"build_version": "Operating System", "topic": "MDM", "push_magic": "MDM",
	"has_token": "MDM", "mdm_enrolled": "MDM",
}

var platformKeyLabels = map[string]string{
	"serial": "Serial Number", "architecture": "Architecture", "storage_total_gb": "Total Storage (GB)",
	"storage_free_gb": "Free Storage (GB)", "product_name": "Product Name",
	"hostname": "Hostname", "ip_address": "IP Address", "mac_address": "MAC Address",
	"FileVaultEnabled": "FileVault", "firewall_enabled": "Firewall", "password_present": "Password Set",
	"password_length": "Password Length", "bitlocker_enabled": "BitLocker", "bitlocker_status": "BitLocker Status",
	"encryption_enabled": "Encryption", "supervised": "Supervised",
	"build_version": "Build Version", "topic": "Push Topic", "push_magic": "Push Magic",
	"has_token": "Has Token", "mdm_enrolled": "MDM Enrolled",
}

func buildPlatformDetails(pd models.JSONB) []platformDetailGroup {
	if len(pd) == 0 {
		return nil
	}
	groups := map[string][]platformDetailItem{}
	for k, v := range pd {
		cat := platformKeyCategories[k]
		if cat == "" {
			cat = "Other"
		}
		label := platformKeyLabels[k]
		if label == "" {
			label = strings.ReplaceAll(k, "_", " ")
			if len(label) > 0 {
				label = strings.ToUpper(label[:1]) + label[1:]
			}
		}
		item := platformDetailItem{Label: label, Type: "text"}
		switch val := v.(type) {
		case bool:
			item.Type = "bool"
			item.BoolVal = val
		default:
			item.Value = fmt.Sprintf("%v", v)
		}
		groups[cat] = append(groups[cat], item)
	}
	order := []string{"Hardware", "Network", "Security", "Operating System", "MDM", "Other"}
	var result []platformDetailGroup
	for _, cat := range order {
		items := groups[cat]
		if len(items) == 0 {
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
		result = append(result, platformDetailGroup{Category: cat, Items: items})
	}
	return result
}

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
