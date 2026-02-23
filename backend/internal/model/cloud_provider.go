package model

import (
	"time"

	"github.com/google/uuid"
)

// CloudProviderConfig represents a stored cloud provider configuration for an organization.
type CloudProviderConfig struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	OrganizationID uuid.UUID    `json:"organization_id" db:"organization_id"`
	ProviderType   CloudProvider `json:"provider_type" db:"provider_type"`
	Name           string        `json:"name" db:"name"`
	Credentials    []byte        `json:"-" db:"credentials"` // encrypted, never serialized
	Enabled        bool          `json:"enabled" db:"enabled"`
	Status         string        `json:"status" db:"status"`
	StatusMessage  string        `json:"status_message" db:"status_message"`
	LastSyncAt     *time.Time    `json:"last_sync_at,omitempty" db:"last_sync_at"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
}

// AWSCredentials holds AWS access credentials.
type AWSCredentials struct {
	AccessKeyID   string `json:"access_key_id"`
	SecretKey     string `json:"secret_key"`
	Region        string `json:"region"`
	AssumeRoleARN string `json:"assume_role_arn,omitempty"`
	ExternalID    string `json:"external_id,omitempty"`
}

// AzureCredentials holds Azure service principal credentials.
type AzureCredentials struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
}

// CreateProviderRequest is the API request to create a cloud provider.
type CreateProviderRequest struct {
	ProviderType string `json:"provider_type"`
	Name         string `json:"name"`
	Credentials  any    `json:"credentials"` // AWSCredentials or AzureCredentials as map
}

// UpdateProviderRequest is the API request to update a cloud provider.
type UpdateProviderRequest struct {
	Name        *string `json:"name,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Credentials any     `json:"credentials,omitempty"`
}
