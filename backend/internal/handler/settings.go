package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// SettingsHandler handles organization settings.
type SettingsHandler struct {
	repo repository.OrganizationRepository
}

func NewSettingsHandler(repo repository.OrganizationRepository) *SettingsHandler {
	return &SettingsHandler{repo: repo}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	org, err := h.repo.GetByID(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"organization": org.Name,
		"settings":     org.Settings,
	})
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	// Check role — only admins can update settings
	claims := auth.GetClaimsFromContext(ctx)
	if claims != nil && claims.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "only admins can update settings")
		return
	}

	org, err := h.repo.GetByID(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	var req struct {
		Name     *string                     `json:"name,omitempty"`
		Settings *model.OrganizationSettings `json:"settings,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.Settings != nil {
		org.Settings = *req.Settings
	}

	if err := h.repo.Update(ctx, org); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "updated",
		"organization": org.Name,
		"settings":     org.Settings,
	})
}
