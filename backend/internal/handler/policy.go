package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
)

// PolicyHandler handles policy management API requests.
// For now, policies are stored in-memory with mock data.
// TODO: Wire to repository when policy tables are created.
type PolicyHandler struct{}

func NewPolicyHandler() *PolicyHandler {
	return &PolicyHandler{}
}

// List returns all policies.
func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	// Return pre-built policy library
	policies := defaultPolicies(orgID)

	WriteJSON(w, http.StatusOK, map[string]any{
		"data":  policies,
		"total": len(policies),
	})
}

// GetByID returns a single policy.
func (h *PolicyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid policy ID")
		return
	}

	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	for _, p := range defaultPolicies(orgID) {
		if p.ID == id {
			WriteJSON(w, http.StatusOK, p)
			return
		}
	}

	writeError(w, http.StatusNotFound, "policy not found")
}

// Create creates a new policy.
func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	_, email, ok := auth.GetUserFromContext(r.Context())
	createdBy := "system"
	if ok {
		createdBy = email
	}

	var req model.PolicyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	policy := model.Policy{
		BaseEntity:      model.NewBaseEntity(),
		OrganizationID:  orgID,
		Name:            req.Name,
		Description:     req.Description,
		Type:            req.Type,
		EnforcementMode: req.EnforcementMode,
		Enabled:         true,
		Conditions:      req.Conditions,
		Providers:       req.Providers,
		Environments:    req.Environments,
		CreatedBy:       createdBy,
	}

	WriteJSON(w, http.StatusCreated, policy)
}

// GetViolations returns policy violations.
func (h *PolicyHandler) GetViolations(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	violations := defaultViolations(orgID)

	WriteJSON(w, http.StatusOK, map[string]any{
		"data":  violations,
		"total": len(violations),
	})
}

// GetSummary returns policy compliance summary.
func (h *PolicyHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	summary := model.PolicySummary{
		TotalPolicies:   8,
		EnabledPolicies: 6,
		TotalViolations: 12,
		OpenViolations:  7,
		ByType: map[model.PolicyType]int{
			model.PolicyTypeInstanceSize: 2,
			model.PolicyTypeTagging:      3,
			model.PolicyTypeStorageType:  1,
			model.PolicyTypeIdleResource: 1,
		},
		BySeverity: map[model.Severity]int{
			model.SeverityCritical: 1,
			model.SeverityHigh:     3,
			model.SeverityMedium:   5,
			model.SeverityLow:      3,
		},
		ByEnforcement: map[model.EnforcementMode]int{
			model.EnforcementModeAlertOnly:   4,
			model.EnforcementModeSoftEnforce: 3,
			model.EnforcementModeHardEnforce: 1,
		},
	}

	WriteJSON(w, http.StatusOK, summary)
}

// Pre-built policy library
func defaultPolicies(orgID uuid.UUID) []model.Policy {
	return []model.Policy{
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001")},
			OrganizationID: orgID, Name: "No oversized instances in non-prod",
			Description: "Instances larger than m5.xlarge are not allowed in dev/staging environments",
			Type: model.PolicyTypeInstanceSize, EnforcementMode: model.EnforcementModeSoftEnforce,
			Enabled: true, ViolationCount: 3,
			Conditions: model.PolicyConditions{BlockedInstanceTypes: []string{"m5.2xlarge", "m5.4xlarge", "m5.8xlarge", "c5.2xlarge", "c5.4xlarge", "r5.2xlarge"}},
			Environments: []string{"dev", "staging"},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000002")},
			OrganizationID: orgID, Name: "Required resource tags",
			Description: "All resources must have Environment, Team, and CostCenter tags",
			Type: model.PolicyTypeTagging, EnforcementMode: model.EnforcementModeAlertOnly,
			Enabled: true, ViolationCount: 8,
			Conditions: model.PolicyConditions{RequiredTags: []string{"Environment", "Team", "CostCenter"}},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000003")},
			OrganizationID: orgID, Name: "Use gp3 storage",
			Description: "All new EBS volumes must use gp3 instead of gp2",
			Type: model.PolicyTypeStorageType, EnforcementMode: model.EnforcementModeHardEnforce,
			Enabled: true, ViolationCount: 5,
			Conditions: model.PolicyConditions{RequiredStorageTypes: []string{"gp3", "io2"}},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000004")},
			OrganizationID: orgID, Name: "No resources in restricted regions",
			Description: "Resources should only be deployed in approved regions",
			Type: model.PolicyTypeRegion, EnforcementMode: model.EnforcementModeAlertOnly,
			Enabled: true, ViolationCount: 1,
			Conditions: model.PolicyConditions{AllowedRegions: []string{"us-east-1", "us-west-2", "eu-west-1", "eu-west-2"}},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000005")},
			OrganizationID: orgID, Name: "Terminate idle EC2 after 7 days",
			Description: "EC2 instances with <5% avg CPU for 7 days should be stopped",
			Type: model.PolicyTypeIdleResource, EnforcementMode: model.EnforcementModeSoftEnforce,
			Enabled: true, ViolationCount: 2,
			Conditions: model.PolicyConditions{MaxIdleDays: 7, CPUThreshold: 5.0},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000006")},
			OrganizationID: orgID, Name: "S3 lifecycle policies required",
			Description: "All S3 buckets must have lifecycle policies configured",
			Type: model.PolicyTypeLifecycle, EnforcementMode: model.EnforcementModeAlertOnly,
			Enabled: true, ViolationCount: 4,
			Conditions: model.PolicyConditions{RequireLifecycle: true},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000007")},
			OrganizationID: orgID, Name: "Dev environment budget cap",
			Description: "Development environment should not exceed $2,000/month",
			Type: model.PolicyTypeBudget, EnforcementMode: model.EnforcementModeSoftEnforce,
			Enabled: false, ViolationCount: 0,
			Conditions: model.PolicyConditions{MaxMonthlyCost: 2000},
			Environments: []string{"dev"},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("10000000-0000-0000-0000-000000000008")},
			OrganizationID: orgID, Name: "Max EBS volume size 500GB",
			Description: "No single EBS volume should exceed 500GB without approval",
			Type: model.PolicyTypeStorageType, EnforcementMode: model.EnforcementModeAlertOnly,
			Enabled: false, ViolationCount: 0,
			Conditions: model.PolicyConditions{MaxVolumeSize: 500},
		},
	}
}

