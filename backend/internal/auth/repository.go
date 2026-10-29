package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("pengguna tidak ditemukan")

// Repository defines database operations for user authentication.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (*UserWithInstitution, error)
	GetByIDAndInstitusi(ctx context.Context, userID, institusiID uuid.UUID) (*UserWithInstitution, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a PostgreSQL-backed auth repository.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) GetByEmail(ctx context.Context, email string) (*UserWithInstitution, error) {
	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	var u UserWithInstitution
	err := r.pool.QueryRow(ctx, `
		SELECT
			u.id,
			u.institusi_id,
			i.code AS kode_institusi,
			i.name AS nama_institusi,
			u.nim_nip,
			u.name,
			u.email,
			u.password_hash,
			u.role,
			u.created_at
		FROM users u
		JOIN institutions i ON i.id = u.institusi_id
		WHERE LOWER(u.email) = $1
		LIMIT 1
	`, cleanEmail).Scan(
		&u.ID,
		&u.InstitusiID,
		&u.KodeInstitusi,
		&u.NamaInstitusi,
		&u.NimNip,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *postgresRepository) GetByIDAndInstitusi(ctx context.Context, userID, institusiID uuid.UUID) (*UserWithInstitution, error) {
	var u UserWithInstitution
	err := r.pool.QueryRow(ctx, `
		SELECT
			u.id,
			u.institusi_id,
			i.code AS kode_institusi,
			i.name AS nama_institusi,
			u.nim_nip,
			u.name,
			u.email,
			u.password_hash,
			u.role,
			u.created_at
		FROM users u
		JOIN institutions i ON i.id = u.institusi_id
		WHERE u.id = $1 AND u.institusi_id = $2
	`, userID, institusiID).Scan(
		&u.ID,
		&u.InstitusiID,
		&u.KodeInstitusi,
		&u.NamaInstitusi,
		&u.NimNip,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}
