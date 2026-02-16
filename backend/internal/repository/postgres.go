// Package repository provides PostgreSQL repository implementations.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/model"
)

// PostgresCostRepository implements CostRepository for PostgreSQL.
type PostgresCostRepository struct {
	db *sql.DB
}

// NewPostgresCostRepository creates a new PostgresCostRepository.
func NewPostgresCostRepository(db *sql.DB) *PostgresCostRepository {
	return &PostgresCostRepository{db: db}
}

func (r *PostgresCostRepository) Create(ctx context.Context, cost *model.CostRecord) error {
	tagsJSON, _ := json.Marshal(cost.Tags)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO costs (id, organization_id, date, amount, currency, provider, service, account_id, region, resource_id, tags, estimated, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, cost.ID, cost.OrganizationID, cost.Date, cost.Amount, cost.Currency, cost.Provider,
		cost.Service, cost.AccountID, cost.Region, cost.ResourceID, tagsJSON, cost.Estimated, cost.CreatedAt)
	return err
}

func (r *PostgresCostRepository) CreateBatch(ctx context.Context, costs []*model.CostRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO costs (id, organization_id, date, amount, currency, provider, service, account_id, region, resource_id, tags, estimated, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (organization_id, date, provider, service, account_id, region, resource_id) 
		DO UPDATE SET amount = EXCLUDED.amount, estimated = EXCLUDED.estimated
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cost := range costs {
		tagsJSON, _ := json.Marshal(cost.Tags)
		_, err := stmt.ExecContext(ctx, cost.ID, cost.OrganizationID, cost.Date, cost.Amount, cost.Currency,
			cost.Provider, cost.Service, cost.AccountID, cost.Region, cost.ResourceID, tagsJSON, cost.Estimated, cost.CreatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresCostRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CostRecord, error) {
	var cost model.CostRecord
	var tagsJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, date, amount, currency, provider, service, account_id, region, resource_id, tags, estimated, created_at
		FROM costs WHERE id = $1
	`, id).Scan(&cost.ID, &cost.OrganizationID, &cost.Date, &cost.Amount, &cost.Currency, &cost.Provider,
		&cost.Service, &cost.AccountID, &cost.Region, &cost.ResourceID, &tagsJSON, &cost.Estimated, &cost.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(tagsJSON, &cost.Tags)
	return &cost, nil
}

func (r *PostgresCostRepository) List(ctx context.Context, filter model.CostFilter, pagination model.Pagination) ([]*model.CostRecord, int, error) {
	query := `SELECT id, organization_id, date, amount, currency, provider, service, account_id, region, resource_id, tags, estimated, created_at
		FROM costs WHERE organization_id = $1 AND date >= $2 AND date <= $3`
	countQuery := `SELECT COUNT(*) FROM costs WHERE organization_id = $1 AND date >= $2 AND date <= $3`
	args := []interface{}{filter.OrganizationID, filter.DateRange.Start, filter.DateRange.End}

	query += fmt.Sprintf(" ORDER BY date DESC LIMIT %d OFFSET %d", pagination.PageSize, (pagination.Page-1)*pagination.PageSize)

	var total int
	r.db.QueryRowContext(ctx, countQuery, args[:3]...).Scan(&total)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var costs []*model.CostRecord
	for rows.Next() {
		var cost model.CostRecord
		var tagsJSON []byte
		err := rows.Scan(&cost.ID, &cost.OrganizationID, &cost.Date, &cost.Amount, &cost.Currency, &cost.Provider,
			&cost.Service, &cost.AccountID, &cost.Region, &cost.ResourceID, &tagsJSON, &cost.Estimated, &cost.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		json.Unmarshal(tagsJSON, &cost.Tags)
		costs = append(costs, &cost)
	}

	return costs, total, nil
}

func (r *PostgresCostRepository) GetAggregated(ctx context.Context, filter model.CostFilter) ([]model.CostAggregation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT date, SUM(amount) as total, provider, service, COUNT(*) as record_count
		FROM costs 
		WHERE organization_id = $1 AND date >= $2 AND date <= $3
		GROUP BY date, provider, service
		ORDER BY date
	`, filter.OrganizationID, filter.DateRange.Start, filter.DateRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggregations []model.CostAggregation
	for rows.Next() {
		var agg model.CostAggregation
		err := rows.Scan(&agg.Date, &agg.Total, &agg.Provider, &agg.Service, &agg.RecordCount)
		if err != nil {
			return nil, err
		}
		agg.Currency = model.CurrencyUSD
		aggregations = append(aggregations, agg)
	}
	return aggregations, nil
}

func (r *PostgresCostRepository) GetBreakdown(ctx context.Context, filter model.CostFilter, dimension string) (*model.CostBreakdown, error) {
	query := fmt.Sprintf(`
		SELECT %s, SUM(amount) as total
		FROM costs 
		WHERE organization_id = $1 AND date >= $2 AND date <= $3
		GROUP BY %s
		ORDER BY total DESC
	`, dimension, dimension)

	rows, err := r.db.QueryContext(ctx, query, filter.OrganizationID, filter.DateRange.Start, filter.DateRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.CostBreakdownItem
	var total float64
	for rows.Next() {
		var item model.CostBreakdownItem
		err := rows.Scan(&item.Name, &item.Amount)
		if err != nil {
			return nil, err
		}
		total += item.Amount
		items = append(items, item)
	}

	for i := range items {
		items[i].Percentage = (items[i].Amount / total) * 100
	}

	return &model.CostBreakdown{
		Dimension: dimension,
		Items:     items,
		Total:     total,
		Currency:  model.CurrencyUSD,
	}, nil
}

func (r *PostgresCostRepository) GetTrend(ctx context.Context, filter model.CostFilter) (*model.CostTrend, error) {
	aggregations, err := r.GetAggregated(ctx, filter)
	if err != nil {
		return nil, err
	}

	var totalCost float64
	for _, agg := range aggregations {
		totalCost += agg.Total
	}

	days := filter.DateRange.End.Sub(filter.DateRange.Start).Hours() / 24
	avgDaily := totalCost / days

	return &model.CostTrend{
		DateRange:    filter.DateRange,
		Granularity:  filter.Granularity,
		DataPoints:   aggregations,
		TotalCost:    totalCost,
		AvgDailyCost: avgDaily,
	}, nil
}

func (r *PostgresCostRepository) GetSummary(ctx context.Context, orgID uuid.UUID, dateRange model.DateRange) (*model.CostSummary, error) {
	filter := model.CostFilter{OrganizationID: orgID, DateRange: dateRange}
	
	breakdown, err := r.GetBreakdown(ctx, filter, "service")
	if err != nil {
		return nil, err
	}

	return &model.CostSummary{
		TotalCost: breakdown.Total,
		Currency:  model.CurrencyUSD,
		DateRange: dateRange,
		ByService: breakdown.Items,
	}, nil
}

func (r *PostgresCostRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM costs WHERE id = $1", id)
	return err
}

// PostgresBudgetRepository implements BudgetRepository for PostgreSQL.
type PostgresBudgetRepository struct {
	db *sql.DB
}

func NewPostgresBudgetRepository(db *sql.DB) *PostgresBudgetRepository {
	return &PostgresBudgetRepository{db: db}
}

func (r *PostgresBudgetRepository) Create(ctx context.Context, budget *model.Budget) error {
	filtersJSON, _ := json.Marshal(budget.Filters)
	thresholdsJSON, _ := json.Marshal(budget.Thresholds)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO budgets (id, organization_id, name, amount, currency, period, filters, thresholds, status, current_spend, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, budget.ID, budget.OrganizationID, budget.Name, budget.Amount, budget.Currency, budget.Period,
		filtersJSON, thresholdsJSON, budget.Status, budget.CurrentSpend, budget.CreatedAt, budget.UpdatedAt)
	return err
}

func (r *PostgresBudgetRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	var filtersJSON, thresholdsJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, name, amount, currency, period, filters, thresholds, status, current_spend, created_at, updated_at
		FROM budgets WHERE id = $1
	`, id).Scan(&budget.ID, &budget.OrganizationID, &budget.Name, &budget.Amount, &budget.Currency, &budget.Period,
		&filtersJSON, &thresholdsJSON, &budget.Status, &budget.CurrentSpend, &budget.CreatedAt, &budget.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(filtersJSON, &budget.Filters)
	json.Unmarshal(thresholdsJSON, &budget.Thresholds)
	return &budget, nil
}

func (r *PostgresBudgetRepository) List(ctx context.Context, orgID uuid.UUID) ([]*model.Budget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, name, amount, currency, period, filters, thresholds, status, current_spend, created_at, updated_at
		FROM budgets WHERE organization_id = $1 ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var budgets []*model.Budget
	for rows.Next() {
		var budget model.Budget
		var filtersJSON, thresholdsJSON []byte
		err := rows.Scan(&budget.ID, &budget.OrganizationID, &budget.Name, &budget.Amount, &budget.Currency, &budget.Period,
			&filtersJSON, &thresholdsJSON, &budget.Status, &budget.CurrentSpend, &budget.CreatedAt, &budget.UpdatedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(filtersJSON, &budget.Filters)
		json.Unmarshal(thresholdsJSON, &budget.Thresholds)
		budgets = append(budgets, &budget)
	}
	return budgets, nil
}

func (r *PostgresBudgetRepository) Update(ctx context.Context, budget *model.Budget) error {
	filtersJSON, _ := json.Marshal(budget.Filters)
	thresholdsJSON, _ := json.Marshal(budget.Thresholds)
	budget.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE budgets SET name = $2, amount = $3, filters = $4, thresholds = $5, status = $6, current_spend = $7, updated_at = $8
		WHERE id = $1
	`, budget.ID, budget.Name, budget.Amount, filtersJSON, thresholdsJSON, budget.Status, budget.CurrentSpend, budget.UpdatedAt)
	return err
}

func (r *PostgresBudgetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM budgets WHERE id = $1", id)
	return err
}

func (r *PostgresBudgetRepository) UpdateSpend(ctx context.Context, id uuid.UUID, currentSpend, forecastedSpend float64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE budgets SET current_spend = $2, forecasted_spend = $3, updated_at = $4 WHERE id = $1
	`, id, currentSpend, forecastedSpend, time.Now().UTC())
	return err
}

func (r *PostgresBudgetRepository) GetSummary(ctx context.Context, orgID uuid.UUID) (*model.BudgetSummary, error) {
	var summary model.BudgetSummary
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0), COALESCE(SUM(current_spend), 0), COALESCE(SUM(forecasted_spend), 0),
			COUNT(*) FILTER (WHERE status = 'warning'), COUNT(*) FILTER (WHERE status = 'exceeded')
		FROM budgets WHERE organization_id = $1
	`, orgID).Scan(&summary.TotalBudgets, &summary.TotalBudgeted, &summary.TotalSpent, &summary.TotalForecasted,
		&summary.AtRiskCount, &summary.ExceededCount)
	return &summary, err
}

// PostgresAnomalyRepository implements AnomalyRepository for PostgreSQL.
type PostgresAnomalyRepository struct {
	db *sql.DB
}

func NewPostgresAnomalyRepository(db *sql.DB) *PostgresAnomalyRepository {
	return &PostgresAnomalyRepository{db: db}
}

func (r *PostgresAnomalyRepository) Create(ctx context.Context, anomaly *model.Anomaly) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO anomalies (id, organization_id, date, actual_amount, expected_amount, deviation, deviation_pct, score, severity, status, service, account_id, region, detected_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, anomaly.ID, anomaly.OrganizationID, anomaly.Date, anomaly.ActualAmount, anomaly.ExpectedAmount,
		anomaly.Deviation, anomaly.DeviationPct, anomaly.Score, anomaly.Severity, anomaly.Status,
		anomaly.Service, anomaly.AccountID, anomaly.Region, anomaly.DetectedAt, anomaly.CreatedAt, anomaly.UpdatedAt)
	return err
}

func (r *PostgresAnomalyRepository) CreateBatch(ctx context.Context, anomalies []*model.Anomaly) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, anomaly := range anomalies {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO anomalies (id, organization_id, date, actual_amount, expected_amount, deviation, deviation_pct, score, severity, status, service, account_id, region, detected_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, anomaly.ID, anomaly.OrganizationID, anomaly.Date, anomaly.ActualAmount, anomaly.ExpectedAmount,
			anomaly.Deviation, anomaly.DeviationPct, anomaly.Score, anomaly.Severity, anomaly.Status,
			anomaly.Service, anomaly.AccountID, anomaly.Region, anomaly.DetectedAt, anomaly.CreatedAt, anomaly.UpdatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresAnomalyRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Anomaly, error) {
	var anomaly model.Anomaly
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, date, actual_amount, expected_amount, deviation, deviation_pct, score, severity, status, service, account_id, region, root_cause, notes, detected_at, acknowledged_at, acknowledged_by, resolved_at, created_at, updated_at
		FROM anomalies WHERE id = $1
	`, id).Scan(&anomaly.ID, &anomaly.OrganizationID, &anomaly.Date, &anomaly.ActualAmount, &anomaly.ExpectedAmount,
		&anomaly.Deviation, &anomaly.DeviationPct, &anomaly.Score, &anomaly.Severity, &anomaly.Status,
		&anomaly.Service, &anomaly.AccountID, &anomaly.Region, &anomaly.RootCause, &anomaly.Notes,
		&anomaly.DetectedAt, &anomaly.AcknowledgedAt, &anomaly.AcknowledgedBy, &anomaly.ResolvedAt, &anomaly.CreatedAt, &anomaly.UpdatedAt)
	return &anomaly, err
}

func (r *PostgresAnomalyRepository) List(ctx context.Context, filter model.AnomalyFilter, pagination model.Pagination) ([]*model.Anomaly, int, error) {
	query := `SELECT id, organization_id, date, actual_amount, expected_amount, deviation, deviation_pct, score, severity, status, service, account_id, region, detected_at, created_at
		FROM anomalies WHERE organization_id = $1`
	args := []interface{}{filter.OrganizationID}

	query += fmt.Sprintf(" ORDER BY detected_at DESC LIMIT %d OFFSET %d", pagination.PageSize, (pagination.Page-1)*pagination.PageSize)

	var total int
	r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM anomalies WHERE organization_id = $1", filter.OrganizationID).Scan(&total)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var anomalies []*model.Anomaly
	for rows.Next() {
		var a model.Anomaly
		err := rows.Scan(&a.ID, &a.OrganizationID, &a.Date, &a.ActualAmount, &a.ExpectedAmount,
			&a.Deviation, &a.DeviationPct, &a.Score, &a.Severity, &a.Status, &a.Service, &a.AccountID, &a.Region, &a.DetectedAt, &a.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		anomalies = append(anomalies, &a)
	}
	return anomalies, total, nil
}

func (r *PostgresAnomalyRepository) Update(ctx context.Context, anomaly *model.Anomaly) error {
	anomaly.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE anomalies SET status = $2, root_cause = $3, notes = $4, updated_at = $5 WHERE id = $1
	`, anomaly.ID, anomaly.Status, anomaly.RootCause, anomaly.Notes, anomaly.UpdatedAt)
	return err
}

func (r *PostgresAnomalyRepository) Acknowledge(ctx context.Context, id uuid.UUID, acknowledgedBy string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE anomalies SET status = 'acknowledged', acknowledged_at = $2, acknowledged_by = $3, updated_at = $4 WHERE id = $1
	`, id, now, acknowledgedBy, now)
	return err
}

func (r *PostgresAnomalyRepository) Resolve(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE anomalies SET status = 'resolved', resolved_at = $2, updated_at = $3 WHERE id = $1
	`, id, now, now)
	return err
}

