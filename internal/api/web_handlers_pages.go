package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// handleWebPolicyList shows the policy list with filtering.
func (s *Server) handleWebPolicyList(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	platform := r.URL.Query().Get("platform")

	policies, _, _ := s.policyRepo.List(ctx, sess.EnterpriseID, 1000, 0)

	if platform != "" {
		var filtered []*models.Policy
		for _, p := range policies {
			if p.Platform == platform {
				filtered = append(filtered, p)
			}
		}
		policies = filtered
	}

	data := map[string]interface{}{
		"ActiveNav": "policies",
		"Policies":  policies,
		"Filter":    map[string]string{"Platform": platform},
	}

	if isHTMX(r) {
		s.renderFragment(w, s.webTemplates["policies"], "policy_table_body", data)
		return
	}
	s.renderPage(w, r, "policies", data)
}

// handleWebPolicyNew shows the create policy form.
func (s *Server) handleWebPolicyNew(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "all"
	}
	s.renderPage(w, r, "policy_form", map[string]interface{}{
		"ActiveNav":      "policies",
		"IsEdit":         false,
		"Policy":         map[string]interface{}{"ID": "", "Name": "", "Description": "", "Platform": platform, "IsActive": true},
		"SettingGroups":  settingsByCategory(platform),
		"ActiveSettings": map[string]interface{}{},
	})
}

// handleWebPolicyCreate handles POST to create a new policy.
func (s *Server) handleWebPolicyCreate(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	r.ParseForm()

	platform := r.FormValue("platform")
	config, invalid := parseSettingsFromForm(r)
	if len(invalid) > 0 {
		s.renderPage(w, r, "policy_form", map[string]interface{}{
			"ActiveNav":      "policies",
			"IsEdit":         false,
			"Policy":         map[string]interface{}{"Name": r.FormValue("name"), "Description": r.FormValue("description"), "Platform": platform, "IsActive": r.FormValue("is_active") == "on"},
			"SettingGroups":  settingsByCategory(platform),
			"ActiveSettings": config,
			"Error":          "Unknown settings: " + strings.Join(invalid, ", "),
		})
		return
	}

	policy := &models.Policy{
		EnterpriseID: sess.EnterpriseID,
		Name:         r.FormValue("name"),
		Description:  r.FormValue("description"),
		Platform:     platform,
		PolicyType:   detectPolicyType(config),
		PolicyConfig: config,
		IsActive:     r.FormValue("is_active") == "on",
	}

	if err := s.policyRepo.Create(ctx, policy); err != nil {
		s.renderPage(w, r, "policy_form", map[string]interface{}{
			"ActiveNav":      "policies",
			"IsEdit":         false,
			"Policy":         policy,
			"SettingGroups":  settingsByCategory(platform),
			"ActiveSettings": config,
			"Error":          "Failed to create policy: " + err.Error(),
		})
		return
	}
	s.logAudit(r, "policy.create", "policy", policy.ID, map[string]interface{}{"name": policy.Name})
	http.Redirect(w, r, "/dashboard/policies", http.StatusFound)
}

// handleWebPolicyEdit shows the edit form for an existing policy.
func (s *Server) handleWebPolicyEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Policy not found", http.StatusNotFound)
		return
	}

	active := map[string]interface{}{}
	for k, v := range policy.PolicyConfig {
		active[k] = fmt.Sprintf("%v", v)
	}

	s.renderPage(w, r, "policy_form", map[string]interface{}{
		"ActiveNav":      "policies",
		"IsEdit":         true,
		"Policy":         policy,
		"SettingGroups":  settingsByCategory(policy.Platform),
		"ActiveSettings": active,
	})
}

