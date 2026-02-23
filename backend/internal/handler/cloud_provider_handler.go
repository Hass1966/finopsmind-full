package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/provider"
	"github.com/finopsmind/backend/internal/repository"
)

// CloudProviderHandler handles cloud provider CRUD and operations.
type CloudProviderHandler struct {
	repo          repository.CloudProviderRepository
	costRepo      repository.CostRepository
	encryptionKey string
	logger        *slog.Logger
}

// NewCloudProviderHandler creates a new CloudProviderHandler.
func NewCloudProviderHandler(repo repository.CloudProviderRepository, costRepo repository.CostRepository, encryptionKey string, logger *slog.Logger) *CloudProviderHandler {
	return &CloudProviderHandler{
		repo:          repo,
		costRepo:      costRepo,
		encryptionKey: encryptionKey,
		logger:        logger,
	}
}

// Create handles POST /providers — stores encrypted credentials.
func (h *CloudProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	claims := auth.GetClaimsFromContext(ctx)
	if claims == nil || claims.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req model.CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ProviderType != "aws" && req.ProviderType != "azure" {
		writeError(w, http.StatusBadRequest, "provider_type must be 'aws' or 'azure'")
		return
	}

	// Parse and validate credentials based on provider type
	credsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid credentials format")
		return
	}

	switch req.ProviderType {
	case "aws":
		var creds model.AWSCredentials
		if err := json.Unmarshal(credsJSON, &creds); err != nil {
			writeError(w, http.StatusBadRequest, "invalid AWS credentials")
			return
		}
		if creds.AccessKeyID == "" || creds.SecretKey == "" || creds.Region == "" {
			writeError(w, http.StatusBadRequest, "access_key_id, secret_key, and region are required for AWS")
			return
		}
	case "azure":
		var creds model.AzureCredentials
		if err := json.Unmarshal(credsJSON, &creds); err != nil {
			writeError(w, http.StatusBadRequest, "invalid Azure credentials")
			return
		}
		if creds.TenantID == "" || creds.ClientID == "" || creds.ClientSecret == "" || creds.SubscriptionID == "" {
			writeError(w, http.StatusBadRequest, "tenant_id, client_id, client_secret, and subscription_id are required for Azure")
			return
		}
	}

	// Encrypt credentials
	encrypted, err := provider.EncryptCredentials(req.Credentials, h.encryptionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt credentials")
		return
	}

	now := time.Now().UTC()
	cfg := &model.CloudProviderConfig{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ProviderType:   model.CloudProvider(req.ProviderType),
		Name:           req.Name,
		Credentials:    encrypted,
		Enabled:        true,
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if cfg.Name == "" {
		cfg.Name = req.ProviderType
	}

	if err := h.repo.Create(ctx, cfg); err != nil {
		writeError(w, http.StatusConflict, "provider already exists for this organization")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"id":            cfg.ID,
		"provider_type": cfg.ProviderType,
		"name":          cfg.Name,
		"status":        cfg.Status,
		"enabled":       cfg.Enabled,
		"created_at":    cfg.CreatedAt,
	})
}

// List handles GET /providers — returns providers with masked credentials.
func (h *CloudProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providers, err := h.repo.GetByOrgID(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list providers")
		return
	}

	results := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		results = append(results, map[string]interface{}{
			"id":             p.ID,
			"name":           p.Name,
			"provider_type":  p.ProviderType,
			"enabled":        p.Enabled,
			"status":         p.Status,
			"status_message": p.StatusMessage,
			"last_sync_at":   p.LastSyncAt,
			"healthy":        p.Status == "connected",
			"created_at":     p.CreatedAt,
			"updated_at":     p.UpdatedAt,
		})
	}

	WriteJSON(w, http.StatusOK, results)
}

// Update handles PUT /providers/{id} — updates name, enabled, or re-encrypts credentials.
func (h *CloudProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	claims := auth.GetClaimsFromContext(ctx)
	if claims == nil || claims.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}

	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	if existing.OrganizationID != orgID {
		writeError(w, http.StatusForbidden, "not your provider")
		return
	}

	var req model.UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Credentials != nil {
		encrypted, err := provider.EncryptCredentials(req.Credentials, h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt credentials")
			return
		}
		existing.Credentials = encrypted
		existing.Status = "pending"
	}

	if err := h.repo.Update(ctx, existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update provider")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "updated"})
}

// Delete handles DELETE /providers/{id}.
func (h *CloudProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	claims := auth.GetClaimsFromContext(ctx)
	if claims == nil || claims.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}

	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	if existing.OrganizationID != orgID {
		writeError(w, http.StatusForbidden, "not your provider")
		return
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete provider")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted"})
}

// TestConnection handles POST /providers/{id}/test — decrypts creds, creates temp provider, checks health.
func (h *CloudProviderHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}

	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	if existing.OrganizationID != orgID {
		writeError(w, http.StatusForbidden, "not your provider")
		return
	}

	// Instantiate temp provider from encrypted creds
	prov, err := provider.NewProviderFromEncryptedCreds(string(existing.ProviderType), existing.Credentials, h.encryptionKey, h.logger)
	if err != nil {
		h.repo.UpdateStatus(ctx, id, "error", err.Error())
		writeError(w, http.StatusBadRequest, "failed to initialize provider: "+err.Error())
		return
	}
	defer prov.Close()

	health := prov.Health(ctx)

	status := "connected"
	if !health.Healthy {
		status = "error"
	}
	h.repo.UpdateStatus(ctx, id, status, health.Message)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"healthy": health.Healthy,
		"message": health.Message,
		"status":  status,
	})
}

// TriggerSync handles POST /providers/{id}/sync — fetch 30 days of costs and persist.
func (h *CloudProviderHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}

	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	if existing.OrganizationID != orgID {
		writeError(w, http.StatusForbidden, "not your provider")
		return
	}

	prov, err := provider.NewProviderFromEncryptedCreds(string(existing.ProviderType), existing.Credentials, h.encryptionKey, h.logger)
	if err != nil {
		h.repo.UpdateStatus(ctx, id, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "failed to initialize provider")
		return
	}
	defer prov.Close()

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	costResp, err := prov.GetCosts(ctx, provider.CostRequest{
		StartDate:   startDate,
		EndDate:     endDate,
		Granularity: "daily",
		GroupBy:     []string{"service"},
	})
	if err != nil {
		h.repo.UpdateStatus(ctx, id, "error", "sync failed: "+err.Error())
		writeError(w, http.StatusInternalServerError, "failed to fetch costs: "+err.Error())
		return
	}

	// Convert to CostRecords and persist
	var records []*model.CostRecord
	for _, item := range costResp.Costs {
		records = append(records, &model.CostRecord{
			BaseEntity:     model.NewBaseEntity(),
			OrganizationID: orgID,
			Date:           item.Date,
			Amount:         item.Amount,
			Currency:       model.CurrencyUSD,
			Provider:       model.CloudProvider(existing.ProviderType),
			Service:        item.Service,
			AccountID:      item.AccountID,
			Region:         item.Region,
		})
	}

	if len(records) > 0 {
		if err := h.costRepo.CreateBatch(ctx, records); err != nil {
			h.repo.UpdateStatus(ctx, id, "error", "failed to persist costs: "+err.Error())
			writeError(w, http.StatusInternalServerError, "failed to persist cost data")
			return
		}
	}

	h.repo.UpdateStatus(ctx, id, "connected", "sync completed successfully")
	h.repo.UpdateLastSync(ctx, id)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "synced",
		"records": len(records),
		"total":   costResp.TotalAmount,
	})
}
