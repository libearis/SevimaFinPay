package tagihan

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"ukt-payment-system/internal/middleware"
	"ukt-payment-system/pkg/response"
)

// Handler handles HTTP endpoints for UKT invoices (Admin Keuangan & Mahasiswa).
type Handler struct {
	service *Service
}

// NewHandler creates a new tagihan HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListStudents handles GET /api/admin/students.
func (h *Handler) ListStudents(c *gin.Context) {
	institusiID, ok := middleware.GetInstitusiID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Identitas institusi tidak valid")
		return
	}

	students, err := h.service.ListStudents(c.Request.Context(), institusiID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar mahasiswa")
		return
	}

	response.OK(c, students)
}

// ListAdminInvoices handles GET /api/admin/invoices.
func (h *Handler) ListAdminInvoices(c *gin.Context) {
	institusiID, ok := middleware.GetInstitusiID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Identitas institusi tidak valid")
		return
	}

	filter := InvoiceFilter{
		Status:   c.Query("status"),
		Semester: c.Query("semester"),
	}

	invoices, err := h.service.ListInvoicesForAdmin(c.Request.Context(), institusiID, filter)
	if err != nil {
		if errors.Is(err, ErrInvalidStatusFilter) {
			response.Error(c, http.StatusBadRequest, "INVALID_STATUS_FILTER", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar tagihan UKT")
		return
	}

	response.OK(c, invoices)
}

// CreateAdminInvoice handles POST /api/admin/invoices.
func (h *Handler) CreateAdminInvoice(c *gin.Context) {
	institusiID, ok := middleware.GetInstitusiID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Identitas institusi tidak valid")
		return
	}

	var req CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAYLOAD", "Field mahasiswa_id, nomor_tagihan, semester, nominal, dan tanggal_kedaluwarsa wajib diisi")
		return
	}

	invoice, err := h.service.CreateInvoice(c.Request.Context(), institusiID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyRequiredFields),
			errors.Is(err, ErrInvalidInvoiceAmount),
			errors.Is(err, ErrInvalidDueDate):
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		case errors.Is(err, ErrStudentNotFoundInTenant):
			response.Error(c, http.StatusForbidden, "CROSS_TENANT_FORBIDDEN", err.Error())
		case errors.Is(err, ErrDuplicateInvoiceNumber):
			response.Error(c, http.StatusConflict, "DUPLICATE_INVOICE_NUMBER", err.Error())
		case errors.Is(err, ErrDuplicateStudentSemester):
			response.Error(c, http.StatusConflict, "DUPLICATE_STUDENT_SEMESTER", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal membuat tagihan UKT baru")
		}
		return
	}

	response.Created(c, invoice)
}

// ListStudentInvoices handles GET /api/student/invoices.
func (h *Handler) ListStudentInvoices(c *gin.Context) {
	institusiID, ok1 := middleware.GetInstitusiID(c)
	userID, ok2 := middleware.GetUserID(c)
	if !ok1 || !ok2 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi mahasiswa tidak valid")
		return
	}

	invoices, err := h.service.ListInvoicesForStudent(c.Request.Context(), institusiID, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil data tagihan mahasiswa")
		return
	}

	response.OK(c, invoices)
}

// GetStudentInvoiceDetail handles GET /api/student/invoices/:id.
func (h *Handler) GetStudentInvoiceDetail(c *gin.Context) {
	institusiID, ok1 := middleware.GetInstitusiID(c)
	userID, ok2 := middleware.GetUserID(c)
	if !ok1 || !ok2 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi mahasiswa tidak valid")
		return
	}

	invoiceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INVOICE_ID", "Format ID tagihan tidak valid")
		return
	}

	invoice, err := h.service.GetInvoiceDetailForStudent(c.Request.Context(), institusiID, userID, invoiceID)
	if err != nil {
		if errors.Is(err, ErrInvoiceNotFound) {
			response.Error(c, http.StatusNotFound, "INVOICE_NOT_FOUND", "Tagihan tidak ditemukan atau bukan milik Anda")
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil detail tagihan")
		return
	}

	response.OK(c, invoice)
}
