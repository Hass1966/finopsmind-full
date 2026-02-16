package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/model"
)

// UserRepository defines data access methods for users.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*model.User, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*model.User, error)
	Update(ctx context.Context, user *model.User) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID, t time.Time) error
	SetAPIKeyHash(ctx context.Context, id uuid.UUID, hash string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PostgresUserRepository implements UserRepository for PostgreSQL.
type PostgresUserRepository struct {
	db *sql.DB
}

// NewPostgresUserRepository creates a new PostgresUserRepository.
func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, organization_id, email, password_hash, first_name, last_name, role, api_key_hash, last_login_at, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, user.ID, user.OrganizationID, user.Email, user.PasswordHash,
		user.FirstName, user.LastName, user.Role, nilIfEmpty(user.APIKeyHash),
		user.LastLoginAt, user.Active, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return r.scanUser(r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, email, password_hash, first_name, last_name, role,
		       COALESCE(api_key_hash, ''), last_login_at, active, created_at, updated_at
		FROM users WHERE id = $1
	`, id))
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.scanUser(r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, email, password_hash, first_name, last_name, role,
		       COALESCE(api_key_hash, ''), last_login_at, active, created_at, updated_at
		FROM users WHERE email = $1
	`, email))
}

func (r *PostgresUserRepository) GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*model.User, error) {
	return r.scanUser(r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, email, password_hash, first_name, last_name, role,
		       COALESCE(api_key_hash, ''), last_login_at, active, created_at, updated_at
		FROM users WHERE api_key_hash = $1
	`, apiKeyHash))
}

func (r *PostgresUserRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*model.User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, email, password_hash, first_name, last_name, role,
		       COALESCE(api_key_hash, ''), last_login_at, active, created_at, updated_at
		FROM users WHERE organization_id = $1 ORDER BY created_at ASC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var u model.User
		err := rows.Scan(&u.ID, &u.OrganizationID, &u.Email, &u.PasswordHash,
			&u.FirstName, &u.LastName, &u.Role, &u.APIKeyHash,
			&u.LastLoginAt, &u.Active, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (r *PostgresUserRepository) Update(ctx context.Context, user *model.User) error {
	user.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email = $2, first_name = $3, last_name = $4, role = $5, active = $6, updated_at = $7
		WHERE id = $1
	`, user.ID, user.Email, user.FirstName, user.LastName, user.Role, user.Active, user.UpdatedAt)
	return err
}

func (r *PostgresUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID, t time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET last_login_at = $2, updated_at = $3 WHERE id = $1
	`, id, t, time.Now().UTC())
	return err
}

func (r *PostgresUserRepository) SetAPIKeyHash(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET api_key_hash = $2, updated_at = $3 WHERE id = $1
	`, id, hash, time.Now().UTC())
	return err
}

func (r *PostgresUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// scanUser scans a single row into a User struct.
func (r *PostgresUserRepository) scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.OrganizationID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role, &u.APIKeyHash,
		&u.LastLoginAt, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// nilIfEmpty returns a *string that is nil when s is empty, for nullable DB columns.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
