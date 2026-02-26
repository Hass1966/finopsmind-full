package handler

import (
	"net/http"
	"strconv"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// ForecastDBHandler handles forecast requests backed by the database.
type ForecastDBHandler struct {
	forecastRepo repository.ForecastRepository
}

// NewForecastDBHandler creates a new ForecastDBHandler.
func NewForecastDBHandler(forecastRepo repository.ForecastRepository) *ForecastDBHandler {
	return &ForecastDBHandler{forecastRepo: forecastRepo}
}

// List returns forecasts for the authenticated user's organization.
func (h *ForecastDBHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	forecasts, total, err := h.forecastRepo.List(ctx, orgID, model.Pagination{Page: page, PageSize: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list forecasts")
		return
	}

	// If no forecasts in DB yet, return empty array (not null)
	if forecasts == nil {
		forecasts = []*model.Forecast{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  forecasts,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetLatest returns the most recent forecast for the authenticated user's organization.
func (h *ForecastDBHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	forecast, err := h.forecastRepo.GetLatest(ctx, orgID)
	if err != nil {
		// No forecast yet — return empty data
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": []interface{}{forecast},
	})
}
