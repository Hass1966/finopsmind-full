package model

import (
	"time"

	"github.com/google/uuid"
)

// PolicyType represents the type of policy.
type PolicyType string

const (
	PolicyTypeInstanceSize   PolicyType = "instance_size"
	PolicyTypeStorageType    PolicyType = "storage_type"
	PolicyTypeTagging        PolicyType = "tagging"
	PolicyTypeRegion         PolicyType = "region"
	PolicyTypeIdleResource   PolicyType = "idle_resource"
	PolicyTypeLifecycle      PolicyType = "lifecycle"
	PolicyTypeBudget         PolicyType = "budget"
	PolicyTypeCustom         PolicyType = "custom"
)

// EnforcementMode determines how violations are handled.
type EnforcementMode string

const (
	EnforcementModeAlertOnly    EnforcementMode = "alert_only"
	EnforcementModeSoftEnforce  EnforcementMode = "soft_enforce"
	EnforcementModeHardEnforce  EnforcementMode = "hard_enforce"
)

// ViolationStatus tracks the state of a policy violation.
type ViolationStatus string

const (
	ViolationStatusOpen         ViolationStatus = "open"
	ViolationStatusAcknowledged ViolationStatus = "acknowledged"
	ViolationStatusRemediated   ViolationStatus = "remediated"
	ViolationStatusExempted     ViolationStatus = "exempted"
)

// Policy defines a governance rule.
type Policy struct {
	BaseEntity
	OrganizationID  uuid.UUID         `json:"organization_id" db:"organization_id"`
	Name            string            `json:"name" db:"name"`
	Description     string            `json:"description" db:"description"`
	Type            PolicyType        `json:"type" db:"type"`
	EnforcementMode EnforcementMode   `json:"enforcement_mode" db:"enforcement_mode"`
	Enabled         bool              `json:"enabled" db:"enabled"`
	Conditions      PolicyConditions  `json:"conditions" db:"conditions"`
	Providers       []CloudProvider   `json:"providers,omitempty" db:"providers"`
	Environments    []string          `json:"environments,omitempty" db:"environments"`
	CreatedBy       string            `json:"created_by" db:"created_by"`
	LastEvaluatedAt *time.Time        `json:"last_evaluated_at,omitempty" db:"last_evaluated_at"`
	ViolationCount  int               `json:"violation_count" db:"violation_count"`
}

// PolicyConditions defines the rule conditions.
type PolicyConditions struct {
	// Instance size policies
	MaxInstanceTypes    []string `json:"max_instance_types,omitempty"`
	BlockedInstanceTypes []string `json:"blocked_instance_types,omitempty"`

	// Storage policies
	RequiredStorageTypes []string `json:"required_storage_types,omitempty"`
	MaxVolumeSize        int      `json:"max_volume_size_gb,omitempty"`

	// Tagging policies
	RequiredTags []string `json:"required_tags,omitempty"`
	TagPatterns  map[string]string `json:"tag_patterns,omitempty"`

	// Region policies
	AllowedRegions  []string `json:"allowed_regions,omitempty"`
	BlockedRegions  []string `json:"blocked_regions,omitempty"`

	// Idle resource policies
	MaxIdleDays     int     `json:"max_idle_days,omitempty"`
	CPUThreshold    float64 `json:"cpu_threshold,omitempty"`

	// Lifecycle policies
	MaxResourceAge  int  `json:"max_resource_age_days,omitempty"`
	RequireLifecycle bool `json:"require_lifecycle_policy,omitempty"`

	// Budget policies
	MaxMonthlyCost  float64 `json:"max_monthly_cost,omitempty"`
	MaxDailyCost    float64 `json:"max_daily_cost,omitempty"`
}

// PolicyViolation records a detected policy breach.
type PolicyViolation struct {
	BaseEntity
	OrganizationID uuid.UUID       `json:"organization_id" db:"organization_id"`
	PolicyID       uuid.UUID       `json:"policy_id" db:"policy_id"`
	PolicyName     string          `json:"policy_name" db:"policy_name"`
	Status         ViolationStatus `json:"status" db:"status"`
	Provider       CloudProvider   `json:"provider" db:"provider"`
	AccountID      string          `json:"account_id" db:"account_id"`
	Region         string          `json:"region" db:"region"`
	ResourceID     string          `json:"resource_id" db:"resource_id"`
	ResourceType   string          `json:"resource_type" db:"resource_type"`
	Description    string          `json:"description" db:"description"`
	Severity       Severity        `json:"severity" db:"severity"`
	Details        map[string]any  `json:"details,omitempty" db:"details"`
	DetectedAt     time.Time       `json:"detected_at" db:"detected_at"`
	RemediatedAt   *time.Time      `json:"remediated_at,omitempty" db:"remediated_at"`
	ExemptedBy     string          `json:"exempted_by,omitempty" db:"exempted_by"`
	ExemptReason   string          `json:"exempt_reason,omitempty" db:"exempt_reason"`
}

// PolicyFilter for querying policies.
type PolicyFilter struct {
	OrganizationID uuid.UUID
	Types          []PolicyType
	Enabled        *bool
}

// ViolationFilter for querying violations.
type ViolationFilter struct {
	OrganizationID uuid.UUID
	PolicyID       *uuid.UUID
	Statuses       []ViolationStatus
	Severities     []Severity
	DateRange      DateRange
}

// PolicySummary provides aggregated policy metrics.
type PolicySummary struct {
	TotalPolicies    int                        `json:"total_policies"`
	EnabledPolicies  int                        `json:"enabled_policies"`
	TotalViolations  int                        `json:"total_violations"`
	OpenViolations   int                        `json:"open_violations"`
	ByType           map[PolicyType]int         `json:"by_type"`
	BySeverity       map[Severity]int           `json:"by_severity"`
	ByEnforcement    map[EnforcementMode]int    `json:"by_enforcement"`
}

// PolicyCreateRequest for creating policies.
type PolicyCreateRequest struct {
	Name            string           `json:"name" validate:"required,min=1,max=255"`
	Description     string           `json:"description"`
	Type            PolicyType       `json:"type" validate:"required"`
	EnforcementMode EnforcementMode  `json:"enforcement_mode" validate:"required"`
	Conditions      PolicyConditions `json:"conditions"`
	Providers       []CloudProvider  `json:"providers,omitempty"`
	Environments    []string         `json:"environments,omitempty"`
}
