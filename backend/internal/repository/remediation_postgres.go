package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/model"
)

type PostgresRemediationRepository struct {
	db *sql.DB
}

func NewPostgresRemediationRepository(db *sql.DB) *PostgresRemediationRepository {
	return &PostgresRemediationRepository{db: db}
}

func (r *PostgresRemediationRepository) Create(ctx context.Context, action *model.RemediationAction) error {
	currentState, _ := json.Marshal(action.CurrentState)
	desiredState, _ := json.Marshal(action.DesiredState)
	rollbackData, _ := json.Marshal(action.RollbackData)
	auditLog, _ := json.Marshal(action.AuditLog)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO remediation_actions (
			id, organization_id, recommendation_id, type, status, provider,
			account_id, region, resource_id, resource_type, description,
			current_state, desired_state, estimated_savings, currency, risk,
			auto_approved, approval_rule, requested_by, approved_by, approved_at,
			executed_at, completed_at, rolled_back_at, failure_reason,
			rollback_data, audit_log, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29
		)`,
		action.ID, action.OrganizationID, action.RecommendationID,
		action.Type, action.Status, action.Provider,
		action.AccountID, action.Region, action.ResourceID, action.ResourceType,
		action.Description, currentState, desiredState,
		action.EstimatedSavings, action.Currency, action.Risk,
		action.AutoApproved, action.ApprovalRule, action.RequestedBy,
		action.ApprovedBy, action.ApprovedAt, action.ExecutedAt,
		action.CompletedAt, action.RolledBackAt, action.FailureReason,
		rollbackData, auditLog, action.CreatedAt, action.UpdatedAt,
	)
	return err
}

func (r *PostgresRemediationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.RemediationAction, error) {
	var a model.RemediationAction
	var currentState, desiredState, rollbackData, auditLog []byte
	var recommendationID *uuid.UUID

	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, recommendation_id, type, status, provider,
			account_id, region, resource_id, resource_type, description,
			current_state, desired_state, estimated_savings, currency, risk,
			auto_approved, approval_rule, requested_by, approved_by, approved_at,
			executed_at, completed_at, rolled_back_at, failure_reason,
			rollback_data, audit_log, created_at, updated_at
		FROM remediation_actions WHERE id = $1`, id,
	).Scan(
		&a.ID, &a.OrganizationID, &recommendationID,
		&a.Type, &a.Status, &a.Provider,
		&a.AccountID, &a.Region, &a.ResourceID, &a.ResourceType,
		&a.Description, &currentState, &desiredState,
		&a.EstimatedSavings, &a.Currency, &a.Risk,
		&a.AutoApproved, &a.ApprovalRule, &a.RequestedBy,
		&a.ApprovedBy, &a.ApprovedAt, &a.ExecutedAt,
		&a.CompletedAt, &a.RolledBackAt, &a.FailureReason,
		&rollbackData, &auditLog, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	a.RecommendationID = recommendationID
	json.Unmarshal(currentState, &a.CurrentState)
	json.Unmarshal(desiredState, &a.DesiredState)
	json.Unmarshal(rollbackData, &a.RollbackData)
	json.Unmarshal(auditLog, &a.AuditLog)

	return &a, nil
}

