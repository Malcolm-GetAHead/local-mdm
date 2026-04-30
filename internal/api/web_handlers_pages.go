package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// handleWebPolicyList shows the policy list with filtering.
func (s *Server) handleWebPolicyList(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	platform := r.URL.Query().Get("platform")
	sortField := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	if sortField == "" {
		sortField = "name"
	}
	if sortDir == "" {
		sortDir = "asc"
	}

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

	// Sort
	sort.Slice(policies, func(i, j int) bool {
		less := strings.ToLower(policies[i].Name) < strings.ToLower(policies[j].Name)
		if sortDir == "desc" {
			return !less
		}
		return less
	})

	data := map[string]interface{}{
		"ActiveNav": "policies",
		"Policies":  policies,
		"Filter":    map[string]string{"Platform": platform, "Sort": sortField, "Dir": sortDir},
	}

	if isHTMXFragment(r) {
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

	assignments, _ := s.groupService.ListAssignmentsByPolicy(ctx, id)

	s.renderPage(w, r, "policy_form", map[string]interface{}{
		"ActiveNav":       "policies",
		"IsEdit":          true,
		"Policy":          policy,
		"SettingGroups":   settingsByCategory(policy.Platform),
		"ActiveSettings":  active,
		"AssignmentCount": len(assignments),
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

// handleWebCompliance shows the compliance dashboard with filters and pagination.
func (s *Server) handleWebCompliance(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	statusFilter := r.URL.Query().Get("status_filter")
	query := strings.ToLower(r.URL.Query().Get("q"))
	sortField := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	if sortField == "" {
		sortField = "device"
	}
	if sortDir == "" {
		sortDir = "asc"
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	compRows, _ := s.reportService.ComplianceReport(ctx, sess.EnterpriseID)

	compliant, nonCompliant, unknown := 0, 0, 0
	type resultRow struct {
		DeviceID    string
		DeviceName  string
		PolicyName  string
		Status      string
		Violations  []string
		EvaluatedAt interface{}
	}
	var allResults []resultRow
	for _, cr := range compRows {
		switch cr.Status {
		case "compliant":
			compliant++
		case "non_compliant":
			nonCompliant++
		default:
			unknown++
		}

		// Apply status filter
		if statusFilter != "" && cr.Status != statusFilter {
			continue
		}
		// Apply text filter
		if query != "" && !strings.Contains(strings.ToLower(cr.DeviceName), query) &&
			!strings.Contains(strings.ToLower(cr.PolicyName), query) {
			continue
		}

		var violations []string
		if v, ok := cr.Details["violations"]; ok {
			switch vv := v.(type) {
			case map[string]interface{}:
				for _, msg := range vv {
					if s, ok := msg.(string); ok {
						violations = append(violations, s)
					}
				}
			case []interface{}:
				for _, item := range vv {
					if s, ok := item.(string); ok {
						violations = append(violations, s)
					}
				}
			}
		}

		allResults = append(allResults, resultRow{
			DeviceID: cr.DeviceID.String(), DeviceName: cr.DeviceName,
			PolicyName: cr.PolicyName, Status: cr.Status,
			Violations: violations, EvaluatedAt: cr.EvaluatedAt,
		})
	}

	// Sort
	sort.Slice(allResults, func(i, j int) bool {
		var less bool
		switch sortField {
		case "device":
			less = strings.ToLower(allResults[i].DeviceName) < strings.ToLower(allResults[j].DeviceName)
		case "policy":
			less = strings.ToLower(allResults[i].PolicyName) < strings.ToLower(allResults[j].PolicyName)
		case "status":
			less = allResults[i].Status < allResults[j].Status
		default:
			less = strings.ToLower(allResults[i].DeviceName) < strings.ToLower(allResults[j].DeviceName)
		}
		if sortDir == "desc" {
			return !less
		}
		return less
	})

	// Paginate
	total := len(allResults)
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
		"ActiveNav":   "compliance",
		"Stats":       map[string]int{"Compliant": compliant, "NonCompliant": nonCompliant, "Unknown": unknown},
		"Results":     allResults[start:end],
		"TotalPages":  totalPages,
		"CurrentPage": page,
		"TotalItems":  total,
		"Filter": map[string]string{
			"StatusFilter": statusFilter, "Query": r.URL.Query().Get("q"),
			"Sort": sortField, "Dir": sortDir,
		},
	}

	if isHTMXFragment(r) {
		s.renderFragment(w, s.webTemplates["compliance"], "compliance_table_body", data)
		return
	}
	s.renderPage(w, r, "compliance", data)
}

// handleWebAuditLog shows the audit log with search/filter and pagination.
func (s *Server) handleWebAuditLog(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	action := r.URL.Query().Get("action")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	logs, total, _ := s.auditLogRepo.Search(ctx, sess.EnterpriseID, action, startDate, endDate, 1000, 0)

	// Batch-resolve user emails (single pass instead of O(n) queries)
	userIDs := make(map[uuid.UUID]bool)
	for _, l := range logs {
		if l.UserID != nil {
			userIDs[*l.UserID] = true
		}
	}
	emailMap := make(map[uuid.UUID]string, len(userIDs))
	for uid := range userIDs {
		if u, err := s.userService.Get(ctx, uid); err == nil {
			emailMap[uid] = u.Email
		}
	}

	// Enrich with user emails and parse details
	var auditLogs []map[string]interface{}
	for _, l := range logs {
		email := ""
		if l.UserID != nil {
			if e, ok := emailMap[*l.UserID]; ok {
				email = e
			} else if e, ok := l.Details["user_email"].(string); ok {
				email = e
			} else {
				email = l.UserID.String()[:8] + "…"
			}
		}

		// Parse details into readable summary and map for expansion
		summary := ""
		detailsMap := map[string]string{}
		if l.ResourceID != nil {
			detailsMap["resource_id"] = l.ResourceID.String()
		}
		for k, v := range l.Details {
			if k == "request_id" || k == "user_email" {
				continue
			}
			detailsMap[k] = fmt.Sprintf("%v", v)
			if summary != "" {
				summary += "; "
			}
			summary += fmt.Sprintf("%s: %v", k, v)
		}
		if len(summary) > 100 {
			summary = summary[:100] + "…"
		}
		if summary == "" {
			summary = "—"
		}

		auditLogs = append(auditLogs, map[string]interface{}{
			"CreatedAt":      l.CreatedAt,
			"UserEmail":      email,
			"Action":         l.Action,
			"ResourceType":   l.ResourceType,
			"DetailsSummary": summary,
			"DetailsMap":     detailsMap,
		})
	}

	_ = total
	totalItems := len(auditLogs)
	totalPages := (totalItems + perPage - 1) / perPage
	start := (page - 1) * perPage
	if start > totalItems {
		start = totalItems
	}
	end := start + perPage
	if end > totalItems {
		end = totalItems
	}

	data := map[string]interface{}{
		"ActiveNav":   "audit",
		"AuditLogs":   auditLogs[start:end],
		"TotalPages":  totalPages,
		"CurrentPage": page,
		"TotalItems":  totalItems,
		"Filter": map[string]string{
			"Action": action, "StartDate": startDate, "EndDate": endDate,
		},
	}

	if isHTMXFragment(r) {
		s.renderFragment(w, s.webTemplates["audit"], "audit_table_body", data)
		return
	}
	s.renderPage(w, r, "audit", data)
}

// handleWebGroups shows the device groups list.
func (s *Server) handleWebGroups(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()

	sortDir := r.URL.Query().Get("dir")
	if sortDir == "" {
		sortDir = "asc"
	}

	groups, _, _ := s.groupService.ListGroups(ctx, sess.EnterpriseID, 100, 0)

	type groupRow struct {
		ID          string
		Name        string
		Description string
		MemberCount int
	}
	var groupIDs []uuid.UUID
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}
	memberCounts, _ := s.groupService.CountMembersByGroupIDs(ctx, groupIDs)
	var rows []groupRow
	for _, g := range groups {
		rows = append(rows, groupRow{
			ID:          g.ID.String(),
			Name:        g.Name,
			Description: g.Description,
			MemberCount: memberCounts[g.ID],
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		less := strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
		if sortDir == "desc" {
			return !less
		}
		return less
	})

	data := map[string]interface{}{
		"ActiveNav": "groups",
		"Groups":    rows,
		"Sort":      "name",
		"Dir":       sortDir,
	}

	if isHTMXFragment(r) {
		s.renderFragment(w, s.webTemplates["groups"], "groups_table_body", data)
		return
	}
	s.renderPage(w, r, "groups", data)
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

	if isHTMXFragment(r) {
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
		http.Redirect(w, r, "/dashboard/groups", http.StatusFound)
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

	s.renderGroupMemberList(w, r, groupID)
}

// handleWebGroupRemoveMember removes a device from a group.
func (s *Server) handleWebGroupRemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID, _ := uuid.Parse(mux.Vars(r)["id"])
	deviceID, _ := uuid.Parse(mux.Vars(r)["device_id"])

	s.groupService.RemoveMember(ctx, groupID, deviceID)
	s.logAudit(r, "group.remove_member", "group", groupID, map[string]interface{}{"device_id": deviceID})

	s.renderGroupMemberList(w, r, groupID)
}

// renderGroupMemberList renders just the member list fragment.
func (s *Server) renderGroupMemberList(w http.ResponseWriter, r *http.Request, groupID uuid.UUID) {
	sess := getSession(r)
	ctx := r.Context()

	group, err := s.groupService.GetGroup(ctx, groupID)
	if err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	members, _, _ := s.groupService.ListMembers(ctx, groupID, 1000, 0)
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
		"Group":      group,
		"AllDevices": rows,
	}
	s.renderFragment(w, s.webTemplates["group_detail"], "member_tbody", data)
}

// handleWebGroupEdit updates a group's name/description.
func (s *Server) handleWebGroupEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := uuid.Parse(mux.Vars(r)["id"])
	r.ParseForm()

	group, err := s.groupService.GetGroup(ctx, id)
	if err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	group.Name = r.FormValue("name")
	group.Description = r.FormValue("description")
	s.groupService.UpdateGroup(ctx, group)
	s.logAudit(r, "group.update", "group", id, map[string]interface{}{"name": group.Name})
	http.Redirect(w, r, fmt.Sprintf("/dashboard/groups/%s", id), http.StatusFound)
}

// handleWebGroupDelete deletes a group.
func (s *Server) handleWebGroupDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID, _ := uuid.Parse(mux.Vars(r)["id"])

	if err := s.groupService.DeleteGroup(ctx, groupID); err != nil {
		http.Error(w, "Failed to delete group", http.StatusInternalServerError)
		return
	}
	s.logAudit(r, "group.delete", "group", groupID, nil)
	w.Header().Set("HX-Trigger", `{"showToast":{"message":"Group deleted","type":"success"}}`)
	// Return empty string to remove the table row via hx-swap="outerHTML"
	w.WriteHeader(http.StatusOK)
}

// handleWebPolicyDelete deletes a policy if it has no assignments.
func (s *Server) handleWebPolicyDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := uuid.Parse(mux.Vars(r)["id"])

	// Check for active assignments
	assignments, _ := s.groupService.ListAssignmentsByPolicy(ctx, id)
	if len(assignments) > 0 {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<div class="p-3 bg-red-50 text-red-700 rounded-lg text-sm">Cannot delete: policy is assigned to devices or groups. Remove all assignments first.</div>`)
		return
	}

	if err := s.policyRepo.Delete(ctx, id); err != nil {
		http.Error(w, "Failed to delete policy", http.StatusInternalServerError)
		return
	}
	s.logAudit(r, "policy.delete", "policy", id, nil)
	w.Header().Set("HX-Trigger", `{"showToast":{"message":"Policy deleted","type":"success"}}`)
	w.WriteHeader(http.StatusOK)
}

// handleWebPolicyAssignPage shows the policy assignment page.
func (s *Server) handleWebPolicyAssignPage(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	id, _ := uuid.Parse(mux.Vars(r)["id"])

	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Policy not found", http.StatusNotFound)
		return
	}

	groups, _, _ := s.groupService.ListGroups(ctx, sess.EnterpriseID, 100, 0)
	devices, _, _ := s.deviceRepo.List(ctx, sess.EnterpriseID, 1000, 0)
	assignments, _ := s.groupService.ListAssignmentsByPolicy(ctx, id)

	// Build assigned set to filter out
	assignedSet := map[string]bool{}
	type assignRow struct {
		ID         string
		TargetType string
		TargetName string
	}
	var rows []assignRow
	for _, a := range assignments {
		key := a.TargetType + ":" + a.TargetID.String()
		assignedSet[key] = true
		name := a.TargetID.String()
		if a.TargetType == "group" {
			if g, err := s.groupService.GetGroup(ctx, a.TargetID); err == nil {
				name = g.Name
			}
		} else if a.TargetType == "device" {
			if d, err := s.deviceRepo.GetByID(ctx, a.TargetID); err == nil {
				name = d.Name
			}
		} else if a.TargetType == "enterprise" {
			name = "Entire Enterprise"
		}
		rows = append(rows, assignRow{ID: a.ID.String(), TargetType: a.TargetType, TargetName: name})
	}

	// Filter out already-assigned groups and devices
	var availGroups []*models.DeviceGroup
	for _, g := range groups {
		if !assignedSet["group:"+g.ID.String()] {
			availGroups = append(availGroups, g)
		}
	}
	var availDevices []*models.Device
	for _, d := range devices {
		if !assignedSet["device:"+d.ID.String()] {
			availDevices = append(availDevices, d)
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50
	totalAssign := len(rows)
	totalPages := (totalAssign + perPage - 1) / perPage
	start := (page - 1) * perPage
	if start > totalAssign {
		start = totalAssign
	}
	end := start + perPage
	if end > totalAssign {
		end = totalAssign
	}

	s.renderPage(w, r, "policy_assign", map[string]interface{}{
		"ActiveNav":        "policies",
		"Policy":           policy,
		"AvailableGroups":  availGroups,
		"AvailableDevices": availDevices,
		"Assignments":      rows[start:end],
		"TotalAssignments": totalAssign,
		"CurrentPage":      page,
		"TotalPages":       totalPages,
	})
}

// handleWebPolicyAssign creates a policy assignment.
func (s *Server) handleWebPolicyAssign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	policyID, _ := uuid.Parse(mux.Vars(r)["id"])
	r.ParseForm()

	targetType := r.FormValue("target_type")
	targetID, _ := uuid.Parse(r.FormValue("target_id"))

	s.groupService.AssignPolicy(ctx, policyID, targetType, targetID, 0)
	s.logAudit(r, "policy.assign", "policy", policyID, map[string]interface{}{"target_type": targetType, "target_id": targetID})
	http.Redirect(w, r, fmt.Sprintf("/dashboard/policies/%s/assign", policyID), http.StatusFound)
}

// handleWebPolicyUnassign removes a policy assignment.
func (s *Server) handleWebPolicyUnassign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	assignmentID, _ := uuid.Parse(mux.Vars(r)["assignment_id"])

	s.groupService.UnassignPolicy(ctx, assignmentID)
	s.logAudit(r, "policy.unassign", "policy", uuid.Nil, map[string]interface{}{"assignment_id": assignmentID})
	w.Header().Set("HX-Trigger", `{"showToast":{"message":"Assignment removed","type":"success"}}`)
	w.WriteHeader(http.StatusOK)
}

// handleWebEnrollmentTokens shows the enrollment tokens list page.
func (s *Server) handleWebEnrollmentTokens(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	tokens, total, _ := s.enrollmentTokenRepo.List(r.Context(), sess.EnterpriseID, perPage, (page-1)*perPage)

	// Enrich tokens with computed status fields
	now := time.Now()
	type tokenView struct {
		ID            uuid.UUID
		Token         string
		Description   string
		MaxUses       *int
		UsesRemaining *int
		ExpiresAt     time.Time
		CreatedAt     time.Time
		RevokedAt     *time.Time
		Expired       bool
		Exhausted     bool
	}
	var views []tokenView
	for _, t := range tokens {
		v := tokenView{
			ID: t.ID, Token: t.Token, Description: t.Description,
			MaxUses: t.MaxUses, UsesRemaining: t.UsesRemaining,
			ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt, RevokedAt: t.RevokedAt,
			Expired:   now.After(t.ExpiresAt),
			Exhausted: t.UsesRemaining != nil && *t.UsesRemaining <= 0,
		}
		views = append(views, v)
	}

	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"ActiveNav":   "enrollment-tokens",
		"Tokens":      views,
		"TotalPages":  totalPages,
		"CurrentPage": page,
		"TotalItems":  total,
	}

	if isHTMXFragment(r) {
		s.renderFragment(w, s.webTemplates["enrollment_tokens"], "enrollment_tokens_table_body", data)
		return
	}
	s.renderPage(w, r, "enrollment_tokens", data)
}

// handleWebEnrollmentTokenCreate creates a new enrollment token from the dashboard form.
func (s *Server) handleWebEnrollmentTokenCreate(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	r.ParseForm()

	description := r.FormValue("description")
	maxUsesStr := r.FormValue("max_uses")
	expiresIn := r.FormValue("expires_in")
	if expiresIn == "" {
		expiresIn = "24h"
	}

	dur, err := time.ParseDuration(expiresIn)
	if err != nil {
		dur = 24 * time.Hour
	}

	var maxUses *int
	if maxUsesStr != "" {
		if v, err := strconv.Atoi(maxUsesStr); err == nil && v > 0 {
			maxUses = &v
		}
	}

	tokenStr, err := generateEnrollmentToken()
	if err != nil {
		w.Header().Set("HX-Trigger", `{"showToast":{"message":"Failed to generate token","type":"error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token := &models.EnrollmentToken{
		EnterpriseID:  sess.EnterpriseID,
		Token:         tokenStr,
		Description:   description,
		MaxUses:       maxUses,
		UsesRemaining: maxUses,
		ExpiresAt:     time.Now().Add(dur),
	}

	if err := s.enrollmentTokenRepo.Create(r.Context(), token); err != nil {
		w.Header().Set("HX-Trigger", `{"showToast":{"message":"Failed to create token","type":"error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.logAudit(r, "enrollment_token.create", "enrollment_token", token.ID, map[string]interface{}{
		"enterprise_id": sess.EnterpriseID,
		"max_uses":      maxUses,
		"expires_at":    token.ExpiresAt,
	})

	// Re-render the full page with the created token banner
	r2 := r.Clone(r.Context())
	r2.Method = "GET"
	q := r2.URL.Query()
	q.Del("page")
	r2.URL.RawQuery = q.Encode()
	// Fetch fresh list
	tokens, total, _ := s.enrollmentTokenRepo.List(r.Context(), sess.EnterpriseID, 50, 0)
	now := time.Now()
	type tokenView struct {
		ID            uuid.UUID
		Token         string
		Description   string
		MaxUses       *int
		UsesRemaining *int
		ExpiresAt     time.Time
		CreatedAt     time.Time
		RevokedAt     *time.Time
		Expired       bool
		Exhausted     bool
	}
	var views []tokenView
	for _, t := range tokens {
		v := tokenView{
			ID: t.ID, Token: t.Token, Description: t.Description,
			MaxUses: t.MaxUses, UsesRemaining: t.UsesRemaining,
			ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt, RevokedAt: t.RevokedAt,
			Expired:   now.After(t.ExpiresAt),
			Exhausted: t.UsesRemaining != nil && *t.UsesRemaining <= 0,
		}
		views = append(views, v)
	}
	totalPages := (total + 49) / 50

	data := map[string]interface{}{
		"ActiveNav":    "enrollment-tokens",
		"Tokens":       views,
		"TotalPages":   totalPages,
		"CurrentPage":  1,
		"TotalItems":   total,
		"CreatedToken": tokenStr,
	}
	s.renderFragment(w, s.webTemplates["enrollment_tokens"], "enrollment_tokens_table_body", data)
}

// handleWebEnrollmentTokenRevoke revokes an enrollment token from the dashboard.
func (s *Server) handleWebEnrollmentTokenRevoke(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		w.Header().Set("HX-Trigger", `{"showToast":{"message":"Invalid token ID","type":"error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.enrollmentTokenRepo.Revoke(r.Context(), id); err != nil {
		w.Header().Set("HX-Trigger", `{"showToast":{"message":"Failed to revoke token","type":"error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.logAudit(r, "enrollment_token.revoke", "enrollment_token", id, nil)

	// Re-render table
	tokens, total, _ := s.enrollmentTokenRepo.List(r.Context(), sess.EnterpriseID, 50, 0)
	now := time.Now()
	type tokenView struct {
		ID            uuid.UUID
		Token         string
		Description   string
		MaxUses       *int
		UsesRemaining *int
		ExpiresAt     time.Time
		CreatedAt     time.Time
		RevokedAt     *time.Time
		Expired       bool
		Exhausted     bool
	}
	var views []tokenView
	for _, t := range tokens {
		v := tokenView{
			ID: t.ID, Token: t.Token, Description: t.Description,
			MaxUses: t.MaxUses, UsesRemaining: t.UsesRemaining,
			ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt, RevokedAt: t.RevokedAt,
			Expired:   now.After(t.ExpiresAt),
			Exhausted: t.UsesRemaining != nil && *t.UsesRemaining <= 0,
		}
		views = append(views, v)
	}
	totalPages := (total + 49) / 50

	data := map[string]interface{}{
		"ActiveNav":   "enrollment-tokens",
		"Tokens":      views,
		"TotalPages":  totalPages,
		"CurrentPage": 1,
		"TotalItems":  total,
	}
	w.Header().Set("HX-Trigger", `{"showToast":{"message":"Token revoked","type":"success"}}`)
	s.renderFragment(w, s.webTemplates["enrollment_tokens"], "enrollment_tokens_table_body", data)
}
