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

// PostgresForecastRepository implements ForecastRepository for PostgreSQL.
type PostgresForecastRepository struct {
	db *sql.DB
}

// NewPostgresForecastRepository creates a new PostgresForecastRepository.
func NewPostgresForecastRepository(db *sql.DB) *PostgresForecastRepository {
	return &PostgresForecastRepository{db: db}
}

func (r *PostgresForecastRepository) Create(ctx context.Context, forecast *model.Forecast) error {
	predictionsJSON, _ := json.Marshal(forecast.Predictions)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO forecasts (id, organization_id, generated_at, model_version, granularity, predictions, total_forecasted, confidence_level, currency, service_filter, account_filter, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			predictions = EXCLUDED.predictions,
			total_forecasted = EXCLUDED.total_forecasted,
			confidence_level = EXCLUDED.confidence_level,
			updated_at = EXCLUDED.updated_at
	`, forecast.ID, forecast.OrganizationID, forecast.GeneratedAt, forecast.ModelVersion,
		forecast.Granularity, predictionsJSON, forecast.TotalForecasted, forecast.ConfidenceLevel,
		forecast.Currency, forecast.ServiceFilter, forecast.AccountFilter, forecast.CreatedAt, forecast.UpdatedAt)
	return err
}

func (r *PostgresForecastRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Forecast, error) {
	var forecast model.Forecast
	var predictionsJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, generated_at, model_version, granularity, predictions, total_forecasted, confidence_level, currency, service_filter, account_filter, created_at, updated_at
		FROM forecasts WHERE id = $1
	`, id).Scan(&forecast.ID, &forecast.OrganizationID, &forecast.GeneratedAt, &forecast.ModelVersion,
		&forecast.Granularity, &predictionsJSON, &forecast.TotalForecasted, &forecast.ConfidenceLevel,
		&forecast.Currency, &forecast.ServiceFilter, &forecast.AccountFilter, &forecast.CreatedAt, &forecast.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(predictionsJSON, &forecast.Predictions)
	return &forecast, nil
}

func (r *PostgresForecastRepository) GetLatest(ctx context.Context, orgID uuid.UUID) (*model.Forecast, error) {
	var forecast model.Forecast
	var predictionsJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, generated_at, model_version, granularity, predictions, total_forecasted, confidence_level, currency, service_filter, account_filter, created_at, updated_at
		FROM forecasts WHERE organization_id = $1 ORDER BY generated_at DESC LIMIT 1
	`, orgID).Scan(&forecast.ID, &forecast.OrganizationID, &forecast.GeneratedAt, &forecast.ModelVersion,
		&forecast.Granularity, &predictionsJSON, &forecast.TotalForecasted, &forecast.ConfidenceLevel,
		&forecast.Currency, &forecast.ServiceFilter, &forecast.AccountFilter, &forecast.CreatedAt, &forecast.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(predictionsJSON, &forecast.Predictions)
	return &forecast, nil
}

func (r *PostgresForecastRepository) List(ctx context.Context, orgID uuid.UUID, pagination model.Pagination) ([]*model.Forecast, int, error) {
	var total int
	r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM forecasts WHERE organization_id = $1", orgID).Scan(&total)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, organization_id, generated_at, model_version, granularity, predictions, total_forecasted, confidence_level, currency, service_filter, account_filter, created_at, updated_at
		FROM forecasts WHERE organization_id = $1 ORDER BY generated_at DESC LIMIT %d OFFSET %d
	`, pagination.PageSize, (pagination.Page-1)*pagination.PageSize), orgID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var forecasts []*model.Forecast
	for rows.Next() {
		var f model.Forecast
		var predictionsJSON []byte
		err := rows.Scan(&f.ID, &f.OrganizationID, &f.GeneratedAt, &f.ModelVersion,
			&f.Granularity, &predictionsJSON, &f.TotalForecasted, &f.ConfidenceLevel,
			&f.Currency, &f.ServiceFilter, &f.AccountFilter, &f.CreatedAt, &f.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		json.Unmarshal(predictionsJSON, &f.Predictions)
		forecasts = append(forecasts, &f)
	}
	return forecasts, total, nil
}

func (r *PostgresForecastRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM forecasts WHERE id = $1", id)
	return err
}

// EnsureTable creates the forecasts table if it doesn't exist.
func (r *PostgresForecastRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS forecasts (
			id UUID PRIMARY KEY,
			organization_id UUID NOT NULL REFERENCES organizations(id),
			generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			model_version VARCHAR(50) NOT NULL DEFAULT '',
			granularity VARCHAR(20) NOT NULL DEFAULT 'daily',
			predictions JSONB NOT NULL DEFAULT '[]',
			total_forecasted DOUBLE PRECISION NOT NULL DEFAULT 0,
			confidence_level DOUBLE PRECISION NOT NULL DEFAULT 0,
			currency VARCHAR(3) NOT NULL DEFAULT 'USD',
			service_filter VARCHAR(255) DEFAULT '',
			account_filter VARCHAR(255) DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create forecasts table: %w", err)
	}

	// Index for fast latest-per-org lookup
	r.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_forecasts_org_generated ON forecasts (organization_id, generated_at DESC)`)
	return nil
}

func init() {
	_ = time.Now // ensure time import is used
}