// handleWebPolicyUpdate handles POST to update an existing policy.
func (s *Server) handleWebPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Policy not found", http.StatusNotFound)
		return
	}

	r.ParseForm()
	platform := r.FormValue("platform")
	config, invalid := parseSettingsFromForm(r)
	if len(invalid) > 0 {
		active := map[string]interface{}{}
		for k, v := range config {
			active[k] = fmt.Sprintf("%v", v)
		}
		s.renderPage(w, r, "policy_form", map[string]interface{}{
			"ActiveNav":      "policies",
			"IsEdit":         true,
			"Policy":         policy,
			"SettingGroups":  settingsByCategory(platform),
			"ActiveSettings": active,
			"Error":          "Unknown settings: " + strings.Join(invalid, ", "),
		})
		return
	}

	policy.Name = r.FormValue("name")
	policy.Description = r.FormValue("description")
	policy.Platform = platform
	policy.PolicyType = detectPolicyType(config)
	policy.PolicyConfig = config
	policy.IsActive = r.FormValue("is_active") == "on"

	if err := s.policyRepo.Update(ctx, policy); err != nil {
		s.renderPage(w, r, "policy_form", map[string]interface{}{
			"ActiveNav":      "policies",
			"IsEdit":         true,
			"Policy":         policy,
			"SettingGroups":  settingsByCategory(platform),
			"ActiveSettings": config,
			"Error":          "Failed to update policy: " + err.Error(),
		})
		return
	}
	s.logAudit(r, "policy.update", "policy", policy.ID, map[string]interface{}{"name": policy.Name})
	http.Redirect(w, r, "/dashboard/policies", http.StatusFound)
}

// handleWebSettingsCatalog returns the settings catalog fragment for HTMX platform switching.
func (s *Server) handleWebSettingsCatalog(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	data := map[string]interface{}{
		"SettingGroups":  settingsByCategory(platform),
		"ActiveSettings": map[string]interface{}{},
	}
	s.renderFragment(w, s.webTemplates["policy_form"], "settings_catalog_body", data)
}

// parseSettingsFromForm extracts setting_* form fields into a policy config map.
// Returns the config and any invalid keys not in the catalog.
func parseSettingsFromForm(r *http.Request) (models.JSONB, []string) {
	valid := validPolicyKeys()
	config := models.JSONB{}
	var invalid []string

	for key, values := range r.Form {
		if !strings.HasPrefix(key, "setting_") {
			continue
		}
		settingKey := strings.TrimPrefix(key, "setting_")
		if !valid[settingKey] {
			invalid = append(invalid, settingKey)
			continue
		}
		val := values[0]
		if val == "" {
			continue
		}
		if val == "true" {
			config[settingKey] = true
		} else if n, err := strconv.Atoi(val); err == nil && n > 0 {
			config[settingKey] = n
		} else {
			config[settingKey] = val
		}
	}
	return config, invalid
}

// detectPolicyType infers the primary policy type from the config keys.
func detectPolicyType(config models.JSONB) string {
	cats := map[string]int{}
	for _, s := range policySettingsCatalog {
		if _, ok := config[s.Key]; ok {
			cats[strings.ToLower(s.Category)]++
		}
	}
	best, bestN := "security", 0
	for cat, n := range cats {
		if n > bestN {
			best, bestN = cat, n
		}
	}
	return best
}

// handleWebCompliance shows the compliance dashboard.
func (s *Server) handleWebCompliance(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	compRows, _ := s.reportService.ComplianceReport(ctx, sess.EnterpriseID)

	compliant, nonCompliant, unknown := 0, 0, 0
	var results []map[string]interface{}
	for _, cr := range compRows {
		switch cr.Status {
		case "compliant":
			compliant++
		case "non_compliant":
			nonCompliant++
		default:
			unknown++
		}

		var violations []string
		// ComplianceRow from reportService has DeviceName and PolicyName already
		results = append(results, map[string]interface{}{
			"DeviceID":    cr.DeviceID,
			"DeviceName":  cr.DeviceName,
			"PolicyName":  cr.PolicyName,
			"Status":      cr.Status,
			"Violations":  violations,
			"EvaluatedAt": cr.EvaluatedAt,
		})
	}

	s.renderPage(w, r, "compliance", map[string]interface{}{
		"ActiveNav": "compliance",
		"Stats": map[string]interface{}{
			"Compliant":    compliant,
			"NonCompliant": nonCompliant,
			"Unknown":      unknown,
		},
		"Results": results,
	})
}

