package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// BudgetHandler handles budget API requests backed by real DB.
type BudgetHandler struct {
	repo repository.BudgetRepository
}

func NewBudgetHandler(repo repository.BudgetRepository) *BudgetHandler {
	return &BudgetHandler{repo: repo}
}

func (h *BudgetHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	budgets, err := h.repo.List(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list budgets")
		return
	}

	if budgets == nil {
		budgets = []*model.Budget{}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": budgets})
}

func (h *BudgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	var req model.BudgetCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "name and amount are required")
		return
	}

	budget := &model.Budget{
		BaseEntity:     model.NewBaseEntity(),
		OrganizationID: orgID,
		Name:           req.Name,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Period:         req.Period,
		Filters:        req.Filters,
		Thresholds:     req.Thresholds,
		Status:         model.BudgetStatusActive,
	}

	if budget.Currency == "" {
		budget.Currency = model.CurrencyUSD
	}
	if budget.Period == "" {
		budget.Period = model.BudgetPeriodMonthly
	}

	if err := h.repo.Create(ctx, budget); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create budget")
		return
	}

	WriteJSON(w, http.StatusCreated, budget)
}

func (h *BudgetHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	budget, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "budget not found")
		return
	}

	WriteJSON(w, http.StatusOK, budget)
}

func (h *BudgetHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	budget, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "budget not found")
		return
	}

	var req model.BudgetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		budget.Name = *req.Name
	}
	if req.Amount != nil {
		budget.Amount = *req.Amount
	}
	if req.Status != nil {
		budget.Status = *req.Status
	}
	if req.Thresholds != nil {
		budget.Thresholds = req.Thresholds
	}

	if err := h.repo.Update(ctx, budget); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update budget")
		return
	}

	WriteJSON(w, http.StatusOK, budget)
}

func (h *BudgetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete budget")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
