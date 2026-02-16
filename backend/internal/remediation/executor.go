package remediation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// Executor handles remediation action execution and auto-approval evaluation.
type Executor struct {
	repo   repository.RemediationRepository
	logger *slog.Logger
	mu     sync.Mutex
}

func NewExecutor(repo repository.RemediationRepository, logger *slog.Logger) *Executor {
	return &Executor{repo: repo, logger: logger}
}

// ProposeAction creates a new remediation action and checks auto-approval rules.
func (e *Executor) ProposeAction(ctx context.Context, req model.RemediationCreateRequest, orgID uuid.UUID, requestedBy string) (*model.RemediationAction, error) {
	action := &model.RemediationAction{
		BaseEntity:       model.NewBaseEntity(),
		OrganizationID:   orgID,
		Type:             req.Type,
		Status:           model.RemediationStatusPendingApproval,
		Provider:         req.Provider,
		AccountID:        req.AccountID,
		Region:           req.Region,
		ResourceID:       req.ResourceID,
		ResourceType:     req.ResourceType,
		Description:      req.Description,
		CurrentState:     req.CurrentState,
		DesiredState:     req.DesiredState,
		EstimatedSavings: req.EstimatedSavings,
		Currency:         req.Currency,
		Risk:             req.Risk,
		RollbackData:     req.RollbackData,
		RequestedBy:      requestedBy,
		AuditLog:         []model.AuditEntry{},
	}

	action.AddAuditEntry(requestedBy, "proposed", fmt.Sprintf("Remediation proposed: %s on %s", req.Type, req.ResourceID))

	// Check auto-approval rules
	rules, err := e.repo.GetActiveRules(ctx, orgID)
	if err != nil {
		e.logger.Warn("failed to fetch auto-approval rules", "error", err)
	} else {
		for _, rule := range rules {
			if e.matchesRule(action, rule) {
				action.Status = model.RemediationStatusApproved
				action.AutoApproved = true
				action.ApprovalRule = rule.Name
				now := time.Now().UTC()
				action.ApprovedAt = &now
				action.ApprovedBy = "auto:" + rule.Name
				action.AddAuditEntry("system", "auto_approved", fmt.Sprintf("Auto-approved by rule: %s", rule.Name))
				break
			}
		}
	}

	if err := e.repo.Create(ctx, action); err != nil {
		return nil, fmt.Errorf("failed to create remediation action: %w", err)
	}

	// If auto-approved, execute immediately in background
	if action.Status == model.RemediationStatusApproved {
		go func() {
			bgCtx := context.Background()
			if err := e.Execute(bgCtx, action.ID); err != nil {
				e.logger.Error("auto-execution failed", "action_id", action.ID, "error", err)
			}
		}()
	}

	return action, nil
}

// Approve marks an action as approved and begins execution.
func (e *Executor) Approve(ctx context.Context, actionID uuid.UUID, approvedBy string) error {
	action, err := e.repo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	if action.Status != model.RemediationStatusPendingApproval {
		return fmt.Errorf("action is not pending approval (current: %s)", action.Status)
	}

	now := time.Now().UTC()
	action.Status = model.RemediationStatusApproved
	action.ApprovedBy = approvedBy
	action.ApprovedAt = &now
	action.AddAuditEntry(approvedBy, "approved", "Action approved for execution")

	if err := e.repo.Update(ctx, action); err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	// Execute in background
	go func() {
		bgCtx := context.Background()
		if err := e.Execute(bgCtx, action.ID); err != nil {
			e.logger.Error("execution failed after approval", "action_id", action.ID, "error", err)
		}
	}()

	return nil
}

// Reject rejects a pending action.
func (e *Executor) Reject(ctx context.Context, actionID uuid.UUID, rejectedBy, reason string) error {
	action, err := e.repo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	if action.Status != model.RemediationStatusPendingApproval {
		return fmt.Errorf("action is not pending approval (current: %s)", action.Status)
	}

	action.Status = model.RemediationStatusRejected
	action.FailureReason = reason
	action.AddAuditEntry(rejectedBy, "rejected", fmt.Sprintf("Rejected: %s", reason))

	return e.repo.Update(ctx, action)
}