// handleWebAuditLog shows the audit log with search/filter.
func (s *Server) handleWebAuditLog(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	action := r.URL.Query().Get("action")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	logs, _, _ := s.auditLogRepo.Search(ctx, sess.EnterpriseID, action, startDate, endDate, 100, 0)

	// Enrich with user emails
	var auditLogs []map[string]interface{}
	userCache := map[uuid.UUID]string{}
	for _, l := range logs {
		email := ""
		if l.UserID != nil {
			if cached, ok := userCache[*l.UserID]; ok {
				email = cached
			}
			// For seed data, we just show the action
		}
		auditLogs = append(auditLogs, map[string]interface{}{
			"CreatedAt":    l.CreatedAt,
			"UserEmail":    email,
			"Action":       l.Action,
			"ResourceType": l.ResourceType,
			"Details":      l.Details,
		})
	}

	data := map[string]interface{}{
		"ActiveNav": "audit",
		"AuditLogs": auditLogs,
		"Filter": map[string]string{
			"Action":    action,
			"StartDate": startDate,
			"EndDate":   endDate,
		},
	}

	if isHTMX(r) {
		s.renderFragment(w, s.webTemplates["audit"], "audit_table_body", data)
		return
	}
	s.renderPage(w, r, "audit", data)
}

// handleWebGroups shows the device groups list.
func (s *Server) handleWebGroups(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	groups, _, _ := s.groupService.ListGroups(ctx, sess.EnterpriseID, 100, 0)

	type groupRow struct {
		ID          string
		Name        string
		Description string
		MemberCount int
	}
	var rows []groupRow
	for _, g := range groups {
		_, total, _ := s.groupService.ListMembers(ctx, g.ID, 1, 0)
		rows = append(rows, groupRow{
			ID:          g.ID.String(),
			Name:        g.Name,
			Description: g.Description,
			MemberCount: total,
		})
	}

	s.renderPage(w, r, "groups", map[string]interface{}{
		"ActiveNav": "groups",
		"Groups":    rows,
	})
}

// handleWebGroupDetail shows a single group with member toggle UI.
func (s *Server) handleWebGroupDetail(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	group, err := s.groupService.GetGroup(ctx, id)
	if err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	members, _, _ := s.groupService.ListMembers(ctx, id, 1000, 0)
	memberSet := map[uuid.UUID]bool{}
	for _, m := range members {
		memberSet[m.ID] = true
	}

	allDevices, _, _ := s.deviceRepo.List(ctx, sess.EnterpriseID, 1000, 0)

	type deviceRow struct {
		ID       string
		Name     string
		Platform string
		Model    string
		Status   string
		InGroup  bool
	}
	var rows []deviceRow
	for _, d := range allDevices {
		rows = append(rows, deviceRow{
			ID: d.ID.String(), Name: d.Name, Platform: d.Platform,
			Model: d.Model, Status: d.Status, InGroup: memberSet[d.ID],
		})
	}

	data := map[string]interface{}{
		"ActiveNav":  "groups",
		"Group":      group,
		"AllDevices": rows,
	}

	if isHTMX(r) {
		s.renderFragment(w, s.webTemplates["group_detail"], "content", data)
		return
	}
	s.renderPage(w, r, "group_detail", data)
}

// handleWebGroupCreate handles POST to create a new group.
func (s *Server) handleWebGroupCreate(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	r.ParseForm()

	group := &models.DeviceGroup{
		EnterpriseID: sess.EnterpriseID,
		Name:         r.FormValue("name"),
		Description:  r.FormValue("description"),
	}
	if err := s.groupService.CreateGroup(ctx, group); err != nil {
		http.Error(w, "Failed to create group: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.logAudit(r, "group.create", "group", group.ID, map[string]interface{}{"name": group.Name})
	http.Redirect(w, r, "/dashboard/groups/"+group.ID.String(), http.StatusFound)
}

// handleWebGroupAddMember adds a device to a group.
func (s *Server) handleWebGroupAddMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID, _ := uuid.Parse(mux.Vars(r)["id"])
	deviceID, _ := uuid.Parse(mux.Vars(r)["device_id"])

	s.groupService.AddMember(ctx, groupID, deviceID)
	s.logAudit(r, "group.add_member", "group", groupID, map[string]interface{}{"device_id": deviceID})

	// Re-render the full member list
	s.handleWebGroupDetail(w, r)
}

// handleWebGroupRemoveMember removes a device from a group.
func (s *Server) handleWebGroupRemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID, _ := uuid.Parse(mux.Vars(r)["id"])
	deviceID, _ := uuid.Parse(mux.Vars(r)["device_id"])

	s.groupService.RemoveMember(ctx, groupID, deviceID)
	s.logAudit(r, "group.remove_member", "group", groupID, map[string]interface{}{"device_id": deviceID})

	s.handleWebGroupDetail(w, r)
}
