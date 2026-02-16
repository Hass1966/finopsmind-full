package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/finopsmind/backend/internal/terraform"
)

type TerraformHandler struct {
	generator *terraform.Generator
	validator *terraform.Validator
}

func NewTerraformHandler() (*TerraformHandler, error) {
	gen, err := terraform.NewGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}
	return &TerraformHandler{generator: gen, validator: terraform.NewValidator()}, nil
}

func (h *TerraformHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/recommendations/{id}/terraform", h.GetTerraform).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/recommendations/{id}/terraform/download", h.DownloadTerraform).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/terraform/supported-types", h.GetSupportedTypes).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/terraform/validate", h.ValidateHCL).Methods("POST", "OPTIONS")
}

type TerraformResponse struct {
	Success    bool                        `json:"success"`
	HCL        string                      `json:"hcl,omitempty"`
	Formatted  string                      `json:"formatted,omitempty"`
	Validation *terraform.ValidationResult `json:"validation,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Metadata   TerraformMetadata           `json:"metadata"`
}

type TerraformMetadata struct {
	RecommendationType string   `json:"recommendation_type"`
	TemplateName       string   `json:"template_name"`
	ResourceID         string   `json:"resource_id"`
	ImportCommand      string   `json:"import_command,omitempty"`
	ApplyWarnings      []string `json:"apply_warnings,omitempty"`
}

func (h *TerraformHandler) GetTerraform(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }

	vars := mux.Vars(r)
	recommendationID := vars["id"]
	if recommendationID == "" { h.sendError(w, http.StatusBadRequest, "recommendation ID required"); return }

	rec := h.getMockRecommendation(recommendationID)
	if rec == nil { h.sendError(w, http.StatusNotFound, "recommendation not found"); return }

	if !h.generator.IsSupported(rec.Type) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("unsupported type: %s", rec.Type)); return
	}

	hcl, err := h.generator.Generate(rec)
	if err != nil { h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("generation failed: %v", err)); return }

	validation := h.validator.ValidateResourceBlock(hcl)

	response := TerraformResponse{
		Success: true, HCL: hcl, Formatted: validation.Formatted, Validation: validation,
		Metadata: TerraformMetadata{
			RecommendationType: rec.Type,
			TemplateName:       strings.ToLower(rec.Type),
			ResourceID:         rec.ResourceID,
			ImportCommand:      h.getImportCommand(rec),
			ApplyWarnings:      h.getApplyWarnings(rec),
		},
	}
	h.sendJSON(w, http.StatusOK, response)
}

func (h *TerraformHandler) DownloadTerraform(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }

	vars := mux.Vars(r)
	rec := h.getMockRecommendation(vars["id"])
	if rec == nil { h.sendError(w, http.StatusNotFound, "not found"); return }

	hcl, _ := h.generator.Generate(rec)
	formatted, err := h.validator.ValidateAndFormat(hcl)
	if err != nil { formatted = hcl }

	filename := fmt.Sprintf("%s_%s.tf", rec.Type, rec.ResourceName)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Write([]byte(formatted))
}

func (h *TerraformHandler) GetSupportedTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }
	types := h.generator.GetSupportedTypes()
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"success": true, "supported_types": types, "count": len(types)})
}

func (h *TerraformHandler) ValidateHCL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }

	var req struct { HCL string `json:"hcl"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { h.sendError(w, http.StatusBadRequest, "invalid body"); return }
	if req.HCL == "" { h.sendError(w, http.StatusBadRequest, "HCL required"); return }

	validation := h.validator.ValidateResourceBlock(req.HCL)
	placeholders := h.validator.CheckPlaceholders(req.HCL)
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"success": true, "validation": validation, "placeholders": placeholders})
}

func (h *TerraformHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TerraformHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, TerraformResponse{Success: false, Error: message})
}

func (h *TerraformHandler) getImportCommand(rec *terraform.Recommendation) string {
	switch {
	case strings.HasPrefix(rec.Type, "ec2"):
		return fmt.Sprintf("terraform import aws_instance.%s %s", rec.ResourceName, rec.ResourceID)
	case strings.HasPrefix(rec.Type, "ebs"):
		return fmt.Sprintf("terraform import aws_ebs_volume.%s %s", rec.ResourceName, rec.ResourceID)
	case strings.HasPrefix(rec.Type, "rds"):
		return fmt.Sprintf("terraform import aws_db_instance.%s %s", rec.ResourceName, rec.ResourceID)
	default:
		return ""
	}
}

func (h *TerraformHandler) getApplyWarnings(rec *terraform.Recommendation) []string {
	warnings := []string{}
	switch rec.Type {
	case "ec2_rightsize":
		warnings = append(warnings, "Instance will be stopped and started")
	case "ec2_terminate", "ebs_delete":
		warnings = append(warnings, "DESTRUCTIVE: Resource will be permanently deleted")
	case "rds_rightsize":
		warnings = append(warnings, "Brief downtime during modification")
	}
	return warnings
}

func (h *TerraformHandler) getMockRecommendation(id string) *terraform.Recommendation {
	mocks := map[string]*terraform.Recommendation{
		"rec-001": {
			ID: "rec-001", Type: "ec2_rightsize", ResourceID: "i-0123456789abcdef0",
			ResourceType: "EC2", ResourceName: "web_server_1", Region: "us-east-1",
			AccountID: "123456789012", CurrentValue: "m5.xlarge", NewValue: "m5.large",
			CurrentCost: 140.16, ProjectedCost: 70.08, MonthlySavings: 70.08, AnnualSavings: 840.96,
			Confidence: 92.5, Impact: "medium",
			Metadata: map[string]interface{}{"instance_type": "m5.xlarge", "new_instance_type": "m5.large"},
		},
		"rec-002": {
			ID: "rec-002", Type: "ebs_gp3_upgrade", ResourceID: "vol-0123456789abcdef0",
			ResourceType: "EBS", ResourceName: "data_volume_1", Region: "us-east-1",
			CurrentValue: "gp2", NewValue: "gp3", MonthlySavings: 20.00, AnnualSavings: 240.00, Confidence: 98.0,
			Metadata: map[string]interface{}{"volume_type": "gp2", "volume_size": float64(500), "iops": float64(3000), "throughput": float64(125)},
		},
		"rec-003": {
			ID: "rec-003", Type: "s3_lifecycle", ResourceID: "my-data-bucket",
			ResourceType: "S3", ResourceName: "data_bucket", Region: "us-east-1",
			MonthlySavings: 300.00, AnnualSavings: 3600.00, Confidence: 85.0,
			Metadata: map[string]interface{}{"bucket_name": "my-data-bucket", "transition_days": float64(30)},
		},
	}
	return mocks[id]
}
