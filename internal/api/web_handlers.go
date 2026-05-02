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
	// Resolve page title
	var titleBuf strings.Builder
	tmpl.ExecuteTemplate(&titleBuf, "page_title", data)
	data["PageTitle"] = titleBuf.String()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// HTMX sidebar navigation: return header+content fragment only
	if isHTMX(r) && r.Header.Get("HX-Target") == "page-content" {
		if err := tmpl.ExecuteTemplate(w, "header", data); err != nil {
			s.logger.Error("Template render error", "template", name, "error", err)
		}
		fmt.Fprint(w, `<div class="p-4 md:p-8">`)
		if err := tmpl.ExecuteTemplate(w, "content", data); err != nil {
			s.logger.Error("Template render error", "template", name, "error", err)
		}
		fmt.Fprint(w, `</div>`)
		return
	}
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

// isHTMXFragment returns true for HTMX requests that target a specific fragment
// (e.g. table body), not full page-content navigation swaps.
func isHTMXFragment(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") != "page-content"
}

// handleDashboardHome shows the main dashboard with stats.
func (s *Server) handleDashboardHome(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	devices, total, _ := s.deviceService.List(ctx, sess.EnterpriseID, 1000, 0)

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

	policies, _, _ := s.policyService.List(ctx, sess.EnterpriseID, 1000, 0)
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

	auditLogs, _, _ := s.auditLogService.Search(ctx, sess.EnterpriseID, "", "", "", 5, 0)

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

	devices, total, _ := s.deviceService.ListFiltered(ctx, sess.EnterpriseID, platform, status, query, sortField, sortDir, perPage, (page-1)*perPage)

	// Get pending enrollment count (reuse ListFiltered to avoid new interface method)
	_, pendingCount, _ := s.deviceService.ListFiltered(ctx, sess.EnterpriseID, "", "pending", "", "name", "asc", 1, 0)

	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"ActiveNav":    "devices",
		"Devices":      devices,
		"TotalPages":   totalPages,
		"CurrentPage":  page,
		"TotalItems":   total,
		"PendingCount": pendingCount,
		"Filter": map[string]string{
			"Platform": platform, "Status": status, "Query": r.URL.Query().Get("q"),
			"Sort": sortField, "Dir": sortDir,
		},
	}

	if isHTMXFragment(r) {
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

	device, err := s.deviceService.Get(ctx, id)
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
	commands, _, _ := s.commandService.ListCommands(ctx, id, 20, 0)

	// Assigned policies — all effective (direct + group + enterprise)
	effectiveAssignments, _ := s.groupService.GetEffectivePolicies(ctx, id, device.EnterpriseID)
	var policyIDs []uuid.UUID
	for _, a := range effectiveAssignments {
		policyIDs = append(policyIDs, a.PolicyID)
	}
	policiesByID := map[uuid.UUID]*models.Policy{}
	if batchPolicies, err := s.policyService.ListByIDs(ctx, policyIDs); err == nil {
		for _, p := range batchPolicies {
			policiesByID[p.ID] = p
		}
	}
	var assignedPolicies []map[string]interface{}
	for _, a := range effectiveAssignments {
		if p, ok := policiesByID[a.PolicyID]; ok {
			assignedPolicies = append(assignedPolicies, map[string]interface{}{
				"ID": p.ID, "Name": p.Name, "Platform": p.Platform, "Via": a.TargetType,
			})
		}
	}

	// Extract installed profiles and apps from platform_data for tabs
	var installedProfiles, installedApps, certificates, availableUpdates, activeExtensions, localUsers []map[string]interface{}
	if pd := device.PlatformData; pd != nil {
		if profiles, ok := pd["installed_profiles"].([]interface{}); ok {
			for _, p := range profiles {
				if pm, ok := p.(map[string]interface{}); ok {
					installedProfiles = append(installedProfiles, pm)
				}
			}
		}
		if apps, ok := pd["installed_apps"].([]interface{}); ok {
			for _, a := range apps {
				if am, ok := a.(map[string]interface{}); ok {
					installedApps = append(installedApps, am)
				}
			}
		}
		if certs, ok := pd["certificates"].([]interface{}); ok {
			for _, c := range certs {
				if cm, ok := c.(map[string]interface{}); ok {
					certificates = append(certificates, cm)
				}
			}
		}
		if updates, ok := pd["available_os_updates"].([]interface{}); ok {
			for _, u := range updates {
				if um, ok := u.(map[string]interface{}); ok {
					availableUpdates = append(availableUpdates, um)
				}
			}
		}
		if exts, ok := pd["active_extensions"].([]interface{}); ok {
			for _, e := range exts {
				if em, ok := e.(map[string]interface{}); ok {
					activeExtensions = append(activeExtensions, em)
				}
			}
		}
		if users, ok := pd["local_users"].([]interface{}); ok {
			for _, u := range users {
				if um, ok := u.(map[string]interface{}); ok {
					localUsers = append(localUsers, um)
				}
			}
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
		"InstalledProfiles": installedProfiles,
		"InstalledApps":     installedApps,
		"Certificates":      certificates,
		"AvailableUpdates":  availableUpdates,
		"ActiveExtensions":  activeExtensions,
		"LocalUsers":        localUsers,
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

// handleWebDeviceCheckin sends an APNs push to trigger device check-in.
func (s *Server) handleWebDeviceCheckin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	device, err := s.deviceService.Get(r.Context(), id)
	if err != nil {
		s.renderFragment(w, s.webTemplates["device_detail"], "action_result", map[string]interface{}{
			"Success": false, "Message": "Device not found",
		})
		return
	}

	// Call NanoMDM push API to send APNs notification
	msg := "Check-in push sent — device will connect shortly"
	success := true

	pushURL := s.config.MacOS.NanoMDMURL + "/v1/push/" + device.DeviceID
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, pushURL, nil)
	req.SetBasicAuth("nanomdm", s.config.MacOS.NanoMDMAPIKey)
	pushClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := pushClient.Do(req)
	if err != nil {
		msg = "Failed to contact NanoMDM: " + err.Error()
		success = false
	} else {
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			msg = "Push failed (APNs not configured?) — device will check in on next reboot"
			success = false
		}
	}

	s.logAudit(r, "device.checkin_push", "device", id, nil)

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
	w.Header().Set("HX-Trigger", `{"showToast":{"message":"Device deleted","type":"success"}}`)
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
	// Batch-fetch all policies referenced by compliance results
	policyIDSet := map[uuid.UUID]bool{}
	for _, cr := range results {
		policyIDSet[cr.PolicyID] = true
	}
	var policyIDs []uuid.UUID
	for id := range policyIDSet {
		policyIDs = append(policyIDs, id)
	}
	policiesByID := map[uuid.UUID]*models.Policy{}
	if batchPolicies, err := s.policyService.ListByIDs(ctx, policyIDs); err == nil {
		for _, p := range batchPolicies {
			policiesByID[p.ID] = p
		}
	}

	var rows []map[string]interface{}
	for _, cr := range results {
		policyName := ""
		var configKeys []string
		if p, ok := policiesByID[cr.PolicyID]; ok {
			policyName = p.Name
			for k := range p.PolicyConfig {
				configKeys = append(configKeys, k)
			}
		}

		if cr.Status == "unknown" {
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

		// Build violation key set — supports both map[string]string (new) and []string (legacy)
		violatedKeys := map[string]bool{}
		if v, ok := cr.Details["violations"]; ok {
			switch vv := v.(type) {
			case map[string]interface{}:
				for key := range vv {
					violatedKeys[key] = true
				}
			}
		}

		sort.Strings(configKeys)
		for _, k := range configKeys {
			status := "pass"
			if violatedKeys[k] {
				status = "fail"
			}
			label := strings.ReplaceAll(k, "_", " ")
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
	"storage_free_gb": "Hardware", "product_name": "Hardware", "chip": "Hardware",
	"storage_capacity_gb": "Hardware", "storage_available_gb": "Hardware",
	"hardware_encryption_caps": "Hardware",
	"hostname": "Network", "ip_address": "Network", "mac_address": "Network",
	"wifi_mac": "Network", "bluetooth_mac": "Network",
	"ethernet_mac": "Network", "local_hostname": "Network",
	"FileVaultEnabled": "Security", "firewall_enabled": "Security", "password_present": "Security",
	"password_length": "Security", "bitlocker_enabled": "Security", "bitlocker_status": "Security",
	"encryption_enabled": "Security", "is_supervised": "Security",
	"authenticated_root_volume": "Security", "activation_lock_manageable": "Security",
	"external_boot_level": "Security",
	"activation_lock_enabled": "Security", "find_my_mac_enabled": "Security",
	"icloud_backup_enabled": "Backup", "last_icloud_backup": "Backup",
	"timezone": "General", "has_battery": "Hardware",
	"auto_update_check": "Updates", "auto_os_install": "Updates",
	"auto_security_updates": "Updates", "auto_app_install": "Updates",
	"background_download": "Updates", "last_update_scan": "Updates",
	"diagnostics_enabled": "Privacy", "app_analytics_enabled": "Privacy",
	"build_version": "Operating System",
	"topic": "MDM", "push_magic": "MDM", "has_token": "MDM", "mdm_enrolled": "MDM",
	// Windows-specific
	"manufacturer": "Hardware", "model": "Hardware", "dev_id": "Hardware",
	"device_name": "Hardware", "processor_arch": "Hardware",
	"total_ram": "Hardware", "total_storage": "Hardware",
	"firmware_version": "Hardware", "hardware_version": "Hardware",
	"software_version": "Operating System", "os_platform": "Operating System",
	"FirewallEnabled": "Security",
}

var platformKeyLabels = map[string]string{
	"serial": "Serial Number", "architecture": "Architecture", "storage_total_gb": "Total Storage (GB)",
	"storage_free_gb": "Free Storage (GB)", "product_name": "Product Name", "chip": "Chip",
	"storage_capacity_gb": "Storage Capacity (GB)", "storage_available_gb": "Storage Available (GB)",
	"hardware_encryption_caps": "Hardware Encryption",
	"hostname": "Hostname", "ip_address": "IP Address", "mac_address": "MAC Address",
	"wifi_mac": "WiFi MAC", "bluetooth_mac": "Bluetooth MAC",
	"ethernet_mac": "Ethernet MAC", "local_hostname": "Local Hostname",
	"FileVaultEnabled": "FileVault", "firewall_enabled": "Firewall", "password_present": "Password Set",
	"password_length": "Password Length", "bitlocker_enabled": "BitLocker", "bitlocker_status": "BitLocker Status",
	"encryption_enabled": "Encryption", "is_supervised": "Supervised",
	"authenticated_root_volume": "Authenticated Root Volume", "activation_lock_manageable": "Activation Lock Manageable",
	"external_boot_level": "External Boot Level",
	"activation_lock_enabled": "Activation Lock", "find_my_mac_enabled": "Find My Mac",
	"icloud_backup_enabled": "iCloud Backup", "last_icloud_backup": "Last iCloud Backup",
	"timezone": "Time Zone", "has_battery": "Has Battery",
	"auto_update_check": "Auto Check for Updates", "auto_os_install": "Auto Install OS Updates",
	"auto_security_updates": "Auto Security Updates (RSR)", "auto_app_install": "Auto Install App Updates",
	"background_download": "Background Download", "last_update_scan": "Last Update Scan",
	"diagnostics_enabled": "Diagnostics Submission", "app_analytics_enabled": "App Analytics",
	"build_version": "Build Version", "topic": "Push Topic", "push_magic": "Push Magic",
	"has_token": "Has Token", "mdm_enrolled": "MDM Enrolled",
	// Windows-specific
	"manufacturer": "Manufacturer", "model": "Model", "dev_id": "Device ID",
	"device_name": "Device Name", "processor_arch": "Processor Architecture",
	"total_ram": "Total RAM", "total_storage": "Total Storage",
	"firmware_version": "Firmware Version", "hardware_version": "Hardware Version",
	"software_version": "Software Version", "os_platform": "OS Platform",
	"FirewallEnabled": "Firewall",
}

// platformValueTranslations maps raw CSP numeric values to human-readable text.
var platformValueTranslations = map[string]map[string]string{
	"processor_arch": {
		"0": "x86", "5": "ARM", "9": "x64 (AMD64)", "12": "ARM64",
	},
}

// bitlockerStatusBits maps DeviceEncryptionStatus bitmask bits to descriptions.
// Value 0 = compliant. Non-zero is a bitmask per BitLocker CSP docs:
// https://learn.microsoft.com/en-us/windows/client-management/mdm/bitlocker-csp#statusdeviceencryptionstatus
//
// Verified against real Windows 11 ARM64 VM (2026-04-29):
//   FullyEncrypted + ProtectionOn  → 0
//   Suspended / Decrypting / Decrypted → 2 (bit 1)
//   EncryptionInProgress → 0
var bitlockerStatusBits = []string{
	"User consent needed",                   // bit 0  — user must launch BitLocker wizard
	"Protection off or suspended",           // bit 1  — MS: "encryption method doesn't match policy"; real-world: any state where protection is not active
	"OS volume unprotected",                 // bit 2  — OS drive has no BitLocker protection
	"TPM-only protector required",           // bit 3  — policy requires TPM-only but not configured
	"TPM+PIN required",                      // bit 4  — policy requires TPM+PIN
	"TPM+startup key required",              // bit 5  — policy requires TPM+startup key
	"TPM+PIN+startup key required",          // bit 6  — policy requires all three
	"TPM required but not present",          // bit 7  — policy requires TPM protector, TPM not used
	"Recovery key backup failed",            // bit 8  — recovery key couldn't be backed up
	"Fixed drive unprotected",               // bit 9  — fixed data drive not encrypted
	"Fixed drive encryption method mismatch", // bit 10 — fixed drive method doesn't match policy
	"Admin sign-in required",                // bit 11 — need admin or AllowStandardUserEncryption=1
	"WinRE not configured",                  // bit 12 — Windows Recovery Environment missing
	"TPM not available",                     // bit 13 — no TPM, disabled in registry, or removable drive
	"TPM not ready",                         // bit 14 — TPM present but needs initialization
	"Network unavailable for key backup",    // bit 15 — can't back up recovery key to network
}

func decodeBitLockerStatus(val string) string {
	if val == "0" {
		return "Encrypted (Compliant)"
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return val
	}
	var issues []string
	for i, desc := range bitlockerStatusBits {
		if n&(1<<i) != 0 {
			issues = append(issues, desc)
		}
	}
	if len(issues) == 0 {
		return "Non-compliant (code " + val + ")"
	}
	return strings.Join(issues, "; ")
}

func buildPlatformDetails(pd models.JSONB) []platformDetailGroup {
	if len(pd) == 0 {
		return nil
	}
	// Skip large array/object fields that have their own tabs
	skipKeys := map[string]bool{
		"installed_profiles": true, "installed_apps": true,
		"installed_profiles_count": true, "installed_apps_count": true,
		"secure_boot": true,
		"certificates": true, "certificates_count": true,
		"managed_app_ids": true, "managed_apps_count": true,
		"available_os_updates": true, "available_os_updates_count": true,
		"os_update_status": true,
		"active_extensions": true, "active_extensions_count": true,
		"local_users": true, "local_users_count": true,
	}
	groups := map[string][]platformDetailItem{}
	for k, v := range pd {
		if skipKeys[k] {
			continue
		}
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
		// Translate raw numeric CSP values to human-readable text
		if k == "bitlocker_status" {
			item.Value = decodeBitLockerStatus(item.Value)
		} else if k == "total_ram" || k == "total_storage" {
			if f, err := strconv.ParseFloat(item.Value, 64); err == nil {
				if f >= 1024 {
					item.Value = fmt.Sprintf("%.1f GB", f/1024)
				} else {
					item.Value = fmt.Sprintf("%.0f MB", f)
				}
			}
		} else if translated, ok := platformValueTranslations[k][item.Value]; ok {
			item.Value = translated
		}
		groups[cat] = append(groups[cat], item)
	}
	order := []string{"Hardware", "Network", "Security", "Operating System", "Updates", "Backup", "Privacy", "General", "MDM", "Other"}
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
