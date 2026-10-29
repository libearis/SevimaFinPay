package tagihan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvoiceNotFound          = errors.New("tagihan tidak ditemukan")
	ErrStudentNotFoundInTenant  = errors.New("mahasiswa tidak ditemukan di institusi Anda")
	ErrDuplicateInvoiceNumber   = errors.New("nomor tagihan sudah digunakan di institusi ini")
	ErrDuplicateStudentSemester = errors.New("mahasiswa ini sudah memiliki tagihan UKT pada semester tersebut")
)

// Repository defines tenant-scoped data access methods for UKT invoices.
// Every method explicitly requires institusiID to enforce strict multi-institution isolation.
type Repository interface {
	GetStudentInInstitution(ctx context.Context, institusiID, mahasiswaID uuid.UUID) (*StudentSummary, error)
	ListStudentsByInstitution(ctx context.Context, institusiID uuid.UUID) ([]StudentSummary, error)
	CreateInvoice(ctx context.Context, inv *Invoice) (*Invoice, error)
	ListInvoicesByInstitution(ctx context.Context, institusiID uuid.UUID, filter InvoiceFilter) ([]Invoice, error)
	ListInvoicesByStudent(ctx context.Context, institusiID, mahasiswaID uuid.UUID) ([]Invoice, error)
	GetInvoiceByIDForStudent(ctx context.Context, institusiID, mahasiswaID, invoiceID uuid.UUID) (*Invoice, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a PostgreSQL-backed tagihan repository.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) GetStudentInInstitution(ctx context.Context, institusiID, mahasiswaID uuid.UUID) (*StudentSummary, error) {
	var s StudentSummary
	err := r.pool.QueryRow(ctx, `
		SELECT id, institusi_id, nim_nip, name, email
		FROM users
		WHERE id = $1 AND institusi_id = $2 AND role = 'MAHASISWA'
	`, mahasiswaID, institusiID).Scan(&s.ID, &s.InstitusiID, &s.NIM, &s.Name, &s.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStudentNotFoundInTenant
		}
		return nil, err
	}
	return &s, nil
}

func (r *postgresRepository) ListStudentsByInstitution(ctx context.Context, institusiID uuid.UUID) ([]StudentSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, institusi_id, nim_nip, name, email
		FROM users
		WHERE institusi_id = $1 AND role = 'MAHASISWA'
		ORDER BY nim_nip ASC
	`, institusiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]StudentSummary, 0)
	for rows.Next() {
		var s StudentSummary
		if err := rows.Scan(&s.ID, &s.InstitusiID, &s.NIM, &s.Name, &s.Email); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *postgresRepository) CreateInvoice(ctx context.Context, inv *Invoice) (*Invoice, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO invoices (
			institusi_id,
			mahasiswa_id,
			nomor_tagihan,
			semester,
			nominal,
			status,
			tanggal_kedaluwarsa
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`,
		inv.InstitusiID,
		inv.MahasiswaID,
		inv.NomorTagihan,
		inv.Semester,
		inv.Nominal,
		inv.Status,
		inv.TanggalKedaluwarsa,
	).Scan(&inv.ID, &inv.CreatedAt, &inv.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "uq_invoices_institusi_nomor") {
				return nil, ErrDuplicateInvoiceNumber
			}
			if strings.Contains(pgErr.ConstraintName, "uq_invoices_institusi_mhs_semester") {
				return nil, ErrDuplicateStudentSemester
			}
		}
		return nil, err
	}

	inv.EligibleForKRS = inv.Status == StatusPaid
	inv.Transactions = []TransactionRecord{}
	return inv, nil
}

