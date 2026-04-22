package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/reporting"
)

// Report handlers (S5-02)

func (s *Server) handleDeviceReport(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	platform := r.URL.Query().Get("platform")
	rows, err := s.reportService.DeviceInventory(r.Context(), user.EnterpriseID, platform)
	if err != nil {
		s.logger.Error("failed to generate device report", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate report")
		return
	}

	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=devices.csv")
		headers := []string{"id", "platform", "name", "serial_number", "os_version", "status"}
		var csvRows [][]string
		for _, d := range rows {
			csvRows = append(csvRows, []string{d.ID.String(), d.Platform, d.Name, d.SerialNumber, d.OSVersion, d.Status})
		}
		reporting.WriteCSV(w, headers, csvRows)
		return
	}
	respondJSON(w, r, http.StatusOK, rows)
}

func (s *Server) handleComplianceReport(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	rows, err := s.reportService.ComplianceReport(r.Context(), user.EnterpriseID)
	if err != nil {
		s.logger.Error("failed to generate compliance report", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate report")
		return
	}

	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=compliance.csv")
		headers := []string{"device_id", "device_name", "platform", "policy_name", "status", "evaluated_at"}
		var csvRows [][]string
		for _, c := range rows {
			csvRows = append(csvRows, []string{c.DeviceID.String(), c.DeviceName, c.Platform, c.PolicyName, c.Status, c.EvaluatedAt.Format(time.RFC3339)})
		}
		reporting.WriteCSV(w, headers, csvRows)
		return
	}
	respondJSON(w, r, http.StatusOK, rows)
}

func (s *Server) handleEnrollmentReport(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	rows, err := s.reportService.EnrollmentReport(r.Context(), user.EnterpriseID, days)
	if err != nil {
		s.logger.Error("failed to generate enrollment report", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate report")
		return
	}
	respondJSON(w, r, http.StatusOK, rows)
}

// Audit log handlers
func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)
	action := r.URL.Query().Get("action")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	var logs []*models.AuditLog
	var total int

	if action != "" || startDate != "" || endDate != "" {
		logs, total, err = s.auditLogRepo.Search(r.Context(), user.EnterpriseID, action, startDate, endDate, limit, offset)
	} else {
		logs, total, err = s.auditLogRepo.List(r.Context(), user.EnterpriseID, limit, offset)
	}
	if err != nil {
		s.logger.Error("failed to list audit logs", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list audit logs")
		return
	}

	respondPaginated(w, r, http.StatusOK, logs, total, limit, offset)
}