// Cancel cancels a pending or approved action before execution.
func (e *Executor) Cancel(ctx context.Context, actionID uuid.UUID, cancelledBy string) error {
	action, err := e.repo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	if action.Status != model.RemediationStatusPendingApproval && action.Status != model.RemediationStatusApproved {
		return fmt.Errorf("action cannot be cancelled (current: %s)", action.Status)
	}

	action.Status = model.RemediationStatusCancelled
	action.AddAuditEntry(cancelledBy, "cancelled", "Action cancelled")

	return e.repo.Update(ctx, action)
}

// Execute runs the remediation action. This is the core execution method.
func (e *Executor) Execute(ctx context.Context, actionID uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	action, err := e.repo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	if action.Status != model.RemediationStatusApproved {
		return fmt.Errorf("action is not approved (current: %s)", action.Status)
	}

	// Mark as executing
	action.Status = model.RemediationStatusExecuting
	now := time.Now().UTC()
	action.ExecutedAt = &now
	action.AddAuditEntry("system", "executing", "Starting execution")
	if err := e.repo.Update(ctx, action); err != nil {
		return err
	}

	// Execute the action (simulated for now - will be replaced with real AWS API calls)
	execErr := e.executeAction(ctx, action)

	if execErr != nil {
		action.Status = model.RemediationStatusFailed
		action.FailureReason = execErr.Error()
		action.AddAuditEntry("system", "failed", fmt.Sprintf("Execution failed: %s", execErr.Error()))
		e.logger.Error("remediation execution failed", "action_id", action.ID, "type", action.Type, "error", execErr)
	} else {
		action.Status = model.RemediationStatusCompleted
		completed := time.Now().UTC()
		action.CompletedAt = &completed
		action.AddAuditEntry("system", "completed", fmt.Sprintf("Successfully executed %s on %s", action.Type, action.ResourceID))
		e.logger.Info("remediation completed", "action_id", action.ID, "type", action.Type, "resource", action.ResourceID, "savings", action.EstimatedSavings)
	}

	return e.repo.Update(ctx, action)
}

// Rollback attempts to reverse a completed remediation action.
func (e *Executor) Rollback(ctx context.Context, actionID uuid.UUID, rolledBackBy string) error {
	action, err := e.repo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	if action.Status != model.RemediationStatusCompleted {
		return fmt.Errorf("only completed actions can be rolled back (current: %s)", action.Status)
	}

	if action.RollbackData == nil || len(action.RollbackData) == 0 {
		return fmt.Errorf("no rollback data available for this action")
	}

	action.AddAuditEntry(rolledBackBy, "rollback_started", "Starting rollback")

	// Execute rollback (simulated)
	rollbackErr := e.rollbackAction(ctx, action)

	if rollbackErr != nil {
		action.AddAuditEntry(rolledBackBy, "rollback_failed", fmt.Sprintf("Rollback failed: %s", rollbackErr.Error()))
		if err := e.repo.Update(ctx, action); err != nil {
			return err
		}
		return rollbackErr
	}

	now := time.Now().UTC()
	action.Status = model.RemediationStatusRolledBack
	action.RolledBackAt = &now
	action.AddAuditEntry(rolledBackBy, "rolled_back", "Successfully rolled back")

	return e.repo.Update(ctx, action)
}

// Repository proxy methods (used by handler)

func (e *Executor) List(ctx context.Context, filter model.RemediationFilter, pagination model.Pagination) ([]*model.RemediationAction, int, error) {
	return e.repo.List(ctx, filter, pagination)
}

func (e *Executor) GetByID(ctx context.Context, id uuid.UUID) (*model.RemediationAction, error) {
	return e.repo.GetByID(ctx, id)
}

func (e *Executor) GetSummary(ctx context.Context, orgID uuid.UUID) (*model.RemediationSummary, error) {
	return e.repo.GetSummary(ctx, orgID)
}

