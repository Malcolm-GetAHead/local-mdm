package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/audit"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Helper functions

func parseJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(mux.Vars(r)[name])
}

func parsePagination(r *http.Request) (int, int) {
	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func respondPaginated(w http.ResponseWriter, r *http.Request, status int, data interface{}, total, limit, offset int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	requestID, _ := r.Context().Value(requestIDKey).(string)
	page := (offset / limit) + 1
	totalPages := (total + limit - 1) / limit

	response := Response{
		Data: data,
		Meta: &MetaInfo{
			Timestamp:  time.Now(),
			RequestID:  requestID,
			Page:       page,
			PerPage:    limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) logAudit(r *http.Request, action, resourceType string, resourceID uuid.UUID, details map[string]interface{}) {
	var enterpriseID, userID uuid.UUID
	if user, err := auth.UserFromContext(r.Context()); err == nil {
		enterpriseID = user.EnterpriseID
		if uid, err := uuid.Parse(user.ID); err == nil {
			userID = uid
		}
		if details == nil {
			details = make(map[string]interface{})
		}
		if user.Email != "" {
			details["user_email"] = user.Email
		}
	}

	// Propagate request ID into audit details
	if details == nil {
		details = make(map[string]interface{})
	}
	if reqID, ok := r.Context().Value(requestIDKey).(string); ok && reqID != "" {
		details["request_id"] = reqID
	}

	ip := net.ParseIP(strings.Split(r.RemoteAddr, ":")[0])

	_ = s.auditLogger.Log(r.Context(), audit.Event{
		EnterpriseID: enterpriseID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    ip,
		UserAgent:    r.UserAgent(),
	})
}

func isValidPlatform(p string) bool {
	return p == models.PlatformWindows || p == models.PlatformMacOS || p == models.PlatformAndroid || p == models.PlatformAll
}

func isValidPolicyType(t string) bool {
	switch t {
	case models.PolicyTypeWiFi, models.PolicyTypeVPN, models.PolicyTypeSecurity,
		models.PolicyTypeApp, models.PolicyTypeRestriction, models.PolicyTypeCompliance:
		return true
	}
	return false
}

func isDuplicateError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "already exists")
}
