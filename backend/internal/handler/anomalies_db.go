package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/mlclient"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// AnomalyDBHandler handles anomaly API requests backed by real DB.
type AnomalyDBHandler struct {
	repo     repository.AnomalyRepository
	mlClient *mlclient.Client
}

func NewAnomalyDBHandler(repo repository.AnomalyRepository, mlClient *mlclient.Client) *AnomalyDBHandler {
	return &AnomalyDBHandler{repo: repo, mlClient: mlClient}
}

func (h *AnomalyDBHandler) List(w http.ResponseWriter, r *http.Request) {
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

	filter := model.AnomalyFilter{
		OrganizationID: orgID,
	}
	pagination := model.Pagination{Page: page, PageSize: pageSize}

	anomalies, total, err := h.repo.List(ctx, filter, pagination)
	if err != nil {
		// Fall back to mock
		GetAnomaliesMock(w, r)
		return
	}

	if total == 0 {
		// No data in DB, return mock
		GetAnomaliesMock(w, r)
		return
	}

	// Count stats
	openCount := 0
	criticalCount := 0
	resolvedCount := 0
	for _, a := range anomalies {
		switch a.Status {
		case model.StatusOpen:
			openCount++
		case model.StatusResolved:
			resolvedCount++
		}
		if a.Severity == model.SeverityCritical {
			criticalCount++
		}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data":     anomalies,
		"total":    total,
		"open":     openCount,
		"critical": criticalCount,
		"resolved": resolvedCount,
	})
}

func (h *AnomalyDBHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	summary, err := h.repo.GetSummary(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get anomaly summary")
		return
	}

	WriteJSON(w, http.StatusOK, summary)
}

func (h *AnomalyDBHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid anomaly ID")
		return
	}

	anomaly, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "anomaly not found")
		return
	}

	var req model.AnomalyUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != nil {
		anomaly.Status = *req.Status
	}
	if req.RootCause != nil {
		anomaly.RootCause = *req.RootCause
	}
	if req.Notes != nil {
		anomaly.Notes = *req.Notes
	}

	if err := h.repo.Update(ctx, anomaly); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update anomaly")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "updated"})
}

func (h *AnomalyDBHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid anomaly ID")
		return
	}

	_, email, ok := auth.GetUserFromContext(ctx)
	acknowledgedBy := "system"
	if ok {
		acknowledgedBy = email
	}

	if err := h.repo.Acknowledge(ctx, id, acknowledgedBy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to acknowledge anomaly")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "acknowledged"})
}

func (h *AnomalyDBHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid anomaly ID")
		return
	}

	if err := h.repo.Resolve(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve anomaly")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "resolved"})
}