func (e *Executor) ListRules(ctx context.Context, orgID uuid.UUID) ([]*model.AutoApprovalRule, error) {
	return e.repo.ListRules(ctx, orgID)
}

func (e *Executor) GetRuleByID(ctx context.Context, id uuid.UUID) (*model.AutoApprovalRule, error) {
	return e.repo.GetRuleByID(ctx, id)
}

func (e *Executor) CreateRule(ctx context.Context, rule *model.AutoApprovalRule) error {
	return e.repo.CreateRule(ctx, rule)
}

func (e *Executor) UpdateRule(ctx context.Context, rule *model.AutoApprovalRule) error {
	return e.repo.UpdateRule(ctx, rule)
}

func (e *Executor) DeleteRule(ctx context.Context, id uuid.UUID) error {
	return e.repo.DeleteRule(ctx, id)
}

// matchesRule checks if an action matches an auto-approval rule.
func (e *Executor) matchesRule(action *model.RemediationAction, rule *model.AutoApprovalRule) bool {
	cond := rule.Conditions

	// Check savings threshold
	if cond.MaxSavings > 0 && action.EstimatedSavings > cond.MaxSavings {
		return false
	}

	// Check allowed types
	if len(cond.AllowedTypes) > 0 {
		found := false
		for _, t := range cond.AllowedTypes {
			if t == action.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check allowed risks
	if len(cond.AllowedRisks) > 0 {
		found := false
		for _, r := range cond.AllowedRisks {
			if r == action.Risk {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check allowed environments (based on account/resource naming convention)
	if len(cond.AllowedEnvironments) > 0 {
		found := false
		resourceLower := strings.ToLower(action.ResourceID + action.AccountID)
		for _, env := range cond.AllowedEnvironments {
			if strings.Contains(resourceLower, strings.ToLower(env)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// executeAction simulates executing a remediation action.
// TODO: Replace with real AWS API calls (ec2.ModifyInstanceAttribute, ec2.StopInstances, etc.)
func (e *Executor) executeAction(ctx context.Context, action *model.RemediationAction) error {
	e.logger.Info("executing remediation action",
		"type", action.Type,
		"provider", action.Provider,
		"resource", action.ResourceID,
		"region", action.Region,
	)

	switch action.Type {
	case model.RemediationTypeResizeInstance:
		return e.simulateExecution(ctx, action, "Resized instance")
	case model.RemediationTypeStopInstance:
		return e.simulateExecution(ctx, action, "Stopped instance")
	case model.RemediationTypeTerminateInstance:
		return e.simulateExecution(ctx, action, "Terminated instance")
	case model.RemediationTypeDeleteVolume:
		return e.simulateExecution(ctx, action, "Deleted EBS volume")
	case model.RemediationTypeUpgradeStorage:
		return e.simulateExecution(ctx, action, "Upgraded storage type")
	case model.RemediationTypeApplyLifecyclePolicy:
		return e.simulateExecution(ctx, action, "Applied S3 lifecycle policy")
	case model.RemediationTypeReleaseEIP:
		return e.simulateExecution(ctx, action, "Released Elastic IP")
	case model.RemediationTypeDeleteSnapshot:
		return e.simulateExecution(ctx, action, "Deleted old snapshot")
	default:
		return fmt.Errorf("unsupported remediation type: %s", action.Type)
	}
}

func (e *Executor) simulateExecution(_ context.Context, action *model.RemediationAction, msg string) error {
	// Simulate a brief execution delay
	time.Sleep(100 * time.Millisecond)
	e.logger.Info(msg, "resource_id", action.ResourceID, "region", action.Region)
	return nil
}

// rollbackAction simulates rolling back a remediation action.
func (e *Executor) rollbackAction(ctx context.Context, action *model.RemediationAction) error {
	e.logger.Info("rolling back remediation action",
		"type", action.Type,
		"resource", action.ResourceID,
	)
	// Simulate rollback
	time.Sleep(100 * time.Millisecond)
	return nil
}