func (r *PostgresRemediationRepository) List(ctx context.Context, filter model.RemediationFilter, pagination model.Pagination) ([]*model.RemediationAction, int, error) {
	// Build WHERE clause
	where := "WHERE organization_id = $1"
	args := []any{filter.OrganizationID}
	argIdx := 2

	if len(filter.Statuses) > 0 {
		where += fmt.Sprintf(" AND status = ANY($%d)", argIdx)
		args = append(args, statusSliceToStringSlice(filter.Statuses))
		argIdx++
	}
	if len(filter.Types) > 0 {
		where += fmt.Sprintf(" AND type = ANY($%d)", argIdx)
		args = append(args, typeSliceToStringSlice(filter.Types))
		argIdx++
	}
	if len(filter.Risks) > 0 {
		where += fmt.Sprintf(" AND risk = ANY($%d)", argIdx)
		args = append(args, riskSliceToStringSlice(filter.Risks))
		argIdx++
	}

	// Count
	var total int
	countQuery := "SELECT COUNT(*) FROM remediation_actions " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch
	offset := (pagination.Page - 1) * pagination.PageSize
	query := fmt.Sprintf(`
		SELECT id, organization_id, recommendation_id, type, status, provider,
			account_id, region, resource_id, resource_type, description,
			current_state, desired_state, estimated_savings, currency, risk,
			auto_approved, approval_rule, requested_by, approved_by, approved_at,
			executed_at, completed_at, rolled_back_at, failure_reason,
			rollback_data, audit_log, created_at, updated_at
		FROM remediation_actions %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, pagination.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var actions []*model.RemediationAction
	for rows.Next() {
		var a model.RemediationAction
		var currentState, desiredState, rollbackData, auditLog []byte
		var recommendationID *uuid.UUID

		if err := rows.Scan(
			&a.ID, &a.OrganizationID, &recommendationID,
			&a.Type, &a.Status, &a.Provider,
			&a.AccountID, &a.Region, &a.ResourceID, &a.ResourceType,
			&a.Description, &currentState, &desiredState,
			&a.EstimatedSavings, &a.Currency, &a.Risk,
			&a.AutoApproved, &a.ApprovalRule, &a.RequestedBy,
			&a.ApprovedBy, &a.ApprovedAt, &a.ExecutedAt,
			&a.CompletedAt, &a.RolledBackAt, &a.FailureReason,
			&rollbackData, &auditLog, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}

		a.RecommendationID = recommendationID
		json.Unmarshal(currentState, &a.CurrentState)
		json.Unmarshal(desiredState, &a.DesiredState)
		json.Unmarshal(rollbackData, &a.RollbackData)
		json.Unmarshal(auditLog, &a.AuditLog)
		actions = append(actions, &a)
	}

	return actions, total, nil
}

func (r *PostgresRemediationRepository) Update(ctx context.Context, action *model.RemediationAction) error {
	currentState, _ := json.Marshal(action.CurrentState)
	desiredState, _ := json.Marshal(action.DesiredState)
	rollbackData, _ := json.Marshal(action.RollbackData)
	auditLog, _ := json.Marshal(action.AuditLog)

	_, err := r.db.ExecContext(ctx, `
		UPDATE remediation_actions SET
			status = $2, approved_by = $3, approved_at = $4,
			executed_at = $5, completed_at = $6, rolled_back_at = $7,
			failure_reason = $8, current_state = $9, desired_state = $10,
			rollback_data = $11, audit_log = $12, auto_approved = $13,
			approval_rule = $14, updated_at = NOW()
		WHERE id = $1`,
		action.ID, action.Status, action.ApprovedBy, action.ApprovedAt,
		action.ExecutedAt, action.CompletedAt, action.RolledBackAt,
		action.FailureReason, currentState, desiredState,
		rollbackData, auditLog, action.AutoApproved, action.ApprovalRule,
	)
	return err
}

func (r *PostgresRemediationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.RemediationStatus) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE remediation_actions SET status = $2, updated_at = NOW() WHERE id = $1",
		id, status)
	return err
}

func (r *PostgresRemediationRepository) GetSummary(ctx context.Context, orgID uuid.UUID) (*model.RemediationSummary, error) {
	summary := &model.RemediationSummary{
		ByType:   make(map[model.RemediationType]int),
		ByStatus: make(map[model.RemediationStatus]int),
		ByRisk:   make(map[model.RemediationRisk]int),
		Currency: model.CurrencyUSD,
	}

	// Get counts by status
	rows, err := r.db.QueryContext(ctx,
		"SELECT status, COUNT(*) FROM remediation_actions WHERE organization_id = $1 GROUP BY status", orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status model.RemediationStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		summary.ByStatus[status] = count
		summary.TotalCount += count
		switch status {
		case model.RemediationStatusPendingApproval:
			summary.PendingCount = count
		case model.RemediationStatusApproved:
			summary.ApprovedCount = count
		case model.RemediationStatusCompleted:
			summary.CompletedCount = count
		case model.RemediationStatusFailed:
			summary.FailedCount = count
		}
	}

	// Get savings
	r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(estimated_savings), 0) FROM remediation_actions WHERE organization_id = $1 AND status = 'completed'",
		orgID).Scan(&summary.TotalSavingsRealized)

	r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(estimated_savings), 0) FROM remediation_actions WHERE organization_id = $1 AND status IN ('pending_approval', 'approved')",
		orgID).Scan(&summary.TotalSavingsPending)

	return summary, nil
}

// Auto-approval rule methods

func (r *PostgresRemediationRepository) CreateRule(ctx context.Context, rule *model.AutoApprovalRule) error {
	conditions, _ := json.Marshal(rule.Conditions)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auto_approval_rules (id, organization_id, name, enabled, conditions, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rule.ID, rule.OrganizationID, rule.Name, rule.Enabled, conditions, rule.CreatedBy, rule.CreatedAt, rule.UpdatedAt,
	)
	return err
}

func (r *PostgresRemediationRepository) ListRules(ctx context.Context, orgID uuid.UUID) ([]*model.AutoApprovalRule, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, organization_id, name, enabled, conditions, created_by, created_at, updated_at FROM auto_approval_rules WHERE organization_id = $1 ORDER BY created_at", orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*model.AutoApprovalRule
	for rows.Next() {
		var rule model.AutoApprovalRule
		var conditions []byte
		if err := rows.Scan(&rule.ID, &rule.OrganizationID, &rule.Name, &rule.Enabled, &conditions, &rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(conditions, &rule.Conditions)
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *PostgresRemediationRepository) GetRuleByID(ctx context.Context, id uuid.UUID) (*model.AutoApprovalRule, error) {
	var rule model.AutoApprovalRule
	var conditions []byte
	err := r.db.QueryRowContext(ctx,
		"SELECT id, organization_id, name, enabled, conditions, created_by, created_at, updated_at FROM auto_approval_rules WHERE id = $1", id,
	).Scan(&rule.ID, &rule.OrganizationID, &rule.Name, &rule.Enabled, &conditions, &rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(conditions, &rule.Conditions)
	return &rule, nil
}

func (r *PostgresRemediationRepository) UpdateRule(ctx context.Context, rule *model.AutoApprovalRule) error {
	conditions, _ := json.Marshal(rule.Conditions)
	_, err := r.db.ExecContext(ctx,
		"UPDATE auto_approval_rules SET name = $2, enabled = $3, conditions = $4, updated_at = NOW() WHERE id = $1",
		rule.ID, rule.Name, rule.Enabled, conditions)
	return err
}

func (r *PostgresRemediationRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM auto_approval_rules WHERE id = $1", id)
	return err
}

func (r *PostgresRemediationRepository) GetActiveRules(ctx context.Context, orgID uuid.UUID) ([]*model.AutoApprovalRule, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, organization_id, name, enabled, conditions, created_by, created_at, updated_at FROM auto_approval_rules WHERE organization_id = $1 AND enabled = true", orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*model.AutoApprovalRule
	for rows.Next() {
		var rule model.AutoApprovalRule
		var conditions []byte
		if err := rows.Scan(&rule.ID, &rule.OrganizationID, &rule.Name, &rule.Enabled, &conditions, &rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(conditions, &rule.Conditions)
		rules = append(rules, &rule)
	}
	return rules, nil
}

// Helper functions for slice conversion (needed for PostgreSQL ANY() queries)

func statusSliceToStringSlice(statuses []model.RemediationStatus) []string {
	s := make([]string, len(statuses))
	for i, v := range statuses {
		s[i] = string(v)
	}
	return s
}

func typeSliceToStringSlice(types []model.RemediationType) []string {
	s := make([]string, len(types))
	for i, v := range types {
		s[i] = string(v)
	}
	return s
}

func riskSliceToStringSlice(risks []model.RemediationRisk) []string {
	s := make([]string, len(risks))
	for i, v := range risks {
		s[i] = string(v)
	}
	return s
}
