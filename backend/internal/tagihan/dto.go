package tagihan

import "github.com/google/uuid"

// CreateInvoiceRequest represents the request body for POST /api/admin/invoices.
// Notice that institusi_id is NOT accepted from the client request body; it is strictly injected from the JWT claims.
type CreateInvoiceRequest struct {
	MahasiswaID        uuid.UUID `json:"mahasiswa_id" binding:"required"`
	NomorTagihan       string    `json:"nomor_tagihan" binding:"required"`
	Semester           string    `json:"semester" binding:"required"`
	Nominal            int64     `json:"nominal" binding:"required"`
	TanggalKedaluwarsa string    `json:"tanggal_kedaluwarsa" binding:"required"`
}

// InvoiceFilter defines optional filter parameters for listing invoices within an institution.
type InvoiceFilter struct {
	Status   string
	Semester string
}
