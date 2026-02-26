package remediation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/finopsmind/backend/internal/model"
)

// AzureExecutor performs real Azure API remediation actions.
type AzureExecutor struct {
	logger *slog.Logger
}

// NewAzureExecutor creates a new AzureExecutor.
func NewAzureExecutor(logger *slog.Logger) *AzureExecutor {
	return &AzureExecutor{logger: logger}
}

// Execute dispatches to the appropriate Azure API call.
// Azure remediation support is limited compared to AWS. Actions that aren't
// directly supported via Azure SDK calls fall back to an error instructing
// the user to use Terraform/scripts.
func (e *AzureExecutor) Execute(ctx context.Context, action *model.RemediationAction, creds *AzureCreds) error {
	switch action.Type {
	case model.RemediationTypeStopInstance:
		return e.stopVM(ctx, action, creds)
	case model.RemediationTypeResizeInstance:
		return e.resizeVM(ctx, action, creds)
	default:
		return fmt.Errorf("Azure remediation for %s not yet supported — use the generated Terraform script instead", action.Type)
	}
}

// Rollback dispatches to the appropriate Azure rollback.
func (e *AzureExecutor) Rollback(ctx context.Context, action *model.RemediationAction, creds *AzureCreds) error {
	switch action.Type {
	case model.RemediationTypeStopInstance:
		return e.startVM(ctx, action, creds)
	default:
		return fmt.Errorf("Azure rollback for %s not supported", action.Type)
	}
}

func (e *AzureExecutor) stopVM(ctx context.Context, action *model.RemediationAction, creds *AzureCreds) error {
	// Azure VM stop requires the Azure Compute SDK.
	// For now, log the intent. Full Azure SDK integration can be added when needed.
	e.logger.Info("Azure VM stop requested",
		"resource_id", action.ResourceID,
		"subscription", creds.SubscriptionID,
	)
	// The Azure provider uses REST API calls directly. For production,
	// this would use the Azure SDK for Go (azure-sdk-for-go/sdk/resourcemanager/compute).
	return fmt.Errorf("Azure VM stop: use Azure portal or generated script — direct API support coming soon")
}

func (e *AzureExecutor) startVM(ctx context.Context, action *model.RemediationAction, creds *AzureCreds) error {
	e.logger.Info("Azure VM start requested (rollback)",
		"resource_id", action.ResourceID,
	)
	return fmt.Errorf("Azure VM start: use Azure portal or generated script")
}

func (e *AzureExecutor) resizeVM(ctx context.Context, action *model.RemediationAction, creds *AzureCreds) error {
	e.logger.Info("Azure VM resize requested",
		"resource_id", action.ResourceID,
	)
	return fmt.Errorf("Azure VM resize: use Azure portal or generated script — direct API support coming soon")
}
