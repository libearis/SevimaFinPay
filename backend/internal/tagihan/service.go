package tagihan

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInvoiceAmount = errors.New("nominal tagihan UKT harus lebih besar dari 0 (bilangan bulat Rupiah)")
	ErrInvalidDueDate       = errors.New("format tanggal kedaluwarsa tidak valid (gunakan RFC3339 atau YYYY-MM-DD)")
	ErrInvalidStatusFilter  = errors.New("filter status hanya boleh bernilai UNPAID, PAID, atau EXPIRED")
	ErrEmptyRequiredFields  = errors.New("nomor tagihan dan semester wajib diisi")
)

// Service encapsulates business logic and multi-tenant isolation rules for UKT invoices.
type Service struct {
	repo Repository
}

// NewService creates a new tagihan Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListStudents returns all students belonging to the admin's institution.
func (s *Service) ListStudents(ctx context.Context, adminInstitusiID uuid.UUID) ([]StudentSummary, error) {
	return s.repo.ListStudentsByInstitution(ctx, adminInstitusiID)
}

// CreateInvoice creates a new UKT invoice for a student strictly within the admin's institution.
func (s *Service) CreateInvoice(ctx context.Context, adminInstitusiID uuid.UUID, req CreateInvoiceRequest) (*Invoice, error) {
	nomor := strings.TrimSpace(req.NomorTagihan)
	semester := strings.TrimSpace(req.Semester)
	if nomor == "" || semester == "" {
		return nil, ErrEmptyRequiredFields
	}

	if req.Nominal <= 0 {
		return nil, ErrInvalidInvoiceAmount
	}

	dueDate, err := parseDueDate(req.TanggalKedaluwarsa)
	if err != nil {
		return nil, ErrInvalidDueDate
	}

	// Verify that the target student actually belongs to the admin's institution!
	// This prevents an admin from Institution A creating an invoice for a student in Institution B.
	student, err := s.repo.GetStudentInInstitution(ctx, adminInstitusiID, req.MahasiswaID)
	if err != nil {
		return nil, err
	}

	inv := &Invoice{
		InstitusiID:        adminInstitusiID,
		MahasiswaID:        student.ID,
		MahasiswaNIM:       student.NIM,
		MahasiswaNama:      student.Name,
		MahasiswaEmail:     student.Email,
		NomorTagihan:       nomor,
		Semester:           semester,
		Nominal:            req.Nominal,
		Status:             StatusUnpaid,
		EligibleForKRS:     false,
		TanggalKedaluwarsa: dueDate,
	}

	return s.repo.CreateInvoice(ctx, inv)
}

// ListInvoicesForAdmin returns invoices strictly belonging to the admin's institution, with optional status/semester filters.
func (s *Service) ListInvoicesForAdmin(ctx context.Context, adminInstitusiID uuid.UUID, filter InvoiceFilter) ([]Invoice, error) {
	status := strings.ToUpper(strings.TrimSpace(filter.Status))
	if status != "" && status != StatusUnpaid && status != StatusPaid && status != StatusExpired {
		return nil, ErrInvalidStatusFilter
	}
	filter.Status = status
	filter.Semester = strings.TrimSpace(filter.Semester)

	return s.repo.ListInvoicesByInstitution(ctx, adminInstitusiID, filter)
}

// ListInvoicesForStudent returns invoices and transaction history strictly belonging to the authenticated student and institution.
func (s *Service) ListInvoicesForStudent(ctx context.Context, studentInstitusiID, studentUserID uuid.UUID) ([]Invoice, error) {
	return s.repo.ListInvoicesByStudent(ctx, studentInstitusiID, studentUserID)
}

// GetInvoiceDetailForStudent returns a single invoice and its transaction history if it belongs to the student and institution.
func (s *Service) GetInvoiceDetailForStudent(ctx context.Context, studentInstitusiID, studentUserID, invoiceID uuid.UUID) (*Invoice, error) {
	return s.repo.GetInvoiceByIDForStudent(ctx, studentInstitusiID, studentUserID, invoiceID)
}

func parseDueDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		// End of day UTC for YYYY-MM-DD inputs
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC), nil
	}
	return time.Time{}, ErrInvalidDueDate
}