func (r *PostgresAnomalyRepository) GetSummary(ctx context.Context, orgID uuid.UUID) (*model.AnomalySummary, error) {
	var summary model.AnomalySummary
	summary.BySeverity = make(map[model.Severity]int)

	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'open'), COALESCE(SUM(deviation), 0), COALESCE(AVG(deviation), 0)
		FROM anomalies WHERE organization_id = $1
	`, orgID).Scan(&summary.TotalCount, &summary.OpenCount, &summary.TotalDeviation, &summary.AvgDeviation)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT severity, COUNT(*) FROM anomalies WHERE organization_id = $1 GROUP BY severity
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var severity model.Severity
		var count int
		rows.Scan(&severity, &count)
		summary.BySeverity[severity] = count
	}

	return &summary, nil
}

// PostgresOrganizationRepository implements OrganizationRepository for PostgreSQL.
type PostgresOrganizationRepository struct {
	db *sql.DB
}

func NewPostgresOrganizationRepository(db *sql.DB) *PostgresOrganizationRepository {
	return &PostgresOrganizationRepository{db: db}
}

func (r *PostgresOrganizationRepository) Create(ctx context.Context, org *model.Organization) error {
	settingsJSON, _ := json.Marshal(org.Settings)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO organizations (id, name, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, org.ID, org.Name, settingsJSON, org.CreatedAt, org.UpdatedAt)
	return err
}

func (r *PostgresOrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	var org model.Organization
	var settingsJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, settings, created_at, updated_at FROM organizations WHERE id = $1
	`, id).Scan(&org.ID, &org.Name, &settingsJSON, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(settingsJSON, &org.Settings)
	return &org, nil
}

func (r *PostgresOrganizationRepository) List(ctx context.Context) ([]*model.Organization, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, settings, created_at, updated_at FROM organizations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*model.Organization
	for rows.Next() {
		var org model.Organization
		var settingsJSON []byte
		err := rows.Scan(&org.ID, &org.Name, &settingsJSON, &org.CreatedAt, &org.UpdatedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(settingsJSON, &org.Settings)
		orgs = append(orgs, &org)
	}
	return orgs, nil
}

func (r *PostgresOrganizationRepository) Update(ctx context.Context, org *model.Organization) error {
	settingsJSON, _ := json.Marshal(org.Settings)
	org.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE organizations SET name = $2, settings = $3, updated_at = $4 WHERE id = $1
	`, org.ID, org.Name, settingsJSON, org.UpdatedAt)
	return err
}

func (r *PostgresOrganizationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", id)
	return err
}