func defaultViolations(orgID uuid.UUID) []model.PolicyViolation {
	return []model.PolicyViolation{
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("20000000-0000-0000-0000-000000000001")},
			OrganizationID: orgID,
			PolicyID: uuid.MustParse("10000000-0000-0000-0000-000000000001"),
			PolicyName: "No oversized instances in non-prod",
			Status: model.ViolationStatusOpen, Provider: model.CloudProviderAWS,
			Region: "us-east-1", ResourceID: "i-0abc123def456",
			ResourceType: "EC2 Instance", Severity: model.SeverityHigh,
			Description: "m5.2xlarge instance running in dev environment",
			Details: map[string]any{"instance_type": "m5.2xlarge", "environment": "dev", "monthly_cost": 280.0},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002")},
			OrganizationID: orgID,
			PolicyID: uuid.MustParse("10000000-0000-0000-0000-000000000002"),
			PolicyName: "Required resource tags",
			Status: model.ViolationStatusOpen, Provider: model.CloudProviderAWS,
			Region: "us-west-2", ResourceID: "i-0xyz789ghi012",
			ResourceType: "EC2 Instance", Severity: model.SeverityMedium,
			Description: "Missing required tags: Team, CostCenter",
			Details: map[string]any{"missing_tags": []string{"Team", "CostCenter"}, "existing_tags": map[string]string{"Environment": "prod"}},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("20000000-0000-0000-0000-000000000003")},
			OrganizationID: orgID,
			PolicyID: uuid.MustParse("10000000-0000-0000-0000-000000000003"),
			PolicyName: "Use gp3 storage",
			Status: model.ViolationStatusOpen, Provider: model.CloudProviderAWS,
			Region: "us-east-1", ResourceID: "vol-0abc123",
			ResourceType: "EBS Volume", Severity: model.SeverityLow,
			Description: "EBS volume using gp2 instead of gp3",
			Details: map[string]any{"current_type": "gp2", "recommended_type": "gp3", "monthly_savings": 8.50},
		},
		{
			BaseEntity: model.BaseEntity{ID: uuid.MustParse("20000000-0000-0000-0000-000000000004")},
			OrganizationID: orgID,
			PolicyID: uuid.MustParse("10000000-0000-0000-0000-000000000005"),
			PolicyName: "Terminate idle EC2 after 7 days",
			Status: model.ViolationStatusOpen, Provider: model.CloudProviderAWS,
			Region: "eu-west-1", ResourceID: "i-0idle456",
			ResourceType: "EC2 Instance", Severity: model.SeverityCritical,
			Description: "EC2 instance idle for 14 days (avg CPU: 1.2%)",
			Details: map[string]any{"idle_days": 14, "avg_cpu": 1.2, "monthly_cost": 156.0},
		},
	}
}
