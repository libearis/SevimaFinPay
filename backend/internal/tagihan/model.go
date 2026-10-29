package tagihan

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusUnpaid  = "UNPAID"
	StatusPaid    = "PAID"
	StatusExpired = "EXPIRED"
)

// StudentSummary represents student user data strictly scoped to an institution.
type StudentSummary struct {
	ID          uuid.UUID `json:"id"`
	InstitusiID uuid.UUID `json:"institusi_id"`
	NIM         string    `json:"nim"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
}

// TransactionRecord represents a single payment/webhook transaction history entry for an invoice.
type TransactionRecord struct {
	ID             uuid.UUID `json:"id"`
	InstitusiID    uuid.UUID `json:"institusi_id"`
	InvoiceID      uuid.UUID `json:"invoice_id"`
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	PaymentChannel string    `json:"payment_channel"`
	Amount         int64     `json:"amount"`
	Status         string    `json:"status"`
	ProcessedAt    time.Time `json:"processed_at"`
}

// Invoice represents a UKT invoice record joined with student details and optional transactions.
type Invoice struct {
	ID                 uuid.UUID           `json:"id"`
	InstitusiID        uuid.UUID           `json:"institusi_id"`
	MahasiswaID        uuid.UUID           `json:"mahasiswa_id"`
	MahasiswaNIM       string              `json:"mahasiswa_nim"`
	MahasiswaNama      string              `json:"mahasiswa_nama"`
	MahasiswaEmail     string              `json:"mahasiswa_email"`
	NomorTagihan       string              `json:"nomor_tagihan"`
	Semester           string              `json:"semester"`
	Nominal            int64               `json:"nominal"`
	Status             string              `json:"status"`
	EligibleForKRS     bool                `json:"eligible_for_krs"`
	TanggalKedaluwarsa time.Time           `json:"tanggal_kedaluwarsa"`
	PaidAt             *time.Time          `json:"paid_at"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	Transactions       []TransactionRecord `json:"transactions"`
}
