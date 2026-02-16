package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/remediation"
)

// RemediationHandler handles remediation API requests.
//
// NOTE: The executor (*remediation.Executor) must expose the following proxy
// methods that delegate to the underlying repository:
//   - List(ctx, filter, pagination) ([]model.RemediationAction, int, error)
//   - GetByID(ctx, id) (*model.RemediationAction, error)
//   - GetSummary(ctx, orgID) (*model.RemediationSummary, error)
//   - ListRules(ctx, orgID) ([]model.AutoApprovalRule, error)
//   - GetRuleByID(ctx, id) (*model.AutoApprovalRule, error)
//   - CreateRule(ctx, rule) error
//   - UpdateRule(ctx, rule) error
//   - DeleteRule(ctx, id) error
type RemediationHandler struct {
	executor *remediation.Executor
}

func NewRemediationHandler(executor *remediation.Executor) *RemediationHandler {
	return &RemediationHandler{executor: executor}
}

// List returns remediation actions with optional filtering.
func (h *RemediationHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	page := 1
	pageSize := 50
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := parseInt(p); err == nil && n > 0 {
			page = n
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if n, err := parseInt(ps); err == nil && n > 0 && n <= 100 {
			pageSize = n
		}
	}

	filter := model.RemediationFilter{
		OrganizationID: orgID,
	}

	// Parse optional status filter
	if s := r.URL.Query().Get("status"); s != "" {
		filter.Statuses = []model.RemediationStatus{model.RemediationStatus(s)}
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Types = []model.RemediationType{model.RemediationType(t)}
	}
	if risk := r.URL.Query().Get("risk"); risk != "" {
		filter.Risks = []model.RemediationRisk{model.RemediationRisk(risk)}
	}

	pagination := model.Pagination{Page: page, PageSize: pageSize}

	actions, total, err := h.executor.List(ctx, filter, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list remediation actions")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"data":  actions,
		"total": total,
	})
}

// GetByID returns a single remediation action.
func (h *RemediationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action ID")
		return
	}

	action, err := h.executor.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "action not found")
		return
	}

	WriteJSON(w, http.StatusOK, action)
}

// Propose creates a new remediation action.
func (h *RemediationHandler) Propose(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	requestedBy := "system"
	if ok {
		requestedBy = email
	}

	var req model.RemediationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action, err := h.executor.ProposeAction(ctx, req, orgID, requestedBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, action)
}

// Approve approves a pending remediation action.
func (h *RemediationHandler) Approve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action ID")
		return
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	approvedBy := "system"
	if ok {
		approvedBy = email
	}

	if err := h.executor.Approve(ctx, id, approvedBy); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"status": "approved"})
}

// Reject rejects a pending remediation action.
func (h *RemediationHandler) Reject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action ID")
		return
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	rejectedBy := "system"
	if ok {
		rejectedBy = email
	}

	var req model.RemediationApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.executor.Reject(ctx, id, rejectedBy, req.Reason); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"status": "rejected"})
}

// Cancel cancels a pending or approved action.
func (h *RemediationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action ID")
		return
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	cancelledBy := "system"
	if ok {
		cancelledBy = email
	}

	if err := h.executor.Cancel(ctx, id, cancelledBy); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}

// Rollback rolls back a completed action.
func (h *RemediationHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action ID")
		return
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	rolledBackBy := "system"
	if ok {
		rolledBackBy = email
	}

	if err := h.executor.Rollback(ctx, id, rolledBackBy); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"status": "rolled_back"})
}

// GetSummary returns a summary of remediation actions.
func (h *RemediationHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	summary, err := h.executor.GetSummary(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get remediation summary")
		return
	}

	WriteJSON(w, http.StatusOK, summary)
}

// Auto-approval rule handlers

// ListRules returns all auto-approval rules.
func (h *RemediationHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	rules, err := h.executor.ListRules(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"data": rules})
}

// CreateRule creates a new auto-approval rule.
func (h *RemediationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	createdBy := "system"
	if ok {
		createdBy = email
	}

	var rule model.AutoApprovalRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule.BaseEntity = model.NewBaseEntity()
	rule.OrganizationID = orgID
	rule.CreatedBy = createdBy

	if err := h.executor.CreateRule(ctx, &rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	WriteJSON(w, http.StatusCreated, rule)
}

// UpdateRule updates an auto-approval rule.
func (h *RemediationHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	existing, err := h.executor.GetRuleByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}

	var update model.AutoApprovalRule
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing.Name = update.Name
	existing.Enabled = update.Enabled
	existing.Conditions = update.Conditions

	if err := h.executor.UpdateRule(ctx, existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	WriteJSON(w, http.StatusOK, existing)
}

// DeleteRule deletes an auto-approval rule.
func (h *RemediationHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	if err := h.executor.DeleteRule(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}
