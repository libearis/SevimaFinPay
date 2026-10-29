package database

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_init_schema.sql
var initSchemaSQL string

// EnsureDatabaseExists connects to the default 'postgres' maintenance database and creates targetDB if it does not exist.
func EnsureDatabaseExists(ctx context.Context, adminDSN string, targetDB string) error {
	pool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		return fmt.Errorf("gagal terhubung ke instance PostgreSQL (admin db): %w", err)
	}
	defer pool.Close()

	var exists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, targetDB).Scan(&exists)
	if err != nil {
		return fmt.Errorf("gagal memeriksa keberadaan database '%s': %w", targetDB, err)
	}

	if !exists {
		// Create database (identifier is validated/trusted from config)
		_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, targetDB))
		if err != nil {
			return fmt.Errorf("gagal membuat database '%s': %w", targetDB, err)
		}
	}
	return nil
}

// NewPostgresPool connects to the PostgreSQL instance with connection pooling.
func NewPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("gagal mem-parse konfigurasi database: %w", err)
	}

	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat connection pool PostgreSQL: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("gagal terhubung ke instance PostgreSQL: %w", err)
	}

	return pool, nil
}

// RunMigrations executes the embedded SQL schema migrations idempotently.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, initSchemaSQL); err != nil {
		return fmt.Errorf("gagal menjalankan migrasi database: %w", err)
	}
	return nil
}
