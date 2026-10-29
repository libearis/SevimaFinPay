package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"ukt-payment-system/internal/tagihan"
)

var (
	ErrInvoiceNotFoundInInstitution = errors.New("nomor tagihan tidak ditemukan pada institusi terkait")
	ErrAmountMismatch               = errors.New("nominal pembayaran tidak sesuai dengan nominal tagihan UKT")
)

// AtomicProcessOutcome holds the outcome of locking & processing an invoice inside an ACID transaction.
type AtomicProcessOutcome struct {
	MahasiswaNIM     string
	MahasiswaNama    string
	Semester         string
	InvoiceStatus    string
	ProcessingResult string
	Message          string
}

// Repository defines database operations for atomic webhook processing and independent audit logging.
type Repository interface {
	ProcessPaymentEventAtomic(ctx context.Context, institusiID uuid.UUID, payload PaymentWebhookPayload, rawBody []byte) (*AtomicProcessOutcome, error)
	InsertAuditLog(ctx context.Context, logEntry WebhookAuditLog) error
	ListRecentAuditLogs(ctx context.Context, institutionCode string, limit int) ([]WebhookAuditLog, error)
	ListSimulatorInvoices(ctx context.Context, institutionCode string) ([]SimulatorInvoiceOption, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a PostgreSQL-backed webhook repository.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

// ProcessPaymentEventAtomic locks the target invoice row with SELECT ... FOR UPDATE,
// checks idempotency and state machine rules, updates invoice status, and records the transaction atomically.
func (r *postgresRepository) ProcessPaymentEventAtomic(
	ctx context.Context,
	institusiID uuid.UUID,
	payload PaymentWebhookPayload,
	rawBody []byte,
) (*AtomicProcessOutcome, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Lock the invoice row scoped strictly to institusi_id and nomor_tagihan
	var invoiceID uuid.UUID
	var mahasiswaID uuid.UUID
	var semester string
	var nominal int64
	var currentStatus string

	err = tx.QueryRow(ctx, `
		SELECT id, mahasiswa_id, semester, nominal, status
		FROM invoices
		WHERE institusi_id = $1 AND nomor_tagihan = $2
		FOR UPDATE
	`, institusiID, payload.NomorTagihan).Scan(&invoiceID, &mahasiswaID, &semester, &nominal, &currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvoiceNotFoundInInstitution
		}
		return nil, err
	}

	var mhsNIM, mhsNama string
	_ = tx.QueryRow(ctx, `SELECT nim_nip, name FROM users WHERE id = $1 AND institusi_id = $2`, mahasiswaID, institusiID).Scan(&mhsNIM, &mhsNama)

	// 2. Check idempotency: has this exact event_id already been recorded for this institution?
	var existingTxID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM transactions
		WHERE institusi_id = $1 AND event_id = $2
	`, institusiID, payload.EventID).Scan(&existingTxID)
	if err == nil {
		// Already processed this exact event_id -> idempotent replay!
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return &AtomicProcessOutcome{
			MahasiswaNIM:     mhsNIM,
			MahasiswaNama:    mhsNama,
			Semester:         semester,
			InvoiceStatus:    currentStatus,
			ProcessingResult: ResultIdempotentReplay,
			Message:          "Webhook dengan event_id ini sudah pernah diproses sebelumnya (idempotent replay)",
		}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	channel := strings.TrimSpace(payload.PaymentChannel)
	if channel == "" {
		channel = "VIRTUAL_ACCOUNT"
	}

	// 3. Evaluate State Machine transitions
	switch payload.EventType {
	case EventPaymentSuccess:
		// If invoice is already PAID (even with a different event_id), treat as idempotent replay to avoid double transactions
		if currentStatus == tagihan.StatusPaid {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, commitErr
			}
			return &AtomicProcessOutcome{
				MahasiswaNIM:     mhsNIM,
				MahasiswaNama:    mhsNama,
				Semester:         semester,
				InvoiceStatus:    tagihan.StatusPaid,
				ProcessingResult: ResultIdempotentReplay,
				Message:          "Tagihan sudah berstatus PAID; tidak membuat riwayat transaksi ganda (idempotent)",
			}, nil
		}

		// Verify amount matches invoice nominal (both UNPAID and EXPIRED late settlement require exact amount)
		if payload.Amount != nominal {
			return nil, ErrAmountMismatch
		}

		now := time.Now().UTC()
		_, err = tx.Exec(ctx, `
			UPDATE invoices
			SET status = $1, paid_at = $2, updated_at = $2
			WHERE id = $3 AND institusi_id = $4
		`, tagihan.StatusPaid, now, invoiceID, institusiID)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (
				institusi_id,
				invoice_id,
				event_id,
				event_type,
				payment_channel,
				amount,
				status,
				raw_payload,
				processed_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'SUCCESS', $7::jsonb, $8)
		`, institusiID, invoiceID, payload.EventID, payload.EventType, channel, payload.Amount, string(rawBody), now)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return &AtomicProcessOutcome{
					MahasiswaNIM:     mhsNIM,
					MahasiswaNama:    mhsNama,
					Semester:         semester,
					InvoiceStatus:    tagihan.StatusPaid,
					ProcessingResult: ResultIdempotentReplay,
					Message:          "Webhook duplikat terdeteksi secara konkuren (idempotent replay)",
				}, nil
			}
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		if currentStatus == tagihan.StatusExpired {
			return &AtomicProcessOutcome{
				MahasiswaNIM:     mhsNIM,
				MahasiswaNama:    mhsNama,
				Semester:         semester,
				InvoiceStatus:    tagihan.StatusPaid,
				ProcessingResult: ResultProcessedLateSettlement,
				Message:          "Late settlement berhasil diproses: status tagihan diperbarui dari EXPIRED menjadi PAID",
			}, nil
		}

		return &AtomicProcessOutcome{
			MahasiswaNIM:     mhsNIM,
			MahasiswaNama:    mhsNama,
			Semester:         semester,
			InvoiceStatus:    tagihan.StatusPaid,
			ProcessingResult: ResultProcessed,
			Message:          "Status tagihan berhasil diperbarui menjadi PAID",
		}, nil

	case EventPaymentExpired:
		// Out-of-Order Protection: PAID is a terminal state! Never downgrade PAID to EXPIRED.
		if currentStatus == tagihan.StatusPaid {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, commitErr
			}
			return &AtomicProcessOutcome{
				MahasiswaNIM:     mhsNIM,
				MahasiswaNama:    mhsNama,
				Semester:         semester,
				InvoiceStatus:    tagihan.StatusPaid,
				ProcessingResult: ResultIgnoredAlreadyPaid,
				Message:          "Event payment.expired diabaikan karena tagihan sudah berstatus PAID (out-of-order protection)",
			}, nil
		}

		if currentStatus == tagihan.StatusExpired {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, commitErr
			}
			return &AtomicProcessOutcome{
				MahasiswaNIM:     mhsNIM,
				MahasiswaNama:    mhsNama,
				Semester:         semester,
				InvoiceStatus:    tagihan.StatusExpired,
				ProcessingResult: ResultIdempotentReplay,
				Message:          "Tagihan sudah berstatus EXPIRED sebelumnya (idempotent replay)",
			}, nil
		}

		// Transition UNPAID -> EXPIRED
		now := time.Now().UTC()
		_, err = tx.Exec(ctx, `
			UPDATE invoices
			SET status = $1, updated_at = $2
			WHERE id = $3 AND institusi_id = $4
		`, tagihan.StatusExpired, now, invoiceID, institusiID)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (
				institusi_id,
				invoice_id,
				event_id,
				event_type,
				payment_channel,
				amount,
				status,
				raw_payload,
				processed_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'EXPIRED', $7::jsonb, $8)
		`, institusiID, invoiceID, payload.EventID, payload.EventType, channel, payload.Amount, string(rawBody), now)
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return &AtomicProcessOutcome{
			MahasiswaNIM:     mhsNIM,
			MahasiswaNama:    mhsNama,
			Semester:         semester,
			InvoiceStatus:    tagihan.StatusExpired,
			ProcessingResult: ResultProcessed,
			Message:          "Status tagihan berhasil diperbarui menjadi EXPIRED",
		}, nil
	}

	return nil, errors.New("event_type tidak didukung")
}

// InsertAuditLog writes a webhook notification entry into webhook_logs on an independent connection
// so that audit logs are ALWAYS persisted even when business validation or signature verification fails.
func (r *postgresRepository) InsertAuditLog(ctx context.Context, entry WebhookAuditLog) error {
	rawJSON := entry.RawPayload
	if !json.Valid([]byte(rawJSON)) {
		b, _ := json.Marshal(map[string]string{"raw_invalid_json": rawJSON})
		rawJSON = string(b)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_logs (
			institusi_id,
			institution_code,
			nomor_tagihan,
			event_id,
			event_type,
			signature_header,
			signature_valid,
			http_status,
			processing_result,
			error_message,
			raw_payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)
	`,
		entry.InstitusiID,
		entry.InstitutionCode,
		entry.NomorTagihan,
		entry.EventID,
		entry.EventType,
		entry.SignatureHeader,
		entry.SignatureValid,
		entry.HTTPStatus,
		entry.ProcessingResult,
		entry.ErrorMessage,
		rawJSON,
	)
	return err
}