func (r *postgresRepository) ListInvoicesByInstitution(ctx context.Context, institusiID uuid.UUID, filter InvoiceFilter) ([]Invoice, error) {
	query := `
		SELECT
			inv.id,
			inv.institusi_id,
			inv.mahasiswa_id,
			u.nim_nip AS mahasiswa_nim,
			u.name AS mahasiswa_nama,
			u.email AS mahasiswa_email,
			inv.nomor_tagihan,
			inv.semester,
			inv.nominal,
			inv.status,
			inv.tanggal_kedaluwarsa,
			inv.paid_at,
			inv.created_at,
			inv.updated_at
		FROM invoices inv
		JOIN users u ON u.id = inv.mahasiswa_id AND u.institusi_id = inv.institusi_id
		WHERE inv.institusi_id = $1
	`
	args := []interface{}{institusiID}
	argIdx := 2

	if filter.Status != "" {
		query += fmt.Sprintf(" AND inv.status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Semester != "" {
		query += fmt.Sprintf(" AND inv.semester = $%d", argIdx)
		args = append(args, filter.Semester)
		argIdx++
	}

	query += " ORDER BY inv.created_at DESC, inv.nomor_tagihan ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invoices := make([]Invoice, 0)
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(
			&inv.ID,
			&inv.InstitusiID,
			&inv.MahasiswaID,
			&inv.MahasiswaNIM,
			&inv.MahasiswaNama,
			&inv.MahasiswaEmail,
			&inv.NomorTagihan,
			&inv.Semester,
			&inv.Nominal,
			&inv.Status,
			&inv.TanggalKedaluwarsa,
			&inv.PaidAt,
			&inv.CreatedAt,
			&inv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		inv.EligibleForKRS = inv.Status == StatusPaid
		inv.Transactions = []TransactionRecord{}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.attachTransactions(ctx, institusiID, invoices); err != nil {
		return nil, err
	}

	return invoices, nil
}

func (r *postgresRepository) ListInvoicesByStudent(ctx context.Context, institusiID, mahasiswaID uuid.UUID) ([]Invoice, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			inv.id,
			inv.institusi_id,
			inv.mahasiswa_id,
			u.nim_nip AS mahasiswa_nim,
			u.name AS mahasiswa_nama,
			u.email AS mahasiswa_email,
			inv.nomor_tagihan,
			inv.semester,
			inv.nominal,
			inv.status,
			inv.tanggal_kedaluwarsa,
			inv.paid_at,
			inv.created_at,
			inv.updated_at
		FROM invoices inv
		JOIN users u ON u.id = inv.mahasiswa_id AND u.institusi_id = inv.institusi_id
		WHERE inv.institusi_id = $1 AND inv.mahasiswa_id = $2
		ORDER BY inv.created_at DESC
	`, institusiID, mahasiswaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invoices := make([]Invoice, 0)
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(
			&inv.ID,
			&inv.InstitusiID,
			&inv.MahasiswaID,
			&inv.MahasiswaNIM,
			&inv.MahasiswaNama,
			&inv.MahasiswaEmail,
			&inv.NomorTagihan,
			&inv.Semester,
			&inv.Nominal,
			&inv.Status,
			&inv.TanggalKedaluwarsa,
			&inv.PaidAt,
			&inv.CreatedAt,
			&inv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		inv.EligibleForKRS = inv.Status == StatusPaid
		inv.Transactions = []TransactionRecord{}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.attachTransactions(ctx, institusiID, invoices); err != nil {
		return nil, err
	}

	return invoices, nil
}

func (r *postgresRepository) GetInvoiceByIDForStudent(ctx context.Context, institusiID, mahasiswaID, invoiceID uuid.UUID) (*Invoice, error) {
	var inv Invoice
	err := r.pool.QueryRow(ctx, `
		SELECT
			inv.id,
			inv.institusi_id,
			inv.mahasiswa_id,
			u.nim_nip AS mahasiswa_nim,
			u.name AS mahasiswa_nama,
			u.email AS mahasiswa_email,
			inv.nomor_tagihan,
			inv.semester,
			inv.nominal,
			inv.status,
			inv.tanggal_kedaluwarsa,
			inv.paid_at,
			inv.created_at,
			inv.updated_at
		FROM invoices inv
		JOIN users u ON u.id = inv.mahasiswa_id AND u.institusi_id = inv.institusi_id
		WHERE inv.id = $1 AND inv.institusi_id = $2 AND inv.mahasiswa_id = $3
	`, invoiceID, institusiID, mahasiswaID).Scan(
		&inv.ID,
		&inv.InstitusiID,
		&inv.MahasiswaID,
		&inv.MahasiswaNIM,
		&inv.MahasiswaNama,
		&inv.MahasiswaEmail,
		&inv.NomorTagihan,
		&inv.Semester,
		&inv.Nominal,
		&inv.Status,
		&inv.TanggalKedaluwarsa,
		&inv.PaidAt,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}

	inv.EligibleForKRS = inv.Status == StatusPaid
	inv.Transactions = []TransactionRecord{}

	slice := []Invoice{inv}
	if err := r.attachTransactions(ctx, institusiID, slice); err != nil {
		return nil, err
	}
	return &slice[0], nil
}

func (r *postgresRepository) attachTransactions(ctx context.Context, institusiID uuid.UUID, invoices []Invoice) error {
	if len(invoices) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, len(invoices))
	idxMap := make(map[uuid.UUID]int, len(invoices))
	for i := range invoices {
		ids[i] = invoices[i].ID
		idxMap[invoices[i].ID] = i
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			institusi_id,
			invoice_id,
			event_id,
			event_type,
			payment_channel,
			amount,
			status,
			processed_at
		FROM transactions
		WHERE institusi_id = $1 AND invoice_id = ANY($2)
		ORDER BY processed_at DESC
	`, institusiID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tr TransactionRecord
		if err := rows.Scan(
			&tr.ID,
			&tr.InstitusiID,
			&tr.InvoiceID,
			&tr.EventID,
			&tr.EventType,
			&tr.PaymentChannel,
			&tr.Amount,
			&tr.Status,
			&tr.ProcessedAt,
		); err != nil {
			return err
		}
		if pos, ok := idxMap[tr.InvoiceID]; ok {
			invoices[pos].Transactions = append(invoices[pos].Transactions, tr)
		}
	}
	return rows.Err()
}
