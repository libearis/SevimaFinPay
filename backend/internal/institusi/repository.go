package institusi

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInstitutionNotFound = errors.New("institusi tidak ditemukan")

// Repository defines data access methods for institutions.
type Repository interface {
	GetByCode(ctx context.Context, code string) (*Institution, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Institution, error)
	ListAll(ctx context.Context) ([]Institution, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a PostgreSQL-backed institution repository.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) GetByCode(ctx context.Context, code string) (*Institution, error) {
	var inst Institution
	err := r.pool.QueryRow(ctx, `
		SELECT id, code, name, hmac_secret_env_key, created_at
		FROM institutions
		WHERE code = $1
	`, code).Scan(&inst.ID, &inst.Code, &inst.Name, &inst.HMACSecretEnvKey, &inst.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstitutionNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Institution, error) {
	var inst Institution
	err := r.pool.QueryRow(ctx, `
		SELECT id, code, name, hmac_secret_env_key, created_at
		FROM institutions
		WHERE id = $1
	`, id).Scan(&inst.ID, &inst.Code, &inst.Name, &inst.HMACSecretEnvKey, &inst.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstitutionNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *postgresRepository) ListAll(ctx context.Context) ([]Institution, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, name, hmac_secret_env_key, created_at
		FROM institutions
		ORDER BY code ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Institution
	for rows.Next() {
		var inst Institution
		if err := rows.Scan(&inst.ID, &inst.Code, &inst.Name, &inst.HMACSecretEnvKey, &inst.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, inst)
	}
	return result, rows.Err()
}
