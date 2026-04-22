package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Device Group handlers (Sprint 4)

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	limit, offset := parsePagination(r)
	groups, total, err := s.groupService.ListGroups(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list groups")
		return
	}
	respondPaginated(w, r, http.StatusOK, groups, total, limit, offset)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.Name == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "Name is required")
		return
	}
	enterpriseID := uuid.Nil
	if user != nil {
		enterpriseID = user.EnterpriseID
	}
	group := &models.DeviceGroup{EnterpriseID: enterpriseID, Name: req.Name, Description: req.Description}
	if err := s.groupService.CreateGroup(r.Context(), group); err != nil {
		if isDuplicateError(err) {
			respondError(w, r, http.StatusConflict, "duplicate", "Group name already exists")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create group")
		return
	}
	s.logAudit(r, "group.create", "group", group.ID, map[string]interface{}{"name": group.Name})
	respondJSON(w, r, http.StatusCreated, group)
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid group ID")
		return
	}
	group, err := s.groupService.GetGroup(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get group")
		return
	}
	respondJSON(w, r, http.StatusOK, group)
}

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid group ID")
		return
	}
	group, err := s.groupService.GetGroup(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get group")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Description != nil {
		group.Description = *req.Description
	}
	if err := s.groupService.UpdateGroup(r.Context(), group); err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update group")
		return
	}
	s.logAudit(r, "group.update", "group", id, nil)
	respondJSON(w, r, http.StatusOK, group)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid group ID")
		return
	}
	if err := s.groupService.DeleteGroup(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete group")
		return
	}
	s.logAudit(r, "group.delete", "group", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid group ID")
		return
	}
	limit, offset := parsePagination(r)
	devices, total, err := s.groupService.ListMembers(r.Context(), id, limit, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list members")
		return
	}
	respondPaginated(w, r, http.StatusOK, devices, total, limit, offset)
}

func (s *Server) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid group ID")
		return
	}
	var req struct {
		DeviceID uuid.UUID `json:"device_id"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.DeviceID == uuid.Nil {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "device_id is required")
		return
	}
	if err := s.groupService.AddMember(r.Context(), groupID, req.DeviceID); err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to add member")
		return
	}
	s.logAudit(r, "group.add_member", "group", groupID, map[string]interface{}{"device_id": req.DeviceID})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid group ID")
		return
	}
	deviceID, err := parseUUIDParam(r, "device_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID")
		return
	}
	if err := s.groupService.RemoveMember(r.Context(), groupID, deviceID); err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to remove member")
		return
	}
	s.logAudit(r, "group.remove_member", "group", groupID, map[string]interface{}{"device_id": deviceID})
	w.WriteHeader(http.StatusNoContent)
}