// ListRecentAuditLogs retrieves recent webhook audit logs (optionally filtered by institution_code) for incident analysis & demo UI.
func (r *postgresRepository) ListRecentAuditLogs(ctx context.Context, institutionCode string, limit int) ([]WebhookAuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := `
		SELECT
			id,
			institusi_id,
			institution_code,
			nomor_tagihan,
			event_id,
			event_type,
			signature_header,
			signature_valid,
			http_status,
			processing_result,
			error_message,
			raw_payload::text,
			received_at
		FROM webhook_logs
	`
	args := []interface{}{}
	if strings.TrimSpace(institutionCode) != "" {
		query += " WHERE institution_code = $1 ORDER BY received_at DESC LIMIT $2"
		args = append(args, strings.TrimSpace(institutionCode), limit)
	} else {
		query += " ORDER BY received_at DESC LIMIT $1"
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]WebhookAuditLog, 0)
	for rows.Next() {
		var item WebhookAuditLog
		if err := rows.Scan(
			&item.ID,
			&item.InstitusiID,
			&item.InstitutionCode,
			&item.NomorTagihan,
			&item.EventID,
			&item.EventType,
			&item.SignatureHeader,
			&item.SignatureValid,
			&item.HTTPStatus,
			&item.ProcessingResult,
			&item.ErrorMessage,
			&item.RawPayload,
			&item.ReceivedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ListSimulatorInvoices returns invoice options with student ownership details for the Mock Payment Gateway UI.
func (r *postgresRepository) ListSimulatorInvoices(ctx context.Context, institutionCode string) ([]SimulatorInvoiceOption, error) {
	query := `
		SELECT
			inst.code AS institution_code,
			inst.name AS institution_name,
			inv.nomor_tagihan,
			inv.semester,
			u.nim_nip AS mahasiswa_nim,
			u.name AS mahasiswa_nama,
			inv.nominal,
			inv.status
		FROM invoices inv
		JOIN institutions inst ON inst.id = inv.institusi_id
		JOIN users u ON u.id = inv.mahasiswa_id AND u.institusi_id = inv.institusi_id
	`
	args := []interface{}{}
	if strings.TrimSpace(institutionCode) != "" {
		query += " WHERE inst.code = $1 ORDER BY inv.created_at DESC, inv.nomor_tagihan ASC"
		args = append(args, strings.TrimSpace(institutionCode))
	} else {
		query += " ORDER BY inst.code ASC, inv.nomor_tagihan ASC"
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SimulatorInvoiceOption, 0)
	for rows.Next() {
		var item SimulatorInvoiceOption
		if err := rows.Scan(
			&item.InstitutionCode,
			&item.InstitutionName,
			&item.NomorTagihan,
			&item.Semester,
			&item.MahasiswaNIM,
			&item.MahasiswaNama,
			&item.Nominal,
			&item.Status,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
