package tagihan

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"ukt-payment-system/internal/middleware"
)

func TestHandler_AdminAndStudentHTTP_MultiTenantIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newFakeTagihanRepo()
	svc := NewService(repo)
	handler := NewHandler(svc)

	instA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	instB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	mhsA := uuid.MustParse("11111111-aaaa-4111-8111-000000000001")
	mhsB := uuid.MustParse("22222222-aaaa-4222-8222-000000000001")

	repo.students = []StudentSummary{
		{ID: mhsA, InstitusiID: instA, NIM: "202610001", Name: "Andi Kampus A", Email: "andi@a.ac.id"},
		{ID: mhsB, InstitusiID: instB, NIM: "202620001", Name: "Rizky Kampus B", Email: "rizky@b.ac.id"},
	}

	// Helper untuk membuat router dengan simulasi klaim JWT di Gin Context
	setupRouter := func(instID, userID uuid.UUID) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(middleware.CtxKeyInstitusiID, instID)
			c.Set(middleware.CtxKeyUserID, userID)
			c.Next()
		})
		r.POST("/api/admin/invoices", handler.CreateAdminInvoice)
		r.GET("/api/admin/invoices", handler.ListAdminInvoices)
		r.GET("/api/student/invoices", handler.ListStudentInvoices)
		return r
	}

	routerAdminA := setupRouter(instA, uuid.New())

	// 1. POST /api/admin/invoices oleh Admin Kampus A untuk Mahasiswa Kampus A -> 201 Created
	bodyValid, _ := json.Marshal(CreateInvoiceRequest{
		MahasiswaID:        mhsA,
		NomorTagihan:       "UKT/2026-1/001",
		Semester:           "2026/2027-GANJIL",
		Nominal:            5500000,
		TanggalKedaluwarsa: "2026-12-31",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/admin/invoices", bytes.NewReader(bodyValid))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	routerAdminA.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("diharapkan 201 Created, didapat %d: %s", w1.Code, w1.Body.String())
	}

	// 2. POST /api/admin/invoices oleh Admin Kampus A untuk Mahasiswa Kampus B -> 403 Forbidden (Cross-Tenant)
	bodyCrossTenant, _ := json.Marshal(CreateInvoiceRequest{
		MahasiswaID:        mhsB,
		NomorTagihan:       "UKT/2026-1/002",
		Semester:           "2026/2027-GANJIL",
		Nominal:            5500000,
		TanggalKedaluwarsa: "2026-12-31",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/admin/invoices", bytes.NewReader(bodyCrossTenant))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	routerAdminA.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("diharapkan 403 Forbidden saat Admin Kampus A menagih Mahasiswa Kampus B, didapat %d", w2.Code)
	}

	// 3. POST duplikat nomor tagihan di Kampus A -> 409 Conflict
	req3 := httptest.NewRequest(http.MethodPost, "/api/admin/invoices", bytes.NewReader(bodyValid))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	routerAdminA.ServeHTTP(w3, req3)

	if w3.Code != http.StatusConflict {
		t.Fatalf("diharapkan 409 Conflict saat nomor tagihan diduplikasi di kampus yang sama, didapat %d", w3.Code)
	}
}
