package api

import (
	"encoding/json"
	"net/http"

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
	s.renderPage(w, r, "policy_form", map[string]interface{}{
		"ActiveNav": "policies",
		"Policy": map[string]interface{}{
			"ID": "", "Name": "", "Description": "", "Platform": "all",
			"PolicyType": "security", "PolicyConfig": map[string]interface{}{}, "IsActive": true,
		},
	})
}

// handleWebPolicyCreate handles POST to create a new policy.
func (s *Server) handleWebPolicyCreate(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ctx := r.Context()
	r.ParseForm()

	var config models.JSONB
	json.Unmarshal([]byte(r.FormValue("policy_config")), &config)

	policy := &models.Policy{
		EnterpriseID: sess.EnterpriseID,
		Name:         r.FormValue("name"),
		Description:  r.FormValue("description"),
		Platform:     r.FormValue("platform"),
		PolicyType:   r.FormValue("policy_type"),
		PolicyConfig: config,
		IsActive:     r.FormValue("is_active") == "on",
	}

	if err := s.policyRepo.Create(ctx, policy); err != nil {
		s.renderPage(w, r, "policy_form", map[string]interface{}{
			"ActiveNav": "policies",
			"Policy":    policy,
			"Error":     "Failed to create policy: " + err.Error(),
		})
		return
	}

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

	s.renderPage(w, r, "policy_form", map[string]interface{}{
		"ActiveNav": "policies",
		"Policy":    policy,
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
	policy.Name = r.FormValue("name")
	policy.Description = r.FormValue("description")
	policy.Platform = r.FormValue("platform")
	policy.PolicyType = r.FormValue("policy_type")
	policy.IsActive = r.FormValue("is_active") == "on"

	var config models.JSONB
	json.Unmarshal([]byte(r.FormValue("policy_config")), &config)
	policy.PolicyConfig = config

	if err := s.policyRepo.Update(ctx, policy); err != nil {
		s.renderPage(w, r, "policy_form", map[string]interface{}{
			"ActiveNav": "policies",
			"Policy":    policy,
			"Error":     "Failed to update policy: " + err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/dashboard/policies", http.StatusFound)
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
