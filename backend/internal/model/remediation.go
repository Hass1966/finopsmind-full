package model

import (
	"time"

	"github.com/google/uuid"
)

// RemediationType represents types of automated remediation actions.
type RemediationType string

const (
	RemediationTypeResizeInstance      RemediationType = "resize_instance"
	RemediationTypeStopInstance        RemediationType = "stop_instance"
	RemediationTypeTerminateInstance   RemediationType = "terminate_instance"
	RemediationTypeDeleteVolume        RemediationType = "delete_volume"
	RemediationTypeUpgradeStorage      RemediationType = "upgrade_storage"
	RemediationTypeApplyLifecyclePolicy RemediationType = "apply_lifecycle_policy"
	RemediationTypeReleaseEIP          RemediationType = "release_eip"
	RemediationTypeDeleteSnapshot      RemediationType = "delete_snapshot"
	RemediationTypeRightsizing         RemediationType = "rightsizing"
	RemediationTypeCleanup             RemediationType = "cleanup"
)

// RemediationStatus represents the lifecycle status of a remediation action.
type RemediationStatus string

const (
	RemediationStatusPendingApproval RemediationStatus = "pending_approval"
	RemediationStatusApproved        RemediationStatus = "approved"
	RemediationStatusRejected        RemediationStatus = "rejected"
	RemediationStatusExecuting       RemediationStatus = "executing"
	RemediationStatusCompleted       RemediationStatus = "completed"
	RemediationStatusFailed          RemediationStatus = "failed"
	RemediationStatusRolledBack      RemediationStatus = "rolled_back"
	RemediationStatusCancelled       RemediationStatus = "cancelled"
)

// RemediationRisk represents the risk level of a remediation action.
type RemediationRisk string

const (
	RemediationRiskLow      RemediationRisk = "low"
	RemediationRiskMedium   RemediationRisk = "medium"
	RemediationRiskHigh     RemediationRisk = "high"
	RemediationRiskCritical RemediationRisk = "critical"
)

// RemediationAction represents an automated remediation action with approval workflow.
type RemediationAction struct {
	BaseEntity
	OrganizationID  uuid.UUID         `json:"organization_id" db:"organization_id"`
	RecommendationID *uuid.UUID       `json:"recommendation_id,omitempty" db:"recommendation_id"`
	Type            RemediationType   `json:"type" db:"type"`
	Status          RemediationStatus `json:"status" db:"status"`
	Provider        CloudProvider     `json:"provider" db:"provider"`
	AccountID       string            `json:"account_id,omitempty" db:"account_id"`
	Region          string            `json:"region,omitempty" db:"region"`
	ResourceID      string            `json:"resource_id,omitempty" db:"resource_id"`
	ResourceType    string            `json:"resource_type,omitempty" db:"resource_type"`
	Description     string            `json:"description" db:"description"`
	CurrentState    map[string]any    `json:"current_state,omitempty" db:"current_state"`
	DesiredState    map[string]any    `json:"desired_state,omitempty" db:"desired_state"`
	EstimatedSavings float64          `json:"estimated_savings" db:"estimated_savings"`
	Currency        Currency          `json:"currency" db:"currency"`
	Risk            RemediationRisk   `json:"risk" db:"risk"`
	AutoApproved    bool              `json:"auto_approved" db:"auto_approved"`
	ApprovalRule    string            `json:"approval_rule,omitempty" db:"approval_rule"`
	RequestedBy     string            `json:"requested_by,omitempty" db:"requested_by"`
	ApprovedBy      string            `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt      *time.Time        `json:"approved_at,omitempty" db:"approved_at"`
	ExecutedAt      *time.Time        `json:"executed_at,omitempty" db:"executed_at"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty" db:"completed_at"`
	RolledBackAt    *time.Time        `json:"rolled_back_at,omitempty" db:"rolled_back_at"`
	FailureReason   string            `json:"failure_reason,omitempty" db:"failure_reason"`
	RollbackData    map[string]any    `json:"rollback_data,omitempty" db:"rollback_data"`
	AuditLog        []AuditEntry      `json:"audit_log,omitempty" db:"audit_log"`
}

// AuditEntry represents a single entry in a remediation action's audit trail.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Details   string    `json:"details,omitempty"`
}

// AddAuditEntry appends a new audit entry to the remediation action's audit log.
func (a *RemediationAction) AddAuditEntry(actor, action, details string) {
	a.AuditLog = append(a.AuditLog, AuditEntry{
		Timestamp: time.Now().UTC(),
		Actor:     actor,
		Action:    action,
		Details:   details,
	})
}

// RemediationFilter defines filtering options for remediation queries.
type RemediationFilter struct {
	OrganizationID uuid.UUID
	Types          []RemediationType
	Statuses       []RemediationStatus
	Risks          []RemediationRisk
	Providers      []CloudProvider
	DateRange      DateRange
}

// RemediationSummary provides a summary of remediation actions.
type RemediationSummary struct {
	TotalCount          int                        `json:"total_count"`
	PendingCount        int                        `json:"pending_count"`
	ApprovedCount       int                        `json:"approved_count"`
	CompletedCount      int                        `json:"completed_count"`
	FailedCount         int                        `json:"failed_count"`
	TotalSavingsRealized float64                   `json:"total_savings_realized"`
	TotalSavingsPending float64                    `json:"total_savings_pending"`
	ByType              map[RemediationType]int    `json:"by_type"`
	ByStatus            map[RemediationStatus]int  `json:"by_status"`
	ByRisk              map[RemediationRisk]int    `json:"by_risk"`
	Currency            Currency                   `json:"currency"`
}

// RemediationCreateRequest represents a request to create a remediation action.
type RemediationCreateRequest struct {
	Type             RemediationType `json:"type"`
	Provider         CloudProvider   `json:"provider"`
	AccountID        string          `json:"account_id"`
	Region           string          `json:"region"`
	ResourceID       string          `json:"resource_id"`
	ResourceType     string          `json:"resource_type"`
	Description      string          `json:"description"`
	CurrentState     map[string]any  `json:"current_state,omitempty"`
	DesiredState     map[string]any  `json:"desired_state,omitempty"`
	EstimatedSavings float64         `json:"estimated_savings"`
	Currency         Currency        `json:"currency"`
	Risk             RemediationRisk `json:"risk"`
	RollbackData     map[string]any  `json:"rollback_data,omitempty"`
}

// RemediationApprovalRequest represents a request to approve or reject a remediation action.
type RemediationApprovalRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// AutoApprovalRule defines conditions under which remediation actions are automatically approved.
type AutoApprovalRule struct {
	BaseEntity
	OrganizationID uuid.UUID              `json:"organization_id" db:"organization_id"`
	Name           string                 `json:"name" db:"name"`
	Enabled        bool                   `json:"enabled" db:"enabled"`
	Conditions     AutoApprovalConditions `json:"conditions" db:"conditions"`
	CreatedBy      string                 `json:"created_by,omitempty" db:"created_by"`
}

// AutoApprovalConditions defines the conditions for automatic approval of remediation actions.
type AutoApprovalConditions struct {
	MaxSavings          float64           `json:"max_savings"`
	AllowedTypes        []RemediationType `json:"allowed_types,omitempty"`
	AllowedRisks        []RemediationRisk `json:"allowed_risks,omitempty"`
	AllowedEnvironments []string          `json:"allowed_environments,omitempty"`
}
