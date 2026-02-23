package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/model"
)

// PostgresCloudProviderRepository implements CloudProviderRepository for PostgreSQL.
type PostgresCloudProviderRepository struct {
	db *sql.DB
}

// NewPostgresCloudProviderRepository creates a new PostgresCloudProviderRepository.
func NewPostgresCloudProviderRepository(db *sql.DB) *PostgresCloudProviderRepository {
	return &PostgresCloudProviderRepository{db: db}
}

func (r *PostgresCloudProviderRepository) Create(ctx context.Context, p *model.CloudProviderConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cloud_providers (id, organization_id, provider_type, name, credentials, enabled, status, status_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, p.ID, p.OrganizationID, p.ProviderType, p.Name, p.Credentials, p.Enabled, p.Status, p.StatusMessage, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PostgresCloudProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CloudProviderConfig, error) {
	var p model.CloudProviderConfig
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, provider_type, name, credentials, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM cloud_providers WHERE id = $1
	`, id).Scan(&p.ID, &p.OrganizationID, &p.ProviderType, &p.Name, &p.Credentials, &p.Enabled, &p.Status, &p.StatusMessage, &p.LastSyncAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresCloudProviderRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*model.CloudProviderConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, provider_type, name, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM cloud_providers WHERE organization_id = $1 ORDER BY created_at
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*model.CloudProviderConfig
	for rows.Next() {
		var p model.CloudProviderConfig
		err := rows.Scan(&p.ID, &p.OrganizationID, &p.ProviderType, &p.Name, &p.Enabled, &p.Status, &p.StatusMessage, &p.LastSyncAt, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		providers = append(providers, &p)
	}
	return providers, nil
}

func (r *PostgresCloudProviderRepository) GetByOrgAndType(ctx context.Context, orgID uuid.UUID, providerType model.CloudProvider) (*model.CloudProviderConfig, error) {
	var p model.CloudProviderConfig
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, provider_type, name, credentials, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM cloud_providers WHERE organization_id = $1 AND provider_type = $2
	`, orgID, providerType).Scan(&p.ID, &p.OrganizationID, &p.ProviderType, &p.Name, &p.Credentials, &p.Enabled, &p.Status, &p.StatusMessage, &p.LastSyncAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresCloudProviderRepository) GetAllEnabled(ctx context.Context) ([]*model.CloudProviderConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, provider_type, name, credentials, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM cloud_providers WHERE enabled = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*model.CloudProviderConfig
	for rows.Next() {
		var p model.CloudProviderConfig
		err := rows.Scan(&p.ID, &p.OrganizationID, &p.ProviderType, &p.Name, &p.Credentials, &p.Enabled, &p.Status, &p.StatusMessage, &p.LastSyncAt, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		providers = append(providers, &p)
	}
	return providers, nil
}

func (r *PostgresCloudProviderRepository) Update(ctx context.Context, p *model.CloudProviderConfig) error {
	p.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE cloud_providers SET name = $2, credentials = $3, enabled = $4, updated_at = $5
		WHERE id = $1
	`, p.ID, p.Name, p.Credentials, p.Enabled, p.UpdatedAt)
	return err
}

func (r *PostgresCloudProviderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, message string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE cloud_providers SET status = $2, status_message = $3, updated_at = $4 WHERE id = $1
	`, id, status, message, time.Now().UTC())
	return err
}

func (r *PostgresCloudProviderRepository) UpdateLastSync(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE cloud_providers SET last_sync_at = $2, updated_at = $3 WHERE id = $1
	`, id, now, now)
	return err
}

func (r *PostgresCloudProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM cloud_providers WHERE id = $1", id)
	return err
}
